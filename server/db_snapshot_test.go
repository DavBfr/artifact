package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// restoreGlobals snapshots the package-level configuration and puts it back
// afterwards. Every test here drives the real boot path, which reads those
// globals, so one test's role or directory must not leak into the next.
func restoreGlobals(t *testing.T) {
	t.Helper()

	previous := struct {
		uploadFolder                  string
		readOnly, dbReplica           bool
		dbRestoreMode, dbBackupPrefix string
		dbBackupDelay, dbPollInterval time.Duration
		snapshotStore                 *s3Storage
		activeReplica                 *snapshotReplica
		activePublisher               *snapshotPublisher
	}{
		uploadFolder, readOnly, dbReplica, dbRestoreMode, dbBackupPrefix,
		dbBackupDelay, dbPollInterval, snapshotStore, activeReplica, activePublisher,
	}

	t.Cleanup(func() {
		closeDatabase()
		uploadFolder = previous.uploadFolder
		readOnly, dbReplica = previous.readOnly, previous.dbReplica
		dbRestoreMode, dbBackupPrefix = previous.dbRestoreMode, previous.dbBackupPrefix
		dbBackupDelay, dbPollInterval = previous.dbBackupDelay, previous.dbPollInterval
		snapshotStore = previous.snapshotStore
		activeReplica, activePublisher = previous.activeReplica, previous.activePublisher
	})
}

// insertTestRecord adds one live file record through the live handle.
func insertTestRecord(t *testing.T, name string) {
	t.Helper()
	insertRecordInto(t, db(), name)
}

func insertRecordInto(t *testing.T, handle *sql.DB, name string) {
	t.Helper()
	if _, err := insertUniqueRecord(handle, FileRecord{
		DisplayName: name,
		Size:        3,
		Modified:    time.Now().UTC().Format(time.RFC3339),
		MimeType:    "text/plain",
	}); err != nil {
		t.Fatalf("inserting %s: %v", name, err)
	}
}

// recordCount reports how many rows the live database holds.
func recordCount(t *testing.T) int64 {
	t.Helper()
	rows, err := countRecords()
	if err != nil {
		t.Fatalf("counting records: %v", err)
	}
	return rows
}

// publishSnapshotFrom writes a snapshot of handle into the store, which is what
// the publisher's publish() does apart from the empty-database guard.
func publishSnapshotFrom(t *testing.T, handle *sql.DB, dir string) {
	t.Helper()

	path := filepath.Join(dir, defaultDBBackupPrefix, dbSnapshotName)
	if err := writeSnapshotTo(handle, path); err != nil {
		t.Fatalf("writeSnapshotTo: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening the staging snapshot: %v", err)
	}
	defer file.Close()

	if _, err := snapshotStore.Put(context.Background(), snapshotKey(), file, maxDBSnapshotSize); err != nil {
		t.Fatalf("publishing the snapshot: %v", err)
	}
}

// bootWriter boots a writable instance in dir.
func bootWriter(t *testing.T, dir string) {
	t.Helper()
	uploadFolder = dir
	readOnly, dbReplica = false, false
	if err := bootDatabase(); err != nil {
		t.Fatalf("bootDatabase: %v", err)
	}
}

func TestBootStartsFreshWhenNothingIsPublished(t *testing.T) {
	restoreGlobals(t)
	store, _ := newFakeSnapshotStore()
	snapshotStore = store

	// A brand new deployment: no database, and nothing in the store. There is
	// nothing to lose, so starting empty is the right answer.
	bootWriter(t, t.TempDir())

	insertTestRecord(t, "one.txt")
	if got := recordCount(t); got != 1 {
		t.Fatalf("record count = %d, want 1", got)
	}
}

func TestBootRefusesWhenSnapshotExistsWithoutLocalDatabase(t *testing.T) {
	restoreGlobals(t)
	store, _ := newFakeSnapshotStore()
	snapshotStore = store

	sourceDir := t.TempDir()
	bootWriter(t, sourceDir)
	insertTestRecord(t, "one.txt")
	publishSnapshotFrom(t, db(), sourceDir)
	closeDatabase()

	// The same store, with no local database at all: this is what a lost or
	// unmounted volume looks like, and starting empty here would publish that
	// emptiness over the only good copy.
	uploadFolder = t.TempDir()
	err := bootDatabase()
	if err == nil {
		t.Fatal("boot accepted a missing database while a snapshot exists")
	}
	if !strings.Contains(err.Error(), "restore-db") {
		t.Fatalf("error does not point at the restore command: %v", err)
	}
}

