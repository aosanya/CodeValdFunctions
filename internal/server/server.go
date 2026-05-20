// Package server implements the FunctionsService gRPC handlers.
package server

import (
	"context"
	"errors"

	codevaldelfunctions "github.com/aosanya/CodeValdFunctions"
	pb "github.com/aosanya/CodeValdFunctions/gen/go/codevaldelfunctions/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements pb.FunctionsServiceServer.
type Server struct {
	pb.UnimplementedFunctionsServiceServer
	mgr codevaldelfunctions.FunctionsManager
}

// New constructs a Server backed by the given FunctionsManager.
func New(mgr codevaldelfunctions.FunctionsManager) *Server {
	return &Server{mgr: mgr}
}

// ListJobs returns all jobs for the agency, optionally filtered.
func (s *Server) ListJobs(ctx context.Context, req *pb.ListJobsRequest) (*pb.ListJobsResponse, error) {
	filter := codevaldelfunctions.JobFilter{}
	if f := req.GetFilter(); f != nil {
		filter.Status = codevaldelfunctions.JobStatus(f.GetStatus())
		filter.FunctionName = f.GetFunctionName()
	}
	jobs, err := s.mgr.ListJobs(ctx, req.GetAgencyId(), filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list jobs: %v", err)
	}
	pbJobs := make([]*pb.Job, len(jobs))
	for i, j := range jobs {
		pbJobs[i] = jobToProto(j)
	}
	return &pb.ListJobsResponse{Jobs: pbJobs}, nil
}

// GetJob returns a single job by ID.
func (s *Server) GetJob(ctx context.Context, req *pb.GetJobRequest) (*pb.GetJobResponse, error) {
	job, err := s.mgr.GetJob(ctx, req.GetAgencyId(), req.GetJobId())
	if err != nil {
		if errors.Is(err, codevaldelfunctions.ErrJobNotFound) {
			return nil, status.Errorf(codes.NotFound, "job %q not found", req.GetJobId())
		}
		return nil, status.Errorf(codes.Internal, "get job: %v", err)
	}
	return &pb.GetJobResponse{Job: jobToProto(job)}, nil
}

// CancelJob cancels a pending or running job.
func (s *Server) CancelJob(ctx context.Context, req *pb.CancelJobRequest) (*pb.CancelJobResponse, error) {
	job, err := s.mgr.CancelJob(ctx, req.GetAgencyId(), req.GetJobId())
	if err != nil {
		if errors.Is(err, codevaldelfunctions.ErrJobNotFound) {
			return nil, status.Errorf(codes.NotFound, "job %q not found", req.GetJobId())
		}
		if errors.Is(err, codevaldelfunctions.ErrInvalidJobTransition) {
			return nil, status.Errorf(codes.FailedPrecondition, "cancel job: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "cancel job: %v", err)
	}
	return &pb.CancelJobResponse{Job: jobToProto(job)}, nil
}

func jobToProto(j codevaldelfunctions.Job) *pb.Job {
	return &pb.Job{
		Id:             j.ID,
		AgencyId:       j.AgencyID,
		Status:         string(j.Status),
		FunctionName:   j.FunctionName,
		TriggerEvent:   j.TriggerEvent,
		TriggerPayload: j.TriggerPayload,
		Result:         j.Result,
		Error:          j.Error,
		RetryCount:     int32(j.RetryCount),
		CreatedAt:      formatTime(j.CreatedAt),
		StartedAt:      formatTime(j.StartedAt),
		CompletedAt:    formatTime(j.CompletedAt),
	}
}

func formatTime(t interface{ IsZero() bool }) string {
	// Use a type switch to handle time.Time without importing time in the type sig.
	type timer interface {
		IsZero() bool
		Format(string) string
	}
	if tt, ok := t.(timer); ok && !tt.IsZero() {
		return tt.Format("2006-01-02T15:04:05Z07:00")
	}
	return ""
}
