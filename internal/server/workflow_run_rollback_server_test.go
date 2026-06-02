package server

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	codevaldfunctions "github.com/aosanya/CodeValdFunctions"
	pb "github.com/aosanya/CodeValdFunctions/gen/go/codevaldfunctions/v1"
)

// fakeFunctionsManager is a minimal stand-in for codevaldfunctions.FunctionsManager
// — only the rollback paths are exercised; other interface methods are
// intentionally unused (returning zero values is enough for the compiler).
type fakeFunctionsManager struct {
	codevaldfunctions.FunctionsManager // embed to inherit unused methods
	rollbackResult                     codevaldfunctions.RollbackByWorkflowRunResult
	rollbackErr                        error
	rollbackCalls                      []rollbackCall
}

type rollbackCall struct {
	agencyID      string
	workflowRunID string
	reason        string
}

func (f *fakeFunctionsManager) RollbackByWorkflowRun(_ context.Context, agencyID, workflowRunID, reason string) (codevaldfunctions.RollbackByWorkflowRunResult, error) {
	f.rollbackCalls = append(f.rollbackCalls, rollbackCall{agencyID, workflowRunID, reason})
	if f.rollbackErr != nil {
		return codevaldfunctions.RollbackByWorkflowRunResult{}, f.rollbackErr
	}
	return f.rollbackResult, nil
}

func TestRollbackByWorkflowRun_RPC_RoundTrip(t *testing.T) {
	mgr := &fakeFunctionsManager{
		rollbackResult: codevaldfunctions.RollbackByWorkflowRunResult{
			WorkflowRunID:    "wf-1",
			CancelledJobIDs:  []string{"job-a"},
			RolledBackJobIDs: []string{"job-b", "job-c"},
			SkippedJobIDs:    []string{"job-d"},
		},
	}
	srv := New(mgr, nil)

	res, err := srv.RollbackByWorkflowRun(context.Background(), &pb.RollbackByWorkflowRunRequest{
		AgencyId:      "agency-x",
		WorkflowRunId: "wf-1",
		Reason:        "regression",
	})
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if res.WorkflowRunId != "wf-1" {
		t.Errorf("WorkflowRunId = %q want wf-1", res.WorkflowRunId)
	}
	if len(res.CancelledJobIds) != 1 || res.CancelledJobIds[0] != "job-a" {
		t.Errorf("CancelledJobIds = %v want [job-a]", res.CancelledJobIds)
	}
	if len(res.RolledBackJobIds) != 2 {
		t.Errorf("RolledBackJobIds len = %d want 2", len(res.RolledBackJobIds))
	}
	if len(res.SkippedJobIds) != 1 || res.SkippedJobIds[0] != "job-d" {
		t.Errorf("SkippedJobIds = %v want [job-d]", res.SkippedJobIds)
	}

	if len(mgr.rollbackCalls) != 1 {
		t.Fatalf("manager called %d times, want 1", len(mgr.rollbackCalls))
	}
	got := mgr.rollbackCalls[0]
	if got.agencyID != "agency-x" || got.workflowRunID != "wf-1" || got.reason != "regression" {
		t.Errorf("manager call args = %+v want {agency-x, wf-1, regression}", got)
	}
}

func TestRollbackByWorkflowRun_RPC_EmptyID_InvalidArgument(t *testing.T) {
	mgr := &fakeFunctionsManager{rollbackErr: codevaldfunctions.ErrWorkflowRunIDRequired}
	srv := New(mgr, nil)

	_, err := srv.RollbackByWorkflowRun(context.Background(), &pb.RollbackByWorkflowRunRequest{
		AgencyId: "agency-x",
	})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("err code = %v want InvalidArgument (err=%v)", got, err)
	}
}

func TestRollbackByWorkflowRun_RPC_ManagerError_Internal(t *testing.T) {
	mgr := &fakeFunctionsManager{rollbackErr: errors.New("storage went sideways")}
	srv := New(mgr, nil)

	_, err := srv.RollbackByWorkflowRun(context.Background(), &pb.RollbackByWorkflowRunRequest{
		AgencyId:      "agency-x",
		WorkflowRunId: "wf-x",
	})
	if got := status.Code(err); got != codes.Internal {
		t.Errorf("err code = %v want Internal (err=%v)", got, err)
	}
}