func TestBootRestoreIgnoreStartsFresh(t *testing.T) {
	restoreGlobals(t)
	store, _ := newFakeSnapshotStore()
	snapshotStore = store

	sourceDir := t.TempDir()
	bootWriter(t, sourceDir)
	insertTestRecord(t, "one.txt")
	publishSnapshotFrom(t, db(), sourceDir)
	closeDatabase()

	uploadFolder = t.TempDir()
	dbRestoreMode = dbRestoreIgnore
	if err := bootDatabase(); err != nil {
		t.Fatalf("ART_DB_RESTORE=ignore did not allow a fresh start: %v", err)
	}
	if got := recordCount(t); got != 0 {
		t.Fatalf("record count = %d, want an empty database", got)
	}
}

func TestBootRestoreAutoDownloadsTheSnapshot(t *testing.T) {
	restoreGlobals(t)
	store, _ := newFakeSnapshotStore()
	snapshotStore = store

	sourceDir := t.TempDir()
	bootWriter(t, sourceDir)
	insertTestRecord(t, "one.txt")
	publishSnapshotFrom(t, db(), sourceDir)
	closeDatabase()

	uploadFolder = t.TempDir()
	dbRestoreMode = dbRestoreAuto
	if err := bootDatabase(); err != nil {
		t.Fatalf("bootDatabase: %v", err)
	}
	if got := recordCount(t); got != 1 {
		t.Fatalf("restored record count = %d, want 1", got)
	}
}

func TestReplicaBootsFromTheSnapshotStore(t *testing.T) {
	restoreGlobals(t)
	store, _ := newFakeSnapshotStore()
	snapshotStore = store

	sourceDir := t.TempDir()
	bootWriter(t, sourceDir)
	insertTestRecord(t, "one.txt")
	publishSnapshotFrom(t, db(), sourceDir)
	closeDatabase()

	// A replica has no volume of its own, so an absent database is its normal
	// state rather than an error.
	uploadFolder = t.TempDir()
	readOnly, dbReplica = true, true
	if err := bootDatabase(); err != nil {
		t.Fatalf("replica bootDatabase: %v", err)
	}
	if activeReplica == nil {
		t.Fatal("a replica boot did not leave a replica running")
	}
	if got := recordCount(t); got != 1 {
		t.Fatalf("replica record count = %d, want 1", got)
	}
}

func TestReplicaPicksUpANewSnapshot(t *testing.T) {
	restoreGlobals(t)
	store, _ := newFakeSnapshotStore()
	snapshotStore = store

	// Writer side, kept open for the whole test.
	sourceDir := t.TempDir()
	bootWriter(t, sourceDir)
	insertTestRecord(t, "one.txt")
	publishSnapshotFrom(t, db(), sourceDir)
	writer := db()

	// Replica side: its own directory, its own handle.
	replicaDir := t.TempDir()
	uploadFolder = replicaDir
	readOnly, dbReplica = true, true
	if err := bootDatabase(); err != nil {
		t.Fatalf("replica bootDatabase: %v", err)
	}
	if got := recordCount(t); got != 1 {
		t.Fatalf("replica record count = %d, want 1", got)
	}

	// Nothing new yet, so a refresh is a no-op.
	activeReplica.refresh(context.Background())
	if got := activeReplica.generation(); got != 0 {
		t.Fatalf("a refresh with no new snapshot bumped the generation to %d", got)
	}

	// The writer takes another upload and publishes again.
	insertRecordInto(t, writer, "two.txt")
	publishSnapshotFrom(t, writer, sourceDir)

	activeReplica.refresh(context.Background())

	if got := recordCount(t); got != 2 {
		t.Fatalf("replica record count after the update = %d, want 2", got)
	}
	if got := activeReplica.generation(); got != 1 {
		t.Fatalf("generation = %d, want 1", got)
	}
}

