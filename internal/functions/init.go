package functions

import (
	"context"
	"log"

	codevaldelfunctions "github.com/aosanya/CodeValdFunctions"
)

// RegisterAll registers all pre-built function handlers with reg.
// Called once at startup before the gRPC server begins serving.
func RegisterAll(reg Registry) {
	reg.Register("work.task.completed", handleWorkTaskCompleted)
}

// handleWorkTaskCompleted logs the incoming payload. Replace with domain
// logic once the function business requirement is fully defined.
func handleWorkTaskCompleted(_ context.Context, job codevaldelfunctions.Job, payload []byte) ([]byte, error) {
	log.Printf("functions: work.task.completed: job_id=%s payload=%s", job.ID, payload)
	return []byte(`{"status":"logged"}`), nil
}
