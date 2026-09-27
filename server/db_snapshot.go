package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// dbSnapshotName is the object name inside the prefix, and the file name in
	// the local scratch directory the snapshot is built in.
	dbSnapshotName = dbFileName

	// defaultDBBackupPrefix keeps the snapshot in the blob bucket without ever
	// colliding with a stored blob. Storage keys are sharded by the first two
	// characters of a slug and slugs are [A-Za-z0-9] only, so a directory
	// starting with "_" cannot be a shard - whereas something like "db" could
	// be, for a slug such as "dby12".
	defaultDBBackupPrefix = "_backup"

	// maxDBSnapshotSize bounds what will be read from the snapshot store, so a
	// wrong prefix pointing at unrelated data cannot fill the disk.
	maxDBSnapshotSize int64 = 4 << 30

	// snapshotOperationTimeout bounds one download or publish.
	snapshotOperationTimeout = 2 * time.Minute

	// shutdownPublishTimeout bounds the final publish on the way out.
	shutdownPublishTimeout = 10 * time.Second

	// dbBackupPollInterval is how often the change counter is read. It only
	// bounds how late a publish can start, not how often one happens.
	dbBackupPollInterval = time.Second

	// defaultDBPollInterval is how often a replica asks whether a new snapshot
	// has been published.
	defaultDBPollInterval = time.Minute

	// shutdownTimeout bounds how long in-flight requests get on the way out.
	shutdownTimeout = 15 * time.Second

	dbRestoreAuto   = "auto"
	dbRestoreIgnore = "ignore"
)

var (
	// snapshotStore is the object store the database snapshot travels through,
	// shared by the writer that publishes it and the readers that subscribe to
	// it. It is available whenever ART_S3_BUCKET is set, independently of
	// ART_STORAGE, so a filesystem-backed deployment can still publish offsite
	// and still run replicas.
	snapshotStore *s3Storage

	// activePublisher and activeReplica are the background loops, if this
	// instance is acting as one. They are also what /api/health reports on.
	activePublisher *snapshotPublisher
	activeReplica   *snapshotReplica

	// backgroundContext is cancelled on shutdown, which stops the snapshot loops
	// after the publisher has had its last chance to publish.
	backgroundContext context.Context
	stopBackground    context.CancelFunc
)

// snapshotKey is the object name of the published snapshot inside the configured
// bucket. The prefix is part of the store, so the key is just the file name.
func snapshotKey() string { return dbSnapshotName }

// snapshotDir is the local scratch directory a snapshot is built in. It sits
// beside the blobs on the writer - on the volume that already exists - and is
// created on demand.
func snapshotDir() string { return filepath.Join(uploadFolder, dbBackupPrefix) }

// snapshotConfigured reports whether a snapshot store is available at all.
func snapshotConfigured() bool { return snapshotStore != nil }

// initSnapshotStore builds the snapshot store when ART_S3_BUCKET names a
// bucket. Unlike blobs it is not selected by ART_STORAGE: the record database is
// small and worth copying offsite even when the blobs are on local disk.
func initSnapshotStore(ctx context.Context) error {
	if s3Bucket == "" {
		if dbReplica {
			return errors.New("ART_DB_REPLICA needs ART_S3_BUCKET: the snapshot lives in the bucket")
		}
		if dbBackupDelay > 0 {
			return errors.New("ART_DB_BACKUP_DELAY needs ART_S3_BUCKET: nothing would be uploaded")
		}
		return nil
	}

	client, err := sharedS3Client(ctx)
	if err != nil {
		return err
	}
	snapshotStore = newS3StorageWithClient(client, dbBackupPrefix)
	return nil
}

// sqlQuote renders s as a single-quoted SQL string literal. VACUUM INTO takes a
// file name expression and not every sqlite build accepts a bound parameter
// there, so the literal is spelled out and the quotes escaped.
func sqlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// writeSnapshotTo builds a consistent copy of the live database at dst with
// VACUUM INTO.
//
// The copy is a single self-contained file: no write-ahead log and no
// shared-memory sidecar, which is exactly what makes it safe to hand to another
// process with no coordination. VACUUM INTO refuses to overwrite, so an existing
// file is removed first, and it must never run inside a transaction - it is a
// write statement in its own right.
func writeSnapshotTo(handle *sql.DB, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := handle.Exec("VACUUM INTO " + sqlQuote(dst)); err != nil {
		return fmt.Errorf("vacuuming into %s: %w", dst, err)
	}
	return nil
}

// verifySnapshot checks a snapshot file before anything is allowed to use it. It
// catches what a partial download and a wrong object both look like: a file that
// opens, passes its own integrity check, and holds the schema this build reads.
func verifySnapshot(path string) error {
	handle, err := openSQLite(path, true)
	if err != nil {
		return err
	}
	defer handle.Close()

	var check string
	if err := handle.QueryRow("PRAGMA quick_check").Scan(&check); err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}
	if check != "ok" {
		return fmt.Errorf("integrity check reported %q", check)
	}

	var version int
	if err := handle.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if version != schemaVersion {
		return fmt.Errorf("snapshot is schema v%d, but this build reads v%d", version, schemaVersion)
	}

	var rows int64
	if err := handle.QueryRow("SELECT count(*) FROM files").Scan(&rows); err != nil {
		return fmt.Errorf("reading records: %w", err)
	}
	return nil
}

// headSnapshot describes the published snapshot without downloading it, which is
// all a reader needs to notice that a new one exists. It costs one request: the
// store answers a Get from a HEAD and only issues a ranged GET once the returned
// reader is actually read.
func headSnapshot(ctx context.Context) (BlobInfo, error) {
	if snapshotStore == nil {
		return BlobInfo{}, errors.New("no snapshot store configured")
	}
	blob, info, err := snapshotStore.Get(ctx, snapshotKey())
	if err != nil {
		return BlobInfo{}, err
	}
	blob.Close()
	return info, nil
}