func TestReplicaKeepsServingWhenTheNewSnapshotIsCorrupt(t *testing.T) {
	restoreGlobals(t)
	store, fake := newFakeSnapshotStore()
	snapshotStore = store

	sourceDir := t.TempDir()
	bootWriter(t, sourceDir)
	insertTestRecord(t, "one.txt")
	publishSnapshotFrom(t, db(), sourceDir)
	writer := db()
	insertRecordInto(t, writer, "two.txt")

	uploadFolder = t.TempDir()
	readOnly, dbReplica = true, true
	if err := bootDatabase(); err != nil {
		t.Fatalf("replica bootDatabase: %v", err)
	}

	// Publish a truncated object: it changes, so the replica fetches it, and it
	// is not a database, so the replica must not adopt it.
	publishSnapshotFrom(t, writer, sourceDir)
	key := store.prefix + snapshotKey()
	fake.objects[key] = fake.objects[key][:128]

	activeReplica.refresh(context.Background())

	if got := recordCount(t); got != 1 {
		t.Fatalf("record count = %d, want the previous snapshot's 1", got)
	}
	if got := activeReplica.generation(); got != 0 {
		t.Fatalf("a corrupt snapshot was adopted (generation %d)", got)
	}
}

func TestSnapshotIsSelfContainedAndReadOnly(t *testing.T) {
	restoreGlobals(t)

	dir := t.TempDir()
	bootWriter(t, dir)
	insertTestRecord(t, "one.txt")

	path := filepath.Join(dir, defaultDBBackupPrefix, dbSnapshotName)
	if err := writeSnapshotTo(db(), path); err != nil {
		t.Fatalf("writeSnapshotTo: %v", err)
	}

	// One file, no sidecars: that is what makes the copy safe to hand to another
	// process with no coordination at all.
	for _, sidecar := range []string{path + "-wal", path + "-shm"} {
		if _, err := os.Stat(sidecar); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("the snapshot left a sidecar at %s", sidecar)
		}
	}

	// Opens through the production read-only DSN, so this also covers mode=ro and
	// the pragmas that have to be skipped for that role.
	handle, err := openSQLite(path, true)
	if err != nil {
		t.Fatalf("opening the snapshot read-only failed: %v", err)
	}
	defer handle.Close()

	var journal string
	if err := handle.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("reading journal_mode: %v", err)
	}
	if journal != "delete" {
		t.Fatalf("snapshot journal_mode = %q, want delete", journal)
	}

	var check string
	if err := handle.QueryRow("PRAGMA quick_check").Scan(&check); err != nil {
		t.Fatalf("running quick_check: %v", err)
	}
	if check != "ok" {
		t.Fatalf("quick_check = %q", check)
	}

	// The property the whole replica design leans on: nothing it can do alters
	// the data.
	if _, err := handle.Exec(
		"INSERT INTO files (slug, display_name, storage_key) VALUES ('zzz', 'x', 'zz/z')",
	); err == nil {
		t.Fatal("a write through the read-only handle succeeded")
	}

	// A reader still runs the schema statements on boot, so they must be no-ops
	// on a snapshot rather than an attempted write.
	if err := initSchema(handle); err != nil {
		t.Fatalf("initSchema on a read-only snapshot failed: %v", err)
	}

	var rows int64
	if err := handle.QueryRow("SELECT count(*) FROM files").Scan(&rows); err != nil {
		t.Fatalf("counting records: %v", err)
	}
	if rows != 1 {
		t.Fatalf("snapshot holds %d records, want 1", rows)
	}
}

func TestVerifySnapshotRejectsAnUnreadableFile(t *testing.T) {
	restoreGlobals(t)

	dir := t.TempDir()
	bootWriter(t, dir)

	truncated := filepath.Join(dir, "truncated.db")
	if err := os.WriteFile(truncated, []byte("this is not a database, not even close"), 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	if err := verifySnapshot(truncated); err == nil {
		t.Fatal("verifySnapshot accepted a file that is not a database")
	}
	if err := verifySnapshot(filepath.Join(dir, "absent.db")); err == nil {
		t.Fatal("verifySnapshot accepted a file that does not exist")
	}
}

func TestVerifySnapshotRejectsAMismatchedSchemaVersion(t *testing.T) {
	restoreGlobals(t)

	dir := t.TempDir()
	bootWriter(t, dir)
	insertTestRecord(t, "one.txt")

	path := filepath.Join(dir, "future.db")
	if err := writeSnapshotTo(db(), path); err != nil {
		t.Fatalf("writeSnapshotTo: %v", err)
	}

	handle, err := openSQLite(path, false)
	if err != nil {
		t.Fatalf("opening the snapshot: %v", err)
	}
	if _, err := handle.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatalf("stamping a future version: %v", err)
	}
	handle.Close()

	// A build that cannot read a snapshot must say so at load time rather than
	// fail on a missing column at request time.
	if err := verifySnapshot(path); err == nil {
		t.Fatal("verifySnapshot accepted a snapshot from a newer schema")
	}
}

