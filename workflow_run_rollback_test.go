package codevaldfunctions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// seedJob inserts a Job directly into the fake DataManager with the given
// status + workflow_run_id, bypassing the manager's transition guards.
// Returns the newly-minted Job ID.
func seedJob(t *testing.T, dm *fakeDataManager, workflowRunID, functionName string, status JobStatus) string {
	t.Helper()
	entity, err := dm.CreateEntity(context.Background(), entitygraph.CreateEntityRequest{
		AgencyID: testAgencyID,
		TypeID:   jobTypeID,
		Properties: map[string]any{
			"status":          string(status),
			"function_name":   functionName,
			"trigger_event":   "task.completed",
			"trigger_payload": "{}",
			"task_id":         "",
			"workflow_run_id": workflowRunID,
			"result":          "",
			"error":           "",
			"retry_count":     0,
			"created_at":      "",
			"started_at":      "",
			"completed_at":    "",
		},
	})
	if err != nil {
		t.Fatalf("seedJob: %v", err)
	}
	return entity.ID
}

func newTestManagerWithPub() (FunctionsManager, *capturingPublisher, *fakeDataManager) {
	pub := &capturingPublisher{}
	dm := newFakeDataManager()
	return NewFunctionsManager(dm, pub, testAgencyID), pub, dm
}

// containsID is a tiny helper to keep result assertions readable.
func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestRollbackByWorkflowRun_EmptyID_ReturnsError(t *testing.T) {
	mgr := newTestManager()
	_, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, "", "")
	if !errors.Is(err, ErrWorkflowRunIDRequired) {
		t.Errorf("err = %v want ErrWorkflowRunIDRequired", err)
	}
}

func TestRollbackByWorkflowRun_NoMatchingJobs_EmptyResult(t *testing.T) {
	mgr, pub, _ := newTestManagerWithPub()
	result, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, "wf-empty", "reason")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if result.WorkflowRunID != "wf-empty" {
		t.Errorf("WorkflowRunID = %q want wf-empty", result.WorkflowRunID)
	}
	if len(result.CancelledJobIDs)+len(result.RolledBackJobIDs)+len(result.SkippedJobIDs) != 0 {
		t.Errorf("expected empty result, got %+v", result)
	}
	if len(pub.events) != 0 {
		t.Errorf("no events expected, got %d", len(pub.events))
	}
}

func TestRollbackByWorkflowRun_InFlightJobs_TransitionToCancelled(t *testing.T) {
	mgr, pub, dm := newTestManagerWithPub()
	const wfID = "wf-inflight"

	inflight := []JobStatus{JobStatusPending, JobStatusRunning, JobStatusRetrying}
	var jobIDs []string
	for _, s := range inflight {
		jobIDs = append(jobIDs, seedJob(t, dm, wfID, "compile", s))
	}

	result, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, "regression")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if len(result.CancelledJobIDs) != len(jobIDs) {
		t.Errorf("CancelledJobIDs = %v want %d entries", result.CancelledJobIDs, len(jobIDs))
	}
	if len(result.RolledBackJobIDs) != 0 {
		t.Errorf("RolledBackJobIDs should be empty, got %v", result.RolledBackJobIDs)
	}

	for _, id := range jobIDs {
		job, err := mgr.GetJob(context.Background(), testAgencyID, id)
		if err != nil {
			t.Fatalf("GetJob %s: %v", id, err)
		}
		if job.Status != JobStatusCancelled {
			t.Errorf("job %s status = %q want cancelled", id, job.Status)
		}
		if job.CompletedAt.IsZero() {
			t.Errorf("job %s: want completed_at set after cancel", id)
		}
	}

	cancelledCount := 0
	for _, e := range pub.events {
		if e.Topic == TopicJobCancelled {
			cancelledCount++
		}
	}
	if cancelledCount != len(jobIDs) {
		t.Errorf("functions.job.cancelled count = %d want %d", cancelledCount, len(jobIDs))
	}
}