// downloadSnapshot fetches the published snapshot into dst and returns what it
// fetched.
//
// The order matters and is the point: fetch to a temporary name, verify it, and
// only then rename it into place. A truncated or wrong-version download must
// never replace a snapshot that is already being served, and a rename is atomic
// within the directory, so a reader either keeps the old file or gets the whole
// new one.
func downloadSnapshot(ctx context.Context, dst string) (BlobInfo, error) {
	if snapshotStore == nil {
		return BlobInfo{}, errors.New("no snapshot store configured")
	}

	blob, info, err := snapshotStore.Get(ctx, snapshotKey())
	if err != nil {
		return BlobInfo{}, err
	}
	defer blob.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return BlobInfo{}, err
	}
	tmp := dst + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return BlobInfo{}, err
	}

	written, copyErr := io.Copy(file, io.LimitReader(blob, maxDBSnapshotSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		os.Remove(tmp)
		return BlobInfo{}, fmt.Errorf("downloading snapshot %s: %w", snapshotKey(), copyErr)
	}
	if closeErr != nil {
		os.Remove(tmp)
		return BlobInfo{}, closeErr
	}
	if written > maxDBSnapshotSize {
		os.Remove(tmp)
		return BlobInfo{}, fmt.Errorf("snapshot exceeds the %d byte limit", maxDBSnapshotSize)
	}
	// A short read is the common failure of a dropped transfer: the store said
	// how big the object is, so a mismatch is never worth trying to open.
	if info.Size > 0 && written != info.Size {
		os.Remove(tmp)
		return BlobInfo{}, fmt.Errorf("snapshot is %d bytes, expected %d", written, info.Size)
	}

	if err := verifySnapshot(tmp); err != nil {
		os.Remove(tmp)
		return BlobInfo{}, fmt.Errorf("snapshot %s is unusable: %w", snapshotKey(), err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return BlobInfo{}, err
	}

	return info, nil
}

// sameSnapshot reports whether two descriptions refer to the same object.
//
// The entity tag is the reliable part: the store derives it from the content, so
// equal bytes give an equal tag and a different snapshot always looks different.
// Size and modification time cover a backend that provides no tag - at the cost
// of missing an update that lands within the same clock second and happens to
// have the same size.
func sameSnapshot(a, b BlobInfo) bool {
	if a.ETag != "" && b.ETag != "" {
		return a.ETag == b.ETag && a.Size == b.Size
	}
	return a.Size == b.Size && a.ModTime.Equal(b.ModTime)
}

// snapshotLooksEmpty reports whether publishing this snapshot would replace a
// real backup with an empty database, and why.
//
// An empty database is what a lost or unmounted volume looks like, and
// publishing it over the good copy would destroy the one thing this mechanism
// exists to survive. Records are tombstoned rather than deleted, so a database
// whose files were all removed still has rows; only one that never held anything
// is empty. Setting ART_DB_RESTORE=ignore says an empty database here is
// intended, and lifts the guard.
func snapshotLooksEmpty(ctx context.Context, rows int64) (string, error) {
	if rows > 0 || dbRestoreMode == dbRestoreIgnore {
		return "", nil
	}

	info, err := headSnapshot(ctx)
	if err != nil {
		if errors.Is(err, errBlobNotFound) {
			return "", nil // nothing published yet, so nothing to lose
		}
		return "", err
	}
	if info.Size == 0 {
		return "", nil
	}

	return fmt.Sprintf(
		"this database is empty while the published snapshot holds data (%d bytes); "+
			"set ART_DB_RESTORE=ignore if the empty database is intended", info.Size), nil
}

// runSnapshotPublisher starts the background publish loop when a delay is
// configured, and reports whether one is running.
func runSnapshotPublisher(wg *sync.WaitGroup) {
	// Only a writer publishes. A read-only instance has nothing to publish, and
	// its database handle would refuse the VACUUM INTO anyway - so a delay
	// inherited from a shared configuration environment is ignored here rather
	// than turning into a stream of failed snapshots.
	if readOnly || dbBackupDelay <= 0 || snapshotStore == nil {
		return
	}

	publisher, err := newSnapshotPublisher()
	if err != nil {
		log.Printf("Snapshot: publishing is disabled, could not start: %v", err)
		return
	}
	activePublisher = publisher

	wg.Add(1)
	go func() {
		defer wg.Done()
		publisher.Run(backgroundContext)
	}()
	log.Printf("Snapshot: publishing %s after %s of quiet (prefix %q)",
		snapshotKey(), dbBackupDelay, dbBackupPrefix)
}

// runSnapshotReplica starts the background refresh loop on a read-only replica.
func runSnapshotReplica(wg *sync.WaitGroup) {
	if activeReplica == nil {
		return
	}
	replica := activeReplica

	wg.Add(1)
	go func() {
		defer wg.Done()
		replica.Run(backgroundContext)
	}()
	log.Printf("Snapshot: following %s every %s", snapshotKey(), dbPollInterval)
}

// databaseHealth describes the local database for /api/health.
func databaseHealth() *DatabaseHealth {
	health := &DatabaseHealth{SchemaVersion: schemaVersion}

	if activeReplica != nil {
		health.Snapshot = snapshotKey()
		health.Generation = activeReplica.generation()
		seconds := activeReplica.age().Seconds()
		health.AgeSeconds = &seconds
	}
	return health
}