func TestSchemaVersionIsStampedOnAnExistingDatabase(t *testing.T) {
	restoreGlobals(t)

	dir := t.TempDir()
	bootWriter(t, dir)

	var stamped int
	if err := db().QueryRow("PRAGMA user_version").Scan(&stamped); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if stamped != schemaVersion {
		t.Fatalf("user_version = %d, want %d", stamped, schemaVersion)
	}

	// A database from a newer build is a downgrade, and is refused.
	handle, err := openSQLite(filepath.Join(dir, dbFileName), false)
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	defer handle.Close()
	if _, err := handle.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatalf("stamping a future version: %v", err)
	}
	if err := checkSchemaVersion(handle, false); err == nil {
		t.Fatal("a database from a newer schema was accepted by a writer")
	}
}

func TestSnapshotPublisherIsSparse(t *testing.T) {
	restoreGlobals(t)
	store, fake := newFakeSnapshotStore()
	snapshotStore = store

	bootWriter(t, t.TempDir())

	dbBackupDelay = 50 * time.Millisecond
	publisher, err := newSnapshotPublisher()
	if err != nil {
		t.Fatalf("newSnapshotPublisher: %v", err)
	}

	ctx := context.Background()

	// Nothing has changed, so nothing is published however often it looks.
	for range 5 {
		publisher.tick(ctx)
	}
	if fake.puts != 0 {
		t.Fatalf("published %d snapshots with nothing changed", fake.puts)
	}

	insertTestRecord(t, "one.txt")
	publisher.tick(ctx)
	if fake.puts != 0 {
		t.Fatalf("published before the quiet period had elapsed (%d puts)", fake.puts)
	}

	time.Sleep(2 * dbBackupDelay)
	publisher.tick(ctx)
	if fake.puts != 1 {
		t.Fatalf("published %d snapshots after one change, want 1", fake.puts)
	}

	// Idle again: the delay is leading edge, so further writes are what make it
	// publish, not the passage of time.
	for range 5 {
		publisher.tick(ctx)
	}
	if fake.puts != 1 {
		t.Fatalf("published %d snapshots while idle, want 1", fake.puts)
	}

	insertTestRecord(t, "two.txt")

	// The window starts when the change is *noticed*, which is what the poll
	// interval bounds: the first tick after a write starts the clock, and the
	// delay has to elapse before a later tick publishes.
	publisher.tick(ctx)
	if fake.puts != 1 {
		t.Fatalf("published %d snapshots without waiting out the second change", fake.puts)
	}

	time.Sleep(2 * dbBackupDelay)
	publisher.tick(ctx)
	if fake.puts != 2 {
		t.Fatalf("published %d snapshots after a second change, want 2", fake.puts)
	}
}

func TestSnapshotPublisherRefusesToOverwriteWithAnEmptyDatabase(t *testing.T) {
	restoreGlobals(t)
	store, fake := newFakeSnapshotStore()

	// A writer with records publishes a snapshot.
	sourceDir := t.TempDir()
	snapshotStore = store
	bootWriter(t, sourceDir)
	insertTestRecord(t, "one.txt")
	publishSnapshotFrom(t, db(), sourceDir)
	closeDatabase()

	// An empty database, as a wiped volume would leave: boot without a store so
	// the boot policy has nothing to refuse, then bring the store back.
	uploadFolder = t.TempDir()
	snapshotStore = nil
	bootWriter(t, uploadFolder)
	snapshotStore = store

	dbBackupDelay = time.Millisecond
	publisher, err := newSnapshotPublisher()
	if err != nil {
		t.Fatalf("newSnapshotPublisher: %v", err)
	}

	err = publisher.publish(context.Background())
	if err == nil {
		t.Fatal("an empty database was published over a snapshot holding data")
	}
	if !strings.Contains(err.Error(), "ART_DB_RESTORE=ignore") {
		t.Fatalf("the refusal does not say how to override it: %v", err)
	}
	if fake.puts != 1 {
		t.Fatalf("the good snapshot was overwritten (%d puts, want the original 1)", fake.puts)
	}

	// The override exists for the operator who emptied the database on purpose.
	dbRestoreMode = dbRestoreIgnore
	if err := publisher.publish(context.Background()); err != nil {
		t.Fatalf("publishing with the override set failed: %v", err)
	}
}