func TestRollbackByWorkflowRun_TerminalJobs_TransitionToRolledBack(t *testing.T) {
	mgr, pub, dm := newTestManagerWithPub()
	const wfID = "wf-terminal"

	completedID := seedJob(t, dm, wfID, "compile", JobStatusCompleted)
	failedID := seedJob(t, dm, wfID, "merge", JobStatusFailed)

	// Pre-stamp completed_at on completedID to verify it is preserved (the
	// rollback must not overwrite the original terminal timestamp).
	const preExistingCompletedAt = "2026-06-01T00:00:00Z"
	if _, err := dm.UpdateEntity(context.Background(), testAgencyID, completedID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"completed_at": preExistingCompletedAt},
	}); err != nil {
		t.Fatalf("seed completed_at: %v", err)
	}

	result, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, "")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if len(result.RolledBackJobIDs) != 2 {
		t.Errorf("RolledBackJobIDs = %v want 2 entries", result.RolledBackJobIDs)
	}
	if len(result.CancelledJobIDs) != 0 {
		t.Errorf("CancelledJobIDs should be empty, got %v", result.CancelledJobIDs)
	}

	for _, id := range []string{completedID, failedID} {
		job, err := mgr.GetJob(context.Background(), testAgencyID, id)
		if err != nil {
			t.Fatalf("GetJob %s: %v", id, err)
		}
		if job.Status != JobStatusRolledBack {
			t.Errorf("job %s status = %q want rolled_back", id, job.Status)
		}
	}

	// completed_at on the pre-stamped job must NOT have been overwritten.
	completedEntity, _ := dm.GetEntity(context.Background(), testAgencyID, completedID)
	if got, _ := completedEntity.Properties["completed_at"].(string); got != preExistingCompletedAt {
		t.Errorf("completed_at on rolled-back job overwritten: got %q want %q", got, preExistingCompletedAt)
	}

	rolledBackCount := 0
	for _, e := range pub.events {
		if e.Topic == TopicJobRolledBack {
			rolledBackCount++
		}
	}
	if rolledBackCount != 2 {
		t.Errorf("functions.job.rolled_back count = %d want 2", rolledBackCount)
	}
}

func TestRollbackByWorkflowRun_AlreadyRolledBack_Skipped(t *testing.T) {
	mgr, pub, dm := newTestManagerWithPub()
	const wfID = "wf-skip"

	cancelledID := seedJob(t, dm, wfID, "compile", JobStatusCancelled)
	rolledID := seedJob(t, dm, wfID, "merge", JobStatusRolledBack)

	result, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, "")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if len(result.SkippedJobIDs) != 2 {
		t.Errorf("SkippedJobIDs = %v want 2 entries", result.SkippedJobIDs)
	}
	if !containsID(result.SkippedJobIDs, cancelledID) || !containsID(result.SkippedJobIDs, rolledID) {
		t.Errorf("SkippedJobIDs %v missing one of %s,%s", result.SkippedJobIDs, cancelledID, rolledID)
	}
	if len(pub.events) != 0 {
		t.Errorf("no events expected for skipped jobs, got %d", len(pub.events))
	}
}

func TestRollbackByWorkflowRun_MixedClosure_PartitionsCorrectly(t *testing.T) {
	mgr, _, dm := newTestManagerWithPub()
	const wfID = "wf-mixed"

	runningID := seedJob(t, dm, wfID, "compile", JobStatusRunning)
	completedID := seedJob(t, dm, wfID, "compile", JobStatusCompleted)
	cancelledID := seedJob(t, dm, wfID, "merge", JobStatusCancelled)
	otherID := seedJob(t, dm, "wf-other", "compile", JobStatusRunning)

	result, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, "mix")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if len(result.CancelledJobIDs) != 1 || result.CancelledJobIDs[0] != runningID {
		t.Errorf("CancelledJobIDs = %v want [%s]", result.CancelledJobIDs, runningID)
	}
	if len(result.RolledBackJobIDs) != 1 || result.RolledBackJobIDs[0] != completedID {
		t.Errorf("RolledBackJobIDs = %v want [%s]", result.RolledBackJobIDs, completedID)
	}
	if len(result.SkippedJobIDs) != 1 || result.SkippedJobIDs[0] != cancelledID {
		t.Errorf("SkippedJobIDs = %v want [%s]", result.SkippedJobIDs, cancelledID)
	}

	// Job anchored to the other workflow must remain untouched.
	other, err := mgr.GetJob(context.Background(), testAgencyID, otherID)
	if err != nil {
		t.Fatalf("GetJob other: %v", err)
	}
	if other.Status != JobStatusRunning {
		t.Errorf("other job status = %q want still running", other.Status)
	}
}

