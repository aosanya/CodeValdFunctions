// Package registrar provides the CodeValdFunctions heartbeat registrar and
// CrossPublisher implementation.
package registrar

import (
	"context"
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
		[]string{"functions.job.completed", "functions.job.failed"},
		[]string{},
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
// Errors are logged but not returned — the triggering operation is already
// persisted.
func (r *Registrar) Publish(_ context.Context, e eventbus.Event) error {
	log.Printf("registrar[codevaldelfunctions]: publish topic=%q agencyID=%q", e.Topic, e.AgencyID)
	// TODO(MVP-FN): call OrchestratorService.Publish RPC when wired.
	return nil
}

// functionsRoutes returns HTTP routes CodeValdFunctions exposes via Cross.
// Job gRPC API routes are added in MVP-FN-006.
func functionsRoutes() []types.RouteInfo {
	return []types.RouteInfo{}
}
