package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

// mintSessionTokenWithPerms mints a session token for the standard test subject
// carrying exactly perms, which is how these tests act as a caller with a narrow
// role.
func mintSessionTokenWithPerms(t *testing.T, perms ...string) string {
	t.Helper()

	token, err := mintSessionToken("mint", sessionIdentity{Subject: "tester"}, perms, time.Hour, time.Now())
	if err != nil {
		t.Fatalf("minting session token: %v", err)
	}
	return token
}

// withUploadServer boots a writable instance on scratch storage, which is what
// the upload handler needs to run end to end.
func withUploadServer(t *testing.T) {
	t.Helper()

	restoreGlobals(t)
	withAuthConfig(t, "", testSecret)
	withRolesConfig(t, "", "")

	uploadFolder = t.TempDir()
	storage = newFsStorage(t.TempDir())
	readOnly, dbReplica, appendOnly = false, false, false
	maxContentLength = 1 << 20

	if err := bootDatabase(); err != nil {
		t.Fatalf("bootDatabase: %v", err)
	}
}

// uploadChain is the real route: authenticate, require file:create, then let the
// handler authorize the effects only it can see.
func uploadChain() http.HandlerFunc {
	return requireToken(requirePermission(permFileCreate, rejectWhenReadOnly(uploadFileHandler)))
}

// uploadRequest builds a multipart upload. Tags are written before the file
// part, which is only one of the two arrangements the handler supports.
func uploadRequest(t *testing.T, token, filename, content string, tags ...string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, tag := range tags {
		if err := writer.WriteField("tags", tag); err != nil {
			t.Fatalf("writing tags field: %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("creating the file part: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("writing the file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func doUpload(t *testing.T, token, filename, content string, tags ...string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	uploadChain()(recorder, uploadRequest(t, token, filename, content, tags...))
	return recorder
}

// responseError decodes the error field of a JSON response.
func responseError(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response %q: %v", recorder.Body.String(), err)
	}
	return body.Error
}

// liveByName reports whether a live file with this display name exists.
func liveByName(t *testing.T, name string) bool {
	t.Helper()

	rec, err := liveRecordByName(name)
	if err != nil {
		t.Fatalf("liveRecordByName(%q): %v", name, err)
	}
	return rec != nil
}

func TestUploadRequiresFileCreate(t *testing.T) {
	withUploadServer(t)

	if got := doUpload(t, "", "a.txt", "x").Code; got != http.StatusUnauthorized {
		t.Errorf("no credential: status = %d, want 401", got)
	}
	if got := doUpload(t, mintSessionTokenWithPerms(t, permTagAdd), "a.txt", "x").Code; got != http.StatusForbidden {
		t.Errorf("without file:create: status = %d, want 403", got)
	}
	if got := doUpload(t, mintSessionTokenWithPerms(t, permFileCreate), "a.txt", "x").Code; got != http.StatusCreated {
		t.Errorf("with file:create: status = %d, want 201", got)
	}
}

// Attaching a tag costs tag:add even though the route only asks for file:create.
func TestUploadWithTagsRequiresTagAdd(t *testing.T) {
	withUploadServer(t)

	recorder := doUpload(t, mintSessionTokenWithPerms(t, permFileCreate), "cat.png", "x", "cat:latest")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := responseError(t, recorder); !strings.Contains(got, permTagAdd) {
		t.Errorf("error = %q, want it to name %s", got, permTagAdd)
	}
	if liveByName(t, "cat.png") {
		t.Error("a refused upload left a live file behind")
	}

	// A tag nobody else holds needs only tag:add.
	recorder = doUpload(t, mintSessionTokenWithPerms(t, permFileCreate, permTagAdd), "cat.png", "x", "cat:latest")
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", recorder.Code, recorder.Body.String())
	}
}

// A tag is globally unique, so naming one that already points at another file
// moves it off that file - a removal there, and so tag:remove.
func TestUploadMovingATagRequiresTagRemove(t *testing.T) {
	withUploadServer(t)

	admin := mintSessionTokenWithPerms(t, allPermissions...)
	if recorder := doUpload(t, admin, "one.txt", "x", "cat:latest"); recorder.Code != http.StatusCreated {
		t.Fatalf("seeding the tag: status = %d (%s)", recorder.Code, recorder.Body.String())
	}

	recorder := doUpload(t, mintSessionTokenWithPerms(t, permFileCreate, permTagAdd), "two.txt", "x", "cat:latest")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := responseError(t, recorder); !strings.Contains(got, permTagRemove) {
		t.Errorf("error = %q, want it to name %s", got, permTagRemove)
	}
	if liveByName(t, "two.txt") {
		t.Error("a refused upload left a live file behind")
	}

	owner, err := tagOwner(db(), Tag{Name: "cat", Suffix: "latest"})
	if err != nil {
		t.Fatalf("tagOwner: %v", err)
	}
	if owner == "" {
		t.Error("the refused upload detached the tag from the file that held it")
	}
}

// Uploading a name that is already live used to be implicit in file:create; it
// destroys the existing file, so it needs file:delete.
func TestUploadSupersedingAFileRequiresFileDelete(t *testing.T) {
	withUploadServer(t)

	admin := mintSessionTokenWithPerms(t, allPermissions...)
	if recorder := doUpload(t, admin, "report.pdf", "v1"); recorder.Code != http.StatusCreated {
		t.Fatalf("seeding: status = %d (%s)", recorder.Code, recorder.Body.String())
	}

	recorder := doUpload(t, mintSessionTokenWithPerms(t, permFileCreate), "report.pdf", "v2")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := responseError(t, recorder); !strings.Contains(got, permFileDelete) {
		t.Errorf("error = %q, want it to name %s", got, permFileDelete)
	}
	if !liveByName(t, "report.pdf") {
		t.Fatal("the refused replacement destroyed the existing file")
	}

	// With file:delete the replacement goes through and reports what it did.
	recorder = doUpload(t, mintSessionTokenWithPerms(t, permFileCreate, permFileDelete), "report.pdf", "v2")
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", recorder.Code, recorder.Body.String())
	}
	var body UploadResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the upload response: %v", err)
	}
	if !body.Replaced {
		t.Error("Replaced = false, want true")
	}
}