func TestRollbackByWorkflowRun_Idempotent(t *testing.T) {
	mgr, _, dm := newTestManagerWithPub()
	const wfID = "wf-idem"
	jobID := seedJob(t, dm, wfID, "compile", JobStatusRunning)

	first, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, "")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(first.CancelledJobIDs) != 1 {
		t.Fatalf("first.CancelledJobIDs = %v want 1", first.CancelledJobIDs)
	}

	second, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, "")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(second.CancelledJobIDs) != 0 {
		t.Errorf("second.CancelledJobIDs = %v want empty (idempotent)", second.CancelledJobIDs)
	}
	if len(second.SkippedJobIDs) != 1 || second.SkippedJobIDs[0] != jobID {
		t.Errorf("second.SkippedJobIDs = %v want [%s]", second.SkippedJobIDs, jobID)
	}
}

func TestRollbackByWorkflowRun_RecordsReasonOnEntity(t *testing.T) {
	mgr, _, dm := newTestManagerWithPub()
	const wfID = "wf-reason"
	jobID := seedJob(t, dm, wfID, "compile", JobStatusRunning)

	if _, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, "manual intervention"); err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}

	entity, _ := dm.GetEntity(context.Background(), testAgencyID, jobID)
	got, _ := entity.Properties["rollback_reason"].(string)
	if !strings.Contains(got, "manual intervention") {
		t.Errorf("rollback_reason = %q want substring 'manual intervention'", got)
	}
}

func TestRollbackByWorkflowRun_CancelledPayloadCarriesPreviousStatusAndReason(t *testing.T) {
	mgr, pub, dm := newTestManagerWithPub()
	const wfID = "wf-payload"
	jobID := seedJob(t, dm, wfID, "compile-flutter", JobStatusRunning)

	if _, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, "operator-cancel"); err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}

	var payload JobCancelledPayload
	found := false
	for _, e := range pub.events {
		if e.Topic != TopicJobCancelled {
			continue
		}
		raw, ok := e.Payload.(string)
		if !ok {
			t.Fatalf("payload type %T want string", e.Payload)
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		found = true
		break
	}
	if !found {
		t.Fatal("no functions.job.cancelled event seen")
	}
	if payload.JobID != jobID {
		t.Errorf("payload.JobID = %q want %q", payload.JobID, jobID)
	}
	if payload.WorkflowRunID != wfID {
		t.Errorf("payload.WorkflowRunID = %q want %q", payload.WorkflowRunID, wfID)
	}
	if payload.PreviousStatus != JobStatusRunning {
		t.Errorf("payload.PreviousStatus = %q want running", payload.PreviousStatus)
	}
	if payload.Reason != "operator-cancel" {
		t.Errorf("payload.Reason = %q want operator-cancel", payload.Reason)
	}
	if payload.FunctionName != "compile-flutter" {
		t.Errorf("payload.FunctionName = %q want compile-flutter", payload.FunctionName)
	}
}

func TestRollbackByWorkflowRun_RolledBackPayloadDistinguishesCompletedVsFailed(t *testing.T) {
	mgr, pub, dm := newTestManagerWithPub()
	const wfID = "wf-distinguish"

	seedJob(t, dm, wfID, "compile", JobStatusCompleted)
	seedJob(t, dm, wfID, "merge", JobStatusFailed)

	if _, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, "audit"); err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}

	seen := map[JobStatus]bool{}
	for _, e := range pub.events {
		if e.Topic != TopicJobRolledBack {
			continue
		}
		raw, _ := e.Payload.(string)
		var p JobRolledBackPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		seen[p.PreviousStatus] = true
	}
	if !seen[JobStatusCompleted] || !seen[JobStatusFailed] {
		t.Errorf("rolled_back payloads missing one previous status: seen=%v", seen)
	}
}

func TestRollbackByWorkflowRun_PreservesFunctionOutputs(t *testing.T) {
	// FEAT spec: function outputs (compile logs, results) must be preserved
	// when freezing a terminal Job as rolled_back.
	mgr, _, dm := newTestManagerWithPub()
	const wfID = "wf-preserve"
	jobID := seedJob(t, dm, wfID, "compile", JobStatusFailed)

	const preservedError = "syntax error at line 42"
	if _, err := dm.UpdateEntity(context.Background(), testAgencyID, jobID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"error":       preservedError,
			"result":      "",
			"retry_count": 3,
		},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := mgr.RollbackByWorkflowRun(context.Background(), testAgencyID, wfID, ""); err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}

	job, err := mgr.GetJob(context.Background(), testAgencyID, jobID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.Error != preservedError {
		t.Errorf("error = %q want %q (must be preserved for debugging)", job.Error, preservedError)
	}
	if job.RetryCount != 3 {
		t.Errorf("retry_count = %d want 3 (must be preserved)", job.RetryCount)
	}
}
