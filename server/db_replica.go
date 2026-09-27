package main

import (
	"context"
	"errors"
	"log"
	"math/rand/v2"
	"sync"
	"time"
)

// snapshotReplica keeps a read-only instance in step with the writer by polling
// the published snapshot.
//
// It never writes: each round it asks the store whether the object changed,
// downloads it if so, verifies it, and swaps it in. Every failure path keeps the
// snapshot already being served, so a bad download or an unreachable store costs
// freshness rather than availability - and because the handle it serves from is
// opened mode=ro, nothing it does can alter the data itself.
type snapshotReplica struct {
	interval time.Duration
	path     string

	mu          sync.RWMutex
	current     BlobInfo // the version the live handle came from
	loadedAt    time.Time
	generations uint64
}

// Run refreshes the local snapshot until ctx is cancelled.
func (r *snapshotReplica) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(r.nextInterval()):
			r.refresh(ctx)
		}
	}
}

// nextInterval returns the poll interval with jitter, so replicas sharing one
// configuration do not all fetch a new snapshot in the same instant.
func (r *snapshotReplica) nextInterval() time.Duration {
	factor := 0.9 + 0.2*rand.Float64()
	return time.Duration(float64(r.interval) * factor)
}

// refresh picks up a newly published snapshot, if there is one.
func (r *snapshotReplica) refresh(ctx context.Context) {
	info, err := headSnapshot(ctx)
	if err != nil {
		if errors.Is(err, errBlobNotFound) {
			log.Printf("Snapshot: nothing published at %s yet", snapshotKey())
			return
		}
		log.Printf("Snapshot: checking for a new version failed: %v", err)
		return
	}

	r.mu.RLock()
	current := r.current
	r.mu.RUnlock()
	if sameSnapshot(info, current) {
		return
	}

	// downloadSnapshot only renames a verified file over the live path, so the
	// database already being served survives every failure below this point.
	if _, err := downloadSnapshot(ctx, r.path); err != nil {
		log.Printf("Snapshot: updating to the published version failed: %v", err)
		return
	}

	handle, err := openSQLite(r.path, true)
	if err != nil {
		log.Printf("Snapshot: opening the new version failed: %v", err)
		return
	}

	replaceDatabase(handle)

	r.mu.Lock()
	r.current = info
	r.loadedAt = time.Now()
	r.generations++
	generations := r.generations
	r.mu.Unlock()

	log.Printf("Snapshot: serving generation %d (%d bytes)", generations, info.Size)
}

// generation counts how many snapshots this instance has loaded.
func (r *snapshotReplica) generation() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.generations
}

// age reports how long ago the snapshot now being served was fetched, which is
// what an operator watching a replica actually needs: the instance is healthy at
// any age, but the age is the lag behind the writer.
func (r *snapshotReplica) age() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return time.Since(r.loadedAt)
}
