package main

import (
	"encoding/json"
	"net/http"
)

type HealthResponse struct {
	Status   string          `json:"status"`
	Service  string          `json:"service"`
	Role     string          `json:"role"`
	ReadOnly bool            `json:"read_only,omitempty"`
	Database *DatabaseHealth `json:"database,omitempty"`
}

// DatabaseHealth describes the local database copy. On a read-only replica it
// also carries how far behind the writer it is: a replica is healthy at any age,
// but the age is the number an alert should watch, so it is reported rather than
// hidden.
type DatabaseHealth struct {
	SchemaVersion int      `json:"schema_version"`
	Snapshot      string   `json:"snapshot,omitempty"`
	Generation    uint64   `json:"generation,omitempty"`
	AgeSeconds    *float64 `json:"snapshot_age_seconds,omitempty"`
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{
		Status:   "healthy",
		Service:  "upload-server",
		Role:     roleName(),
		ReadOnly: readOnly,
		Database: databaseHealth(),
	})
}
