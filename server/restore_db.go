package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"
)

// runRestoreCommand implements the `restore-db` subcommand, which writes the
// published snapshot over the local database.
//
// This is deliberately a one-shot command rather than something the server does
// on its own at startup. Replacing a database is the one action here with no way
// back, and "the volume was lost" and "this is a new deployment" are not things
// a program can reliably tell apart - so the server refuses to start in that
// situation and points at this command instead. It also refuses to overwrite an
// existing database without -force, for the same reason.
func runRestoreCommand(args []string) int {
	fs := flag.NewFlagSet("restore-db", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	force := fs.Bool("force", false, "replace an existing local database")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Restore the published database snapshot into the local data directory.

Usage:
  upload_server restore-db [-force]

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), snapshotOperationTimeout)
	defer cancel()

	if err := initSnapshotStore(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if !snapshotConfigured() {
		fmt.Fprintln(os.Stderr, "error: ART_S3_BUCKET is not set, so there is no snapshot to restore from")
		return 1
	}

	info, err := headSnapshot(ctx)
	if err != nil {
		if errors.Is(err, errBlobNotFound) {
			fmt.Fprintf(os.Stderr, "error: no snapshot published at %s\n", snapshotKey())
			return 1
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	path := databasePath()
	if _, err := os.Stat(path); err == nil && !*force {
		fmt.Fprintf(os.Stderr,
			"error: %s already exists\n  Re-run with -force to replace it.\n", path)
		return 1
	}
	if err := os.MkdirAll(uploadFolder, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if _, err := downloadSnapshot(ctx, path); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("Restored %s (%d bytes, published %s) to %s\n",
		snapshotKey(), info.Size, info.ModTime.UTC().Format(time.RFC3339), path)
	return 0
}
