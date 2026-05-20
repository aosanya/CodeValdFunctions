// Package registrar provides the CodeValdFunctions heartbeat registrar and
// CrossPublisher implementation.
package registrar

import (
	"context"
	"encoding/json"
	"log"
	"time"

	codevaldelfunctions "github.com/aosanya/CodeValdFunctions"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
	sharedregistrar "github.com/aosanya/CodeValdSharedLib/registrar"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// Registrar sends periodic heartbeat registrations to CodeValdCross and
// implements [codevaldelfunctions.CrossPublisher] for event publishing.
type Registrar struct {
	heartbeat sharedregistrar.Registrar
}

// Compile-time assertion that *Registrar implements CrossPublisher.
var _ codevaldelfunctions.CrossPublisher = (*Registrar)(nil)

// New constructs a Registrar that heartbeats to the CodeValdCross gRPC server.
//
//   - crossAddr     — host:port of the CodeValdCross gRPC server
//   - advertiseAddr — host:port that Cross dials back on
//   - agencyID      — agency this instance serves
//   - pingInterval  — heartbeat cadence; ≤ 0 means only the initial ping
//   - pingTimeout   — per-RPC timeout for each Register call
func New(
	crossAddr, advertiseAddr, agencyID string,
	pingInterval, pingTimeout time.Duration,
) (*Registrar, error) {
	hb, err := sharedregistrar.New(
		crossAddr,
		advertiseAddr,
		agencyID,
		"codevaldelfunctions",
		[]string{"functions.job.created", "functions.job.started", "functions.job.completed", "functions.job.failed"},
		[]string{"work.task.completed"},
		functionsRoutes(),
		pingInterval,
		pingTimeout,
	)
	if err != nil {
		return nil, err
	}
	return &Registrar{heartbeat: hb}, nil
}

// Run starts the heartbeat loop. Must be called inside a goroutine.
func (r *Registrar) Run(ctx context.Context) {
	r.heartbeat.Run(ctx)
}

// Close releases the gRPC connection used for heartbeats.
func (r *Registrar) Close() {
	r.heartbeat.Close()
}

// Publish implements [codevaldelfunctions.CrossPublisher].
func (r *Registrar) Publish(ctx context.Context, e eventbus.Event) error {
	payload := ""
	switch p := e.Payload.(type) {
	case string:
		payload = p
	case map[string]string:
		b, _ := json.Marshal(p)
		payload = string(b)
	}
	log.Printf("registrar[codevaldelfunctions]: publish topic=%q agencyID=%q", e.Topic, e.AgencyID)
	return r.heartbeat.Publish(ctx, e.AgencyID, e.Topic, "codevaldelfunctions", payload)
}

// functionsRoutes returns the HTTP routes CodeValdFunctions exposes via Cross.
func functionsRoutes() []types.RouteInfo {
	agency := types.PathBinding{URLParam: "agencyId", Field: "agency_id"}
	return []types.RouteInfo{
		{
			Method:       "GET",
			Pattern:      "/functions/{agencyId}/jobs",
			Capability:   "list_jobs",
			GrpcMethod:   "/codevaldelfunctions.v1.FunctionsService/ListJobs",
			PathBindings: []types.PathBinding{agency},
		},
		{
			Method:     "GET",
			Pattern:    "/functions/{agencyId}/jobs/{jobId}",
			Capability: "get_job",
			GrpcMethod: "/codevaldelfunctions.v1.FunctionsService/GetJob",
			PathBindings: []types.PathBinding{
				agency,
				{URLParam: "jobId", Field: "job_id"},
			},
		},
		{
			Method:     "DELETE",
			Pattern:    "/functions/{agencyId}/jobs/{jobId}",
			Capability: "cancel_job",
			GrpcMethod: "/codevaldelfunctions.v1.FunctionsService/CancelJob",
			IsWrite:    true,
			PathBindings: []types.PathBinding{
				agency,
				{URLParam: "jobId", Field: "job_id"},
			},
		},
	}
}
