// Package server implements the FunctionsService gRPC handlers.
package server

import (
	codevaldelfunctions "github.com/aosanya/CodeValdFunctions"
	pb "github.com/aosanya/CodeValdFunctions/gen/go/codevaldelfunctions/v1"
)

// Server implements pb.FunctionsServiceServer.
// Job management RPCs (ListJobs, GetJob, CancelJob) are added in MVP-FN-006.
type Server struct {
	pb.UnimplementedFunctionsServiceServer
	mgr codevaldelfunctions.FunctionsManager
}

// New constructs a Server backed by the given FunctionsManager.
func New(mgr codevaldelfunctions.FunctionsManager) *Server {
	return &Server{mgr: mgr}
}
