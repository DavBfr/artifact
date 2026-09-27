package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// schemaVersion is what this build expects to find in PRAGMA user_version. The
// stamp lives inside the database file itself, so a snapshot published by one
// instance is self-describing for whichever instance picks it up - no sidecar
// object, no extra request, and nothing to keep in step by hand.
//
// Bump it whenever a change to initSchema would break an older build reading the
// same file. That is what lets a replica refuse a snapshot it cannot serve
// instead of failing on a missing column at request time.
const schemaVersion = 1

// dbSwapGracePeriod is how long a replaced database handle stays usable. A live
// replica swaps handles whenever it picks up a new snapshot, and the queries
// already running against the old one have to finish somewhere.
const dbSwapGracePeriod = time.Minute

// dbRef holds the live database handle. A read-only replica replaces it on every
// snapshot, so every query goes through db() rather than holding a handle that
// may be swapped underneath it.
var dbRef atomic.Pointer[sql.DB]

// db returns the database handle to query.
func db() *sql.DB { return dbRef.Load() }

// databasePath is where the sqlite file lives: directly in the upload folder, so
// a single volume holds both the records and, with filesystem storage, the
// blobs they point at.
func databasePath() string { return filepath.Join(uploadFolder, dbFileName) }

// sqliteDSN builds the connection string for path.
//
// A read-only instance opens with mode=ro and deliberately NOT immutable=1.
// immutable=1 is a promise that the file never changes, and sqlite acts on it by
// ignoring the write-ahead log completely. That is fine for a snapshot, which is
// written once in delete journal mode with no sidecars, but silently wrong for a
// live database that has an uncheckpointed -wal beside it: the main file alone
// is then stale, and a read-only instance would serve data missing every write
// still in the log. Since a read-only instance can be pointed at either, mode=ro
// on its own is the safe choice - it reads the log when there is one.
func sqliteDSN(path string, readOnlyDB bool) string {
	if !readOnlyDB {
		return path
	}
	return "file:" + path + "?mode=ro"
}

// openSQLite opens one connection pool for the sqlite file and applies the
// pragmas the handle's role allows.
func openSQLite(path string, readOnlyDB bool) (*sql.DB, error) {
	handle, err := sql.Open("sqlite", sqliteDSN(path, readOnlyDB))
	if err != nil {
		return nil, err
	}

	// SQLite allows one writer at a time, so a single pooled connection
	// serializes access instead of the callers racing on SQLITE_BUSY. The
	// snapshot writer's dirty counter (total_changes) is per connection, which
	// is only a usable global count because of this.
	handle.SetMaxOpenConns(1)

	// journal_mode is skipped when read-only: changing it rewrites the database
	// header, which fails with "attempt to write a readonly database" on a file
	// opened mode=ro. A snapshot is already in delete mode, so it needs no WAL.
	pragmas := "PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;"
	if !readOnlyDB {
		pragmas = "PRAGMA journal_mode=WAL; " + pragmas
	}
	if _, err := handle.Exec(pragmas); err != nil {
		handle.Close()
		return nil, fmt.Errorf("configuring database: %w", err)
	}

	return handle, nil
}

// bootDatabase decides what the local database should be, opens it, and applies
// the schema.
//
// The two roles differ in what an absent local file means. For a writer it is
// either a brand new deployment or a lost volume, and only the snapshot store
// can tell those apart - so a missing file *with* a snapshot published is
// refused rather than quietly replaced by an empty database that would then be
// published over the only good copy. For a read-only replica an absent file is
// the normal state: it downloads the newest snapshot and serves that.
func bootDatabase() error {
	ctx, cancel := context.WithTimeout(context.Background(), snapshotOperationTimeout)
	defer cancel()

	path := databasePath()
	_, statErr := os.Stat(path)
	haveLocal := statErr == nil

	switch {
	case readOnly:
		if haveLocal {
			log.Printf("Read-only instance using the snapshot already at %s", path)
			break
		}
		if !snapshotConfigured() {
			return errors.New("read-only instance has no database and no snapshot to download: set ART_S3_BUCKET")
		}
		info, err := downloadSnapshot(ctx, path)
		if err != nil {
			return err
		}
		activeReplica = &snapshotReplica{
			interval: dbPollInterval,
			path:     path,
			current:  info,
			loadedAt: time.Now(),
		}
		log.Printf("Read-only instance downloaded snapshot %s (%d bytes, %s)",
			snapshotKey(), info.Size, info.ModTime.UTC().Format(time.RFC3339))

	case haveLocal:
		// Nothing to decide: the local database is the source of truth.

	default:
		if err := startWithoutDatabase(ctx, path); err != nil {
			return err
		}
	}

	handle, err := openSQLite(path, readOnly)
	if err != nil {
		return err
	}
	if err := initSchema(handle); err != nil {
		handle.Close()
		return fmt.Errorf("initializing schema: %w", err)
	}
	if err := checkSchemaVersion(handle, readOnly); err != nil {
		handle.Close()
		return err
	}

	dbRef.Store(handle)
	log.Printf("Database: %s (role %s, schema v%d)", path, roleName(), schemaVersion)
	return nil
}

