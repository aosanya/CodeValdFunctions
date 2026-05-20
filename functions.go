package codevaldelfunctions

import "context"

// FunctionsManager is the top-level interface for the CodeValdFunctions domain.
// Job lifecycle methods are added in MVP-FN-003.
// Event dispatch is wired in MVP-FN-004 and MVP-FN-005.
type FunctionsManager interface {
	// Ping verifies the manager is healthy. Used by the gRPC health service.
	Ping(ctx context.Context) error
}
