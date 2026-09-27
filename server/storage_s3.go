package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// s3API is the slice of the S3 client this backend uses for reading and
// deleting. Narrowing it to an interface keeps the ranged reader testable
// without a network.
type s3API interface {
	HeadObject(ctx context.Context, in *s3.HeadObjectInput, opt ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(ctx context.Context, in *s3.GetObjectInput, opt ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opt ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// uploaderAPI is the transfer manager's upload call. Uploads go through the
// manager rather than a bare PutObject because the request body streams in with
// no length known up front: the manager buffers it in parts, which the SDK can
// sign and checksum even over plain HTTP.
type uploaderAPI interface {
	UploadObject(ctx context.Context, in *transfermanager.UploadObjectInput, opt ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error)
}

// s3Storage keeps blobs in an S3 bucket, which is what lets several instances
// serve the same files without replicating them.
type s3Storage struct {
	client   s3API
	uploader uploaderAPI
	bucket   string
	prefix   string // normalised to "" or "…/"
}

// newS3Storage builds the S3 backend from the ART_S3_* configuration.
// Credentials come from the AWS default chain (environment, shared config,
// container or instance role), so long-lived keys never have to be baked in.
func newS3Storage(ctx context.Context) (Storage, error) {
	if s3Bucket == "" {
		return nil, errors.New("ART_S3_BUCKET is required when ART_STORAGE=s3")
	}

	var opts []func(*awsconfig.LoadOptions) error
	switch {
	case s3Region != "":
		opts = append(opts, awsconfig.WithRegion(s3Region))
	case s3Endpoint != "":
		// S3-compatible stores (MinIO and friends) ignore the region, but the
		// SDK refuses to build a client without one.
		opts = append(opts, awsconfig.WithRegion("us-east-1"))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading aws configuration: %w", err)
	}

	client := s3.NewFromConfig(cfg, s3ClientOptions(s3Endpoint, s3PathStyle))

	return &s3Storage{
		client:   client,
		uploader: transfermanager.New(client),
		bucket:   s3Bucket,
		prefix:   normaliseS3Prefix(s3Prefix),
	}, nil
}

// s3ClientOptions points the client at the configured endpoint. It is shared
// with the tests so the wire behaviour they exercise is the one that ships.
func s3ClientOptions(endpoint string, pathStyle bool) func(*s3.Options) {
	return func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}

		// S3-compatible stores are normally reached with the bucket in the path
		// rather than the hostname, so path-style is the default for them.
		o.UsePathStyle = pathStyle
	}
}

// normaliseS3Prefix turns "artifacts" into "artifacts/" so object keys are
// built by plain concatenation, and an unset prefix into "".
func normaliseS3Prefix(prefix string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/"
}

func (s *s3Storage) objectKey(key string) string { return s.prefix + key }

func (s *s3Storage) Put(ctx context.Context, key string, r io.Reader, maxBytes int64) (int64, error) {
	reader := newCapReader(r, maxBytes)

	// The transfer manager streams the body in buffered parts, so nothing has to
	// be staged on local disk and the size cap is still enforced while reading.
	_, err := s.uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
		Body:   reader,
	})
	if err != nil {
		if reader.overflowed {
			// The stream stopped at the cap, but a rejected PUT can still have
			// left a partial object behind.
			_ = s.Delete(ctx, key)
			return 0, errBlobTooLarge
		}
		return 0, err
	}
	if reader.n > maxBytes {
		_ = s.Delete(ctx, key)
		return 0, errBlobTooLarge
	}

	return reader.n, nil
}

func (s *s3Storage) Get(ctx context.Context, key string) (io.ReadSeekCloser, BlobInfo, error) {
	// A HEAD gives the size and modification time without transferring the
	// object, which is exactly what ServeContent needs before it reads.
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, BlobInfo{}, errBlobNotFound
		}
		return nil, BlobInfo{}, err
	}

	info := BlobInfo{Size: aws.ToInt64(head.ContentLength)}
	if head.LastModified != nil {
		info.ModTime = *head.LastModified
	}

	return &s3ReadSeeker{
		client: s.client,
		bucket: s.bucket,
		key:    s.objectKey(key),
		ctx:    ctx,
		size:   info.Size,
	}, info, nil
}

func (s *s3Storage) Delete(ctx context.Context, key string) error {
	// S3 deletes are idempotent: removing a key that isn't there succeeds.
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	return err
}

// s3ReadSeeker reads one object through ranged GETs. http.ServeContent sizes a
// response by seeking to the end and then seeks to the range it needs, so the
// size is answered from the HEAD result and only the reads that follow become
// requests: a plain download costs one GET, and a ranged request costs one GET
// for that range.
type s3ReadSeeker struct {
	client s3API
	bucket string
	key    string
	ctx    context.Context
	size   int64
	pos    int64
	body   io.ReadCloser
}

func (s *s3ReadSeeker) Read(p []byte) (int, error) {
	if s.pos >= s.size {
		return 0, io.EOF
	}
	if s.body == nil {
		if err := s.openAt(); err != nil {
			return 0, err
		}
	}

	n, err := s.body.Read(p)
	s.pos += int64(n)
	if err == io.EOF {
		s.closeBody()
		if s.pos < s.size {
			// The object ended early: never let a truncated transfer look like
			// a complete response.
			return n, io.ErrUnexpectedEOF
		}
	}
	return n, err
}

func (s *s3ReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var target int64
	switch whence {
	case io.SeekStart:
		target = offset
	case io.SeekCurrent:
		target = s.pos + offset
	case io.SeekEnd:
		target = s.size + offset
	default:
		return 0, errors.New("s3 storage: invalid seek origin")
	}
	if target < 0 {
		return 0, errors.New("s3 storage: negative seek position")
	}

	// The open body only covers the position it was opened at, so moving
	// elsewhere abandons it and the next read opens a ranged request.
	if target != s.pos {
		s.closeBody()
	}
	s.pos = target
	return target, nil
}

func (s *s3ReadSeeker) Close() error {
	s.closeBody()
	return nil
}

func (s *s3ReadSeeker) closeBody() {
	if s.body != nil {
		s.body.Close()
		s.body = nil
	}
}

func (s *s3ReadSeeker) openAt() error {
	in := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key),
	}
	if s.pos > 0 {
		in.Range = aws.String(fmt.Sprintf("bytes=%d-", s.pos))
	}

	out, err := s.client.GetObject(s.ctx, in)
	if err != nil {
		return err
	}
	s.body = out.Body
	return nil
}

// isS3NotFound reports whether err is S3 saying the key does not exist. The SDK
// surfaces that as a typed error for some operations and as a generic API error
// for others, so every shape is checked.
func isS3NotFound(err error) bool {
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return true
		}
	}

	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		return respErr.HTTPStatusCode() == http.StatusNotFound
	}
	return false
}