// startWithoutDatabase handles a writer whose database file is absent, which on
// its own is ambiguous: a first boot and a lost volume look identical.
func startWithoutDatabase(ctx context.Context, path string) error {
	switch dbRestoreMode {
	case dbRestoreIgnore:
		log.Printf("No database at %s; starting a new one (ART_DB_RESTORE=ignore)", path)
		return nil

	case dbRestoreAuto:
		if !snapshotConfigured() {
			log.Printf("No database at %s and no snapshot store configured; starting a new one", path)
			return nil
		}
		info, err := downloadSnapshot(ctx, path)
		if err != nil {
			return err
		}
		log.Printf("No database at %s; restored the published snapshot (%d bytes, %s)",
			path, info.Size, info.ModTime.UTC().Format(time.RFC3339))
		return nil
	}

	if !snapshotConfigured() {
		// Nothing to lose, and no way to tell a lost volume from a new one.
		log.Printf("No database at %s and no snapshot store configured; starting a new one", path)
		return nil
	}

	info, err := headSnapshot(ctx)
	if err != nil {
		if errors.Is(err, errBlobNotFound) {
			log.Printf("No database at %s and nothing published yet; starting a new one", path)
			return nil
		}
		// Unreachable is not the same as absent, and the consequences are
		// asymmetric: starting empty would overwrite the good copy on the first
		// publish, so an unanswerable question stops the boot instead.
		return fmt.Errorf("no database at %s and the snapshot store could not be reached: %w", path, err)
	}

	return fmt.Errorf(`no database at %s, but a snapshot is published at %s (%d bytes, %s)

  Restore it:   upload_server restore-db
  Start fresh:  ART_DB_RESTORE=ignore`,
		path, snapshotKey(), info.Size, info.ModTime.UTC().Format(time.RFC3339))
}

// checkSchemaVersion compares the version stamped in the file against the one
// this build writes.
//
// A replica refuses a mismatch outright: it cannot migrate, and serving a
// snapshot it does not understand would mean failing on a missing column at
// request time instead of refusing at load time. A writer adopts an older file
// (there is nothing to migrate yet, so the schema already means what the stamp
// says) and refuses a newer one, which would be a downgrade.
func checkSchemaVersion(handle *sql.DB, readOnlyDB bool) error {
	var version int
	if err := handle.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if version == schemaVersion {
		return nil
	}

	if readOnlyDB {
		return fmt.Errorf("snapshot is schema v%d, but this build reads v%d", version, schemaVersion)
	}
	if version > schemaVersion {
		return fmt.Errorf("database schema v%d is newer than this build's v%d", version, schemaVersion)
	}

	if _, err := handle.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("stamping schema version: %w", err)
	}
	log.Printf("Stamped database schema version v%d (was v%d)", schemaVersion, version)
	return nil
}

// dbTotalChanges reports how many rows this connection has changed since it was
// opened. It is sqlite's own counter, so the snapshot writer detects writes
// without any of the mutation paths having to report them.
func dbTotalChanges() (int64, error) {
	var changes int64
	if err := db().QueryRow("SELECT total_changes()").Scan(&changes); err != nil {
		return 0, err
	}
	return changes, nil
}

// countRecords reports how many rows the files table holds, tombstones
// included. Deletions are soft, so this only reaches zero for a database that
// never held anything.
func countRecords() (int64, error) {
	var rows int64
	if err := db().QueryRow("SELECT count(*) FROM files").Scan(&rows); err != nil {
		return 0, err
	}
	return rows, nil
}

// replaceDatabase swaps in a freshly opened handle. The old one is closed after
// a grace period rather than immediately: Close waits for queries that have
// started, and the delay additionally covers a query that has been handed the
// old handle but has not started yet. A replica only ever swaps while it has a
// complete new snapshot ready, so nothing is ever served from a missing file.
func replaceDatabase(next *sql.DB) {
	old := dbRef.Swap(next)
	if old == nil {
		return
	}
	time.AfterFunc(dbSwapGracePeriod, func() { _ = old.Close() })
}

// closeDatabase closes the live handle, if any.
func closeDatabase() {
	if handle := dbRef.Swap(nil); handle != nil {
		_ = handle.Close()
	}
}

// roleName describes what this instance does with the data.
func roleName() string {
	if readOnly {
		return "read-only"
	}
	return "writer"
}