// Superseding a file also drops the tags it still held, which is the same
// removal and costs the same permission.
func TestUploadSupersedingATaggedFileRequiresTagRemove(t *testing.T) {
	withUploadServer(t)

	admin := mintSessionTokenWithPerms(t, allPermissions...)
	if recorder := doUpload(t, admin, "tagged.txt", "v1", "keep:latest"); recorder.Code != http.StatusCreated {
		t.Fatalf("seeding: status = %d (%s)", recorder.Code, recorder.Body.String())
	}

	// Re-listing the tag needs tag:add, and moving it off the file being
	// superseded needs tag:remove - which this credential deliberately lacks.
	recorder := doUpload(t, mintSessionTokenWithPerms(t, permFileCreate, permFileDelete, permTagAdd), "tagged.txt", "v2", "keep:latest")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := responseError(t, recorder); !strings.Contains(got, permTagRemove) {
		t.Errorf("error = %q, want it to name %s", got, permTagRemove)
	}
}

// A failed upload must not take the file it was superseding with it. The record
// used to be retired when the upload was reserved, so a 413 left the existing
// file soft-deleted with its blob orphaned.
func TestFailedUploadKeepsTheSupersededFile(t *testing.T) {
	withUploadServer(t)

	admin := mintSessionTokenWithPerms(t, allPermissions...)
	if recorder := doUpload(t, admin, "big.bin", "small"); recorder.Code != http.StatusCreated {
		t.Fatalf("seeding: status = %d (%s)", recorder.Code, recorder.Body.String())
	}

	before, err := liveRecordByName("big.bin")
	if err != nil {
		t.Fatalf("liveRecordByName: %v", err)
	}
	if before == nil {
		t.Fatal("seeding left no live record")
	}

	// Over the cap, so the stream fails well after the record was reserved.
	maxContentLength = 4
	recorder := doUpload(t, admin, "big.bin", "far too large to fit")
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (%s)", recorder.Code, recorder.Body.String())
	}

	after, err := liveRecordByName("big.bin")
	if err != nil {
		t.Fatalf("liveRecordByName: %v", err)
	}
	if after == nil {
		t.Fatal("a failed upload destroyed the file it was superseding")
	}
	if after.Slug != before.Slug {
		t.Errorf("live slug = %q, want the original %q", after.Slug, before.Slug)
	}
	if after.Size != before.Size {
		t.Errorf("live size = %d, want the original %d", after.Size, before.Size)
	}
}

// ART_APPEND_ONLY removes the superseding path entirely, so a creator-only
// credential can upload a name that is already live.
func TestAppendOnlyUploadNeedsNoDeletePermission(t *testing.T) {
	withUploadServer(t)
	appendOnly = true

	admin := mintSessionTokenWithPerms(t, allPermissions...)
	if recorder := doUpload(t, admin, "keep.txt", "v1"); recorder.Code != http.StatusCreated {
		t.Fatalf("seeding: status = %d (%s)", recorder.Code, recorder.Body.String())
	}

	recorder := doUpload(t, mintSessionTokenWithPerms(t, permFileCreate), "keep.txt", "v2")
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", recorder.Code, recorder.Body.String())
	}
}

// Adding a tag that belongs to another file moves it, so the tag route needs
// tag:remove as well as the tag:add it already requires.
func TestAddTagRouteMovingATagRequiresTagRemove(t *testing.T) {
	withUploadServer(t)

	admin := mintSessionTokenWithPerms(t, allPermissions...)
	if recorder := doUpload(t, admin, "one.txt", "x", "cat:latest"); recorder.Code != http.StatusCreated {
		t.Fatalf("seeding the tag: status = %d (%s)", recorder.Code, recorder.Body.String())
	}
	if recorder := doUpload(t, admin, "two.txt", "x"); recorder.Code != http.StatusCreated {
		t.Fatalf("seeding the target: status = %d (%s)", recorder.Code, recorder.Body.String())
	}
	two, err := liveRecordByName("two.txt")
	if err != nil || two == nil {
		t.Fatalf("liveRecordByName(two.txt): %v", err)
	}

	handler := requireToken(requirePermission(permTagAdd, rejectWhenReadOnly(addFileTagsHandler)))
	request := func(token string) *httptest.ResponseRecorder {
		body := strings.NewReader(`{"tags":["cat:latest"]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/tags/"+two.Slug, body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		// The route variable is normally filled in by the router.
		req = mux.SetURLVars(req, map[string]string{"slug": two.Slug})

		recorder := httptest.NewRecorder()
		handler(recorder, req)
		return recorder
	}

	recorder := request(mintSessionTokenWithPerms(t, permTagAdd))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := responseError(t, recorder); !strings.Contains(got, permTagRemove) {
		t.Errorf("error = %q, want it to name %s", got, permTagRemove)
	}

	owner, err := tagOwner(db(), Tag{Name: "cat", Suffix: "latest"})
	if err != nil {
		t.Fatalf("tagOwner: %v", err)
	}
	if owner != "" && owner == two.Slug {
		t.Error("the refused request moved the tag anyway")
	}

	recorder = request(mintSessionTokenWithPerms(t, permTagAdd, permTagRemove))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
}
