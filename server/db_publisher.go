package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// snapshotPublisher publishes the database to the snapshot store once it has
// been quiet for a while.
//
// Writes are detected with sqlite's own total_changes counter, so no mutation
// path has to report anything, and the delay is a leading-edge timer: it starts
// when the database goes from clean to dirty and is not pushed back by further
// writes, which keeps a busy server publishing on a bounded schedule instead of
// never. Nothing is published while nothing changes - which matters, because the
// same object is what the replicas poll.
type snapshotPublisher struct {
	delay time.Duration
	path  string // staging file: <uploadFolder>/_backup/artifact.db

	mu         sync.Mutex
	published  int64     // total_changes at the last successful publish
	dirtySince time.Time // zero while nothing has changed since then
}

// newSnapshotPublisher prepares the staging directory and takes the baseline of
// the change counter, so a fresh boot does not immediately publish.
func newSnapshotPublisher() (*snapshotPublisher, error) {
	if err := os.MkdirAll(snapshotDir(), 0o755); err != nil {
		return nil, err
	}

	staging := filepath.Join(snapshotDir(), dbSnapshotName)
	// A process killed mid-snapshot can leave the temporary file behind; the
	// snapshot itself is rebuilt from scratch every time.
	if err := os.Remove(staging + ".tmp"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	baseline, err := dbTotalChanges()
	if err != nil {
		return nil, err
	}

	return &snapshotPublisher{
		delay:     dbBackupDelay,
		path:      staging,
		published: baseline,
	}, nil
}

// Run publishes until ctx is cancelled, then publishes once more if the last
// changes never made it out.
func (p *snapshotPublisher) Run(ctx context.Context) {
	ticker := time.NewTicker(dbBackupPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.flush()
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

// tick publishes when the database has been dirty for longer than the delay.
func (p *snapshotPublisher) tick(ctx context.Context) {
	changes, err := dbTotalChanges()
	if err != nil {
		log.Printf("Snapshot: reading the change counter failed: %v", err)
		return
	}

	p.mu.Lock()
	if changes == p.published {
		// Clean: the next write is what starts the window again.
		p.mu.Unlock()
		return
	}
	if p.dirtySince.IsZero() {
		p.dirtySince = time.Now()
	}
	due := time.Since(p.dirtySince) >= p.delay
	p.mu.Unlock()

	if !due {
		return
	}

	if err := p.publish(ctx); err != nil {
		// Restarting the window turns a failure into one retry per delay
		// rather than one per tick, which is the backoff this needs.
		p.mu.Lock()
		p.dirtySince = time.Now()
		p.mu.Unlock()
		log.Printf("Snapshot: publishing failed, will retry: %v", err)
		return
	}

	p.mu.Lock()
	p.dirtySince = time.Time{}
	p.mu.Unlock()
}

// publish writes one snapshot of the database to the snapshot store.
func (p *snapshotPublisher) publish(ctx context.Context) error {
	// Read the counter before taking the snapshot, so any write that lands while
	// the copy is being made keeps the database dirty for the next round.
	before, err := dbTotalChanges()
	if err != nil {
		return err
	}

	rows, err := countRecords()
	if err != nil {
		return err
	}
	reason, err := snapshotLooksEmpty(ctx, rows)
	if err != nil {
		return err
	}
	if reason != "" {
		return errors.New(reason)
	}

	if err := writeSnapshotTo(db(), p.path); err != nil {
		return err
	}

	file, err := os.Open(p.path)
	if err != nil {
		return err
	}
	defer file.Close()

	written, err := snapshotStore.Put(ctx, snapshotKey(), file, maxDBSnapshotSize)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.published = before
	p.mu.Unlock()

	log.Printf("Snapshot: published %s (%d bytes, %d records)", snapshotKey(), written, rows)
	return nil
}

// flush publishes outstanding changes on the way out, so a writer being retired
// does not leave its replicas a delay behind for ever. It is best effort: the
// live database remains the source of truth, and a missed publish costs the
// replicas freshness, not data.
func (p *snapshotPublisher) flush() {
	changes, err := dbTotalChanges()
	if err != nil {
		return
	}

	p.mu.Lock()
	outstanding := changes != p.published
	p.mu.Unlock()
	if !outstanding {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownPublishTimeout)
	defer cancel()

	if err := p.publish(ctx); err != nil {
		log.Printf("Snapshot: final publish failed: %v", err)
		return
	}
	log.Printf("Snapshot: published the last changes on shutdown")
}
