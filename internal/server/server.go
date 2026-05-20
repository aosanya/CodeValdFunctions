// Package server implements the FunctionsService gRPC handlers.
package server

import (
	pb "github.com/aosanya/CodeValdFunctions/gen/go/codevaldelfunctions/v1"
)

// Server implements pb.FunctionsServiceServer.
// Job management RPCs (ListJobs, GetJob, CancelJob) are added in MVP-FN-006.
type Server struct {
	pb.UnimplementedFunctionsServiceServer
}

// New constructs a Server.
func New() *Server {
	return &Server{}
}