func TestReadOnlyHandleReadsUncheckpointedWriteAheadLog(t *testing.T) {
	restoreGlobals(t)

	dir := t.TempDir()
	bootWriter(t, dir)
	insertTestRecord(t, "one.txt")
	insertTestRecord(t, "two.txt")

	// The writes are in the -wal, not the main file: that is the state a process
	// killed before it could checkpoint leaves behind, and it is why a read-only
	// handle must not be opened immutable. immutable=1 tells sqlite to ignore the
	// log outright, which does not fail - it quietly serves the stale main file.
	if _, err := os.Stat(filepath.Join(dir, dbFileName+"-wal")); err != nil {
		t.Fatalf("expected a write-ahead log beside the database: %v", err)
	}

	// The writer's own handle stays open, so the log is never checkpointed away.
	reader, err := openSQLite(filepath.Join(dir, dbFileName), true)
	if err != nil {
		t.Fatalf("opening the database read-only failed: %v", err)
	}
	defer reader.Close()

	var rows int64
	if err := reader.QueryRow("SELECT count(*) FROM files").Scan(&rows); err != nil {
		t.Fatalf("counting records through the read-only handle: %v", err)
	}
	if rows != 2 {
		t.Fatalf("a read-only handle sees %d records, want 2", rows)
	}

	// The schema statements a read-only boot runs must be no-ops here too, rather
	// than an attempted write against a log-mode database.
	if err := initSchema(reader); err != nil {
		t.Fatalf("initSchema against a read-only database with a write-ahead log failed: %v", err)
	}
}

func TestSnapshotRoundTripsThroughTheRealS3Client(t *testing.T) {
	for _, overTLS := range []bool{false, true} {
		transport := "http"
		if overTLS {
			transport = "https"
		}

		t.Run(transport, func(t *testing.T) {
			restoreGlobals(t)

			// The stub goes through the real SDK client and its signing, which is
			// the part the in-process fake cannot exercise. The snapshot body is
			// a file rather than a stream, so it is worth confirming the framing
			// works for it too.
			store, _ := newStubBackedStorageWith(t, overTLS)
			store.prefix = normaliseS3Prefix(defaultDBBackupPrefix)
			snapshotStore = store

			sourceDir := t.TempDir()
			bootWriter(t, sourceDir)
			insertTestRecord(t, "one.txt")
			insertTestRecord(t, "two.txt")
			publishSnapshotFrom(t, db(), sourceDir)
			closeDatabase()

			uploadFolder = t.TempDir()
			readOnly, dbReplica = true, true
			if err := bootDatabase(); err != nil {
				t.Fatalf("replica boot over %s: %v", transport, err)
			}
			if got := recordCount(t); got != 2 {
				t.Fatalf("replica record count = %d, want 2", got)
			}
		})
	}
}

func TestReadOnlyInstancesNeverPublish(t *testing.T) {
	restoreGlobals(t)
	store, fake := newFakeSnapshotStore()
	snapshotStore = store

	sourceDir := t.TempDir()
	bootWriter(t, sourceDir)
	insertTestRecord(t, "one.txt")
	publishSnapshotFrom(t, db(), sourceDir)
	closeDatabase()

	uploadFolder = t.TempDir()
	readOnly, dbReplica = true, true
	if err := bootDatabase(); err != nil {
		t.Fatalf("replica bootDatabase: %v", err)
	}

	// A delay inherited from a shared configuration environment must not start a
	// publisher here: the writer is the only thing that publishes.
	dbBackupDelay = time.Second
	var background sync.WaitGroup
	runSnapshotPublisher(&background)

	if activePublisher != nil {
		t.Fatal("a read-only instance started a snapshot publisher")
	}
	if fake.puts != 1 {
		t.Fatalf("puts = %d, want only the original snapshot", fake.puts)
	}

	background.Wait()
}

func TestRejectWhenReadOnlyBlocksMutations(t *testing.T) {
	restoreGlobals(t)

	reached := false
	handler := rejectWhenReadOnly(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	})

	readOnly = true
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodPost, "/api/upload", nil))

	if reached {
		t.Fatal("a read-only instance ran the mutating handler")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "read-only") {
		t.Fatalf("body does not explain the refusal: %s", recorder.Body.String())
	}

	readOnly = false
	recorder = httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodPost, "/api/upload", nil))

	if !reached {
		t.Fatal("a writable instance refused the mutating handler")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}
