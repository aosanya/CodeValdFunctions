// Package server implements the FunctionsService gRPC handlers.
package server

import (
	"context"
	"errors"

	codevaldfunctions "github.com/aosanya/CodeValdFunctions"
	pb "github.com/aosanya/CodeValdFunctions/gen/go/codevaldfunctions/v1"
	"github.com/aosanya/CodeValdFunctions/internal/functions"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements pb.FunctionsServiceServer.
type Server struct {
	pb.UnimplementedFunctionsServiceServer
	mgr    codevaldfunctions.FunctionsManager
	runner *functions.BinaryRunner
}

// New constructs a Server backed by the given FunctionsManager and BinaryRunner.
func New(mgr codevaldfunctions.FunctionsManager, runner *functions.BinaryRunner) *Server {
	return &Server{mgr: mgr, runner: runner}
}

// ListJobs returns all jobs for the agency, optionally filtered.
func (s *Server) ListJobs(ctx context.Context, req *pb.ListJobsRequest) (*pb.ListJobsResponse, error) {
	filter := codevaldfunctions.JobFilter{}
	if f := req.GetFilter(); f != nil {
		filter.Status = codevaldfunctions.JobStatus(f.GetStatus())
		filter.FunctionName = f.GetFunctionName()
		filter.WorkflowRunID = f.GetWorkflowRunId()
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
		if errors.Is(err, codevaldfunctions.ErrJobNotFound) {
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
		if errors.Is(err, codevaldfunctions.ErrJobNotFound) {
			return nil, status.Errorf(codes.NotFound, "job %q not found", req.GetJobId())
		}
		if errors.Is(err, codevaldfunctions.ErrInvalidJobTransition) {
			return nil, status.Errorf(codes.FailedPrecondition, "cancel job: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "cancel job: %v", err)
	}
	return &pb.CancelJobResponse{Job: jobToProto(job)}, nil
}

// RollbackByWorkflowRun applies the CodeValdFunctions leg of the WorkflowRun
// rollback contract (FEAT-20260602-004 Phase 2): in-flight Jobs go to
// cancelled; already-terminal Jobs go to rolled_back as a frozen audit record.
func (s *Server) RollbackByWorkflowRun(ctx context.Context, req *pb.RollbackByWorkflowRunRequest) (*pb.RollbackByWorkflowRunResponse, error) {
	result, err := s.mgr.RollbackByWorkflowRun(ctx, req.GetAgencyId(), req.GetWorkflowRunId(), req.GetReason())
	if err != nil {
		if errors.Is(err, codevaldfunctions.ErrWorkflowRunIDRequired) {
			return nil, status.Errorf(codes.InvalidArgument, "rollback by workflow run: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "rollback by workflow run: %v", err)
	}
	return &pb.RollbackByWorkflowRunResponse{
		WorkflowRunId:    result.WorkflowRunID,
		CancelledJobIds:  result.CancelledJobIDs,
		RolledBackJobIds: result.RolledBackJobIDs,
		SkippedJobIds:    result.SkippedJobIDs,
	}, nil
}

// DeployFunction writes a function binary and manifest to the functions directory.
func (s *Server) DeployFunction(ctx context.Context, req *pb.DeployFunctionRequest) (*pb.DeployFunctionResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetTrigger() == "" {
		return nil, status.Error(codes.InvalidArgument, "trigger is required")
	}
	if len(req.GetBinary()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "binary is required")
	}
	if err := s.runner.Deploy(req.GetName(), req.GetTrigger(), req.GetDescription(), req.GetBinary()); err != nil {
		return nil, status.Errorf(codes.Internal, "deploy function: %v", err)
	}
	path := s.runner.BinaryPath(req.GetName())
	return &pb.DeployFunctionResponse{Name: req.GetName(), Path: path}, nil
}

func jobToProto(j codevaldfunctions.Job) *pb.Job {
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
		TaskId:         j.TaskID,
		WorkflowRunId:  j.WorkflowRunID,
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
