package codevaldfunctions

import (
	"context"
	"errors"
	"testing"
)

const testAgencyID = "agency-test"

func newTestManager() FunctionsManager {
	return NewFunctionsManager(newFakeDataManager(), nil, testAgencyID)
}

// helpers

func mustCreateJob(t *testing.T, mgr FunctionsManager) Job {
	t.Helper()
	job, err := mgr.CreateJob(context.Background(), testAgencyID, "compile", "task.completed", `{}`, "", "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	return job
}

// mustCreateJobWithRun is mustCreateJob plus an explicit workflow_run_id.
func mustCreateJobWithRun(t *testing.T, mgr FunctionsManager, workflowRunID string) Job {
	t.Helper()
	job, err := mgr.CreateJob(context.Background(), testAgencyID, "compile", "task.completed", `{}`, "", workflowRunID)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	return job
}

// ── CreateJob ────────────────────────────────────────────────────────────────

func TestCreateJob_PendingStatus(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)
	if job.Status != JobStatusPending {
		t.Errorf("want status %q, got %q", JobStatusPending, job.Status)
	}
	if job.ID == "" {
		t.Error("want non-empty ID")
	}
	if job.FunctionName != "compile" {
		t.Errorf("want function_name %q, got %q", "compile", job.FunctionName)
	}
}

// ── StartJob ─────────────────────────────────────────────────────────────────

func TestStartJob_PendingToRunning(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)

	started, err := mgr.StartJob(context.Background(), testAgencyID, job.ID)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	if started.Status != JobStatusRunning {
		t.Errorf("want status %q, got %q", JobStatusRunning, started.Status)
	}
	if started.StartedAt.IsZero() {
		t.Error("want started_at set")
	}
}

func TestStartJob_InvalidFromCompleted(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)
	job, _ = mgr.StartJob(context.Background(), testAgencyID, job.ID)
	_, _ = mgr.CompleteJob(context.Background(), testAgencyID, job.ID, "ok")

	_, err := mgr.StartJob(context.Background(), testAgencyID, job.ID)
	if !errors.Is(err, ErrInvalidJobTransition) {
		t.Errorf("want ErrInvalidJobTransition, got %v", err)
	}
}

func TestStartJob_InvalidFromFailed(t *testing.T) {
	mgr := newTestManager()
	ctx := context.Background()
	job := mustCreateJob(t, mgr)
	// exhaust retries
	for i := 0; i < maxRetries; i++ {
		job, _ = mgr.StartJob(ctx, testAgencyID, job.ID)
		job, _ = mgr.FailJob(ctx, testAgencyID, job.ID, "err")
		if job.Status == JobStatusFailed {
			break
		}
	}

	_, err := mgr.StartJob(ctx, testAgencyID, job.ID)
	if !errors.Is(err, ErrInvalidJobTransition) {
		t.Errorf("want ErrInvalidJobTransition, got %v", err)
	}
}

// ── CompleteJob ───────────────────────────────────────────────────────────────

func TestCompleteJob_RunningToCompleted(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)
	job, _ = mgr.StartJob(context.Background(), testAgencyID, job.ID)

	done, err := mgr.CompleteJob(context.Background(), testAgencyID, job.ID, `{"out":"ok"}`)
	if err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}
	if done.Status != JobStatusCompleted {
		t.Errorf("want status %q, got %q", JobStatusCompleted, done.Status)
	}
	if done.Result != `{"out":"ok"}` {
		t.Errorf("unexpected result %q", done.Result)
	}
	if done.CompletedAt.IsZero() {
		t.Error("want completed_at set")
	}
}

func TestCompleteJob_InvalidFromPending(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)

	_, err := mgr.CompleteJob(context.Background(), testAgencyID, job.ID, "")
	if !errors.Is(err, ErrInvalidJobTransition) {
		t.Errorf("want ErrInvalidJobTransition, got %v", err)
	}
}

// ── FailJob ───────────────────────────────────────────────────────────────────

func TestFailJob_RetryingWhenUnderMaxRetries(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)
	job, _ = mgr.StartJob(context.Background(), testAgencyID, job.ID)

	failed, err := mgr.FailJob(context.Background(), testAgencyID, job.ID, "timeout")
	if err != nil {
		t.Fatalf("FailJob: %v", err)
	}
	if failed.Status != JobStatusRetrying {
		t.Errorf("want status %q, got %q", JobStatusRetrying, failed.Status)
	}
	if failed.RetryCount != 1 {
		t.Errorf("want retry_count 1, got %d", failed.RetryCount)
	}
}

func TestFailJob_FailedWhenMaxRetriesExhausted(t *testing.T) {
	mgr := newTestManager()
	ctx := context.Background()
	job := mustCreateJob(t, mgr)

	for i := 0; i < maxRetries; i++ {
		job, _ = mgr.StartJob(ctx, testAgencyID, job.ID)
		job, _ = mgr.FailJob(ctx, testAgencyID, job.ID, "err")
	}

	if job.Status != JobStatusFailed {
		t.Errorf("want status %q after %d retries, got %q", JobStatusFailed, maxRetries, job.Status)
	}
	if job.CompletedAt.IsZero() {
		t.Error("want completed_at set on terminal failure")
	}
}

func TestFailJob_InvalidFromPending(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)

	_, err := mgr.FailJob(context.Background(), testAgencyID, job.ID, "err")
	if !errors.Is(err, ErrInvalidJobTransition) {
		t.Errorf("want ErrInvalidJobTransition, got %v", err)
	}
}

// ── CancelJob ─────────────────────────────────────────────────────────────────

func TestCancelJob_FromPending(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)

	cancelled, err := mgr.CancelJob(context.Background(), testAgencyID, job.ID)
	if err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	if cancelled.Status != JobStatusCancelled {
		t.Errorf("want status %q, got %q", JobStatusCancelled, cancelled.Status)
	}
}

func TestCancelJob_FromRunning(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)
	job, _ = mgr.StartJob(context.Background(), testAgencyID, job.ID)

	cancelled, err := mgr.CancelJob(context.Background(), testAgencyID, job.ID)
	if err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	if cancelled.Status != JobStatusCancelled {
		t.Errorf("want status %q, got %q", JobStatusCancelled, cancelled.Status)
	}
}

func TestCancelJob_InvalidFromCompleted(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)
	job, _ = mgr.StartJob(context.Background(), testAgencyID, job.ID)
	_, _ = mgr.CompleteJob(context.Background(), testAgencyID, job.ID, "ok")

	_, err := mgr.CancelJob(context.Background(), testAgencyID, job.ID)
	if !errors.Is(err, ErrInvalidJobTransition) {
		t.Errorf("want ErrInvalidJobTransition, got %v", err)
	}
}

// ── ListJobs ──────────────────────────────────────────────────────────────────

func TestListJobs_Empty(t *testing.T) {
	mgr := newTestManager()
	jobs, err := mgr.ListJobs(context.Background(), testAgencyID, JobFilter{})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("want 0 jobs, got %d", len(jobs))
	}
}

func TestListJobs_ReturnsAll(t *testing.T) {
	mgr := newTestManager()
	mustCreateJob(t, mgr)
	mustCreateJob(t, mgr)

	jobs, err := mgr.ListJobs(context.Background(), testAgencyID, JobFilter{})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("want 2 jobs, got %d", len(jobs))
	}
}

func TestListJobs_FilterByStatus(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJob(t, mgr)
	_, _ = mgr.StartJob(context.Background(), testAgencyID, job.ID)
	mustCreateJob(t, mgr) // stays pending

	running, err := mgr.ListJobs(context.Background(), testAgencyID, JobFilter{Status: JobStatusRunning})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(running) != 1 {
		t.Errorf("want 1 running job, got %d", len(running))
	}
}

// ── GetJob ────────────────────────────────────────────────────────────────────

func TestGetJob_NotFound(t *testing.T) {
	mgr := newTestManager()

	_, err := mgr.GetJob(context.Background(), testAgencyID, "nonexistent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("want ErrJobNotFound, got %v", err)
	}
}

// ── workflow_run_id propagation (FEAT-20260602-002) ──────────────────────────

func TestCreateJob_PersistsWorkflowRunID(t *testing.T) {
	mgr := newTestManager()
	job := mustCreateJobWithRun(t, mgr, "wfr_abc123")
	if job.WorkflowRunID != "wfr_abc123" {
		t.Errorf("want workflow_run_id %q on returned Job, got %q", "wfr_abc123", job.WorkflowRunID)
	}
	got, err := mgr.GetJob(context.Background(), testAgencyID, job.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.WorkflowRunID != "wfr_abc123" {
		t.Errorf("want workflow_run_id %q after GetJob, got %q", "wfr_abc123", got.WorkflowRunID)
	}
}

func TestCreateJob_EmptyWorkflowRunIDAllowed(t *testing.T) {
	// Orphan policy: a non-pipeline trigger may legitimately have no run ID.
	mgr := newTestManager()
	job := mustCreateJobWithRun(t, mgr, "")
	if job.WorkflowRunID != "" {
		t.Errorf("want empty workflow_run_id, got %q", job.WorkflowRunID)
	}
}

func TestListJobs_FilterByWorkflowRunID(t *testing.T) {
	mgr := newTestManager()
	mustCreateJobWithRun(t, mgr, "wfr_run1")
	mustCreateJobWithRun(t, mgr, "wfr_run1")
	mustCreateJobWithRun(t, mgr, "wfr_run2")
	mustCreateJobWithRun(t, mgr, "")

	run1, err := mgr.ListJobs(context.Background(), testAgencyID, JobFilter{WorkflowRunID: "wfr_run1"})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(run1) != 2 {
		t.Errorf("want 2 jobs for wfr_run1, got %d", len(run1))
	}
	for _, j := range run1 {
		if j.WorkflowRunID != "wfr_run1" {
			t.Errorf("filter leaked: job %s has workflow_run_id %q", j.ID, j.WorkflowRunID)
		}
	}

	run2, err := mgr.ListJobs(context.Background(), testAgencyID, JobFilter{WorkflowRunID: "wfr_run2"})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(run2) != 1 {
		t.Errorf("want 1 job for wfr_run2, got %d", len(run2))
	}
}

func TestSetJobWorkflowRunID_Backfills(t *testing.T) {
	// Simulates the start-pipeline case: Job created with empty run ID, then
	// stamped after the function returns its minted run.
	mgr := newTestManager()
	job := mustCreateJobWithRun(t, mgr, "")
	if job.WorkflowRunID != "" {
		t.Fatalf("precondition: want empty workflow_run_id, got %q", job.WorkflowRunID)
	}

	updated, err := mgr.SetJobWorkflowRunID(context.Background(), testAgencyID, job.ID, "wfr_minted")
	if err != nil {
		t.Fatalf("SetJobWorkflowRunID: %v", err)
	}
	if updated.WorkflowRunID != "wfr_minted" {
		t.Errorf("want %q on returned Job, got %q", "wfr_minted", updated.WorkflowRunID)
	}
	got, _ := mgr.GetJob(context.Background(), testAgencyID, job.ID)
	if got.WorkflowRunID != "wfr_minted" {
		t.Errorf("want %q persisted, got %q", "wfr_minted", got.WorkflowRunID)
	}
}

func TestSetJobWorkflowRunID_NotFound(t *testing.T) {
	mgr := newTestManager()
	_, err := mgr.SetJobWorkflowRunID(context.Background(), testAgencyID, "nonexistent", "wfr_x")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("want ErrJobNotFound, got %v", err)
	}
}

func TestPublish_IncludesWorkflowRunID(t *testing.T) {
	pub := &capturingPublisher{}
	dm := newFakeDataManager()
	mgr := NewFunctionsManager(dm, pub, testAgencyID)

	if _, err := mgr.CreateJob(context.Background(), testAgencyID, "compile", "todo.completed", `{}`, "", "wfr_pub_test"); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if len(pub.events) == 0 {
		t.Fatal("want at least one published event")
	}
	for _, e := range pub.events {
		if e.Topic != "functions.job.created" {
			continue
		}
		payload, ok := e.Payload.(map[string]string)
		if !ok {
			t.Fatalf("publish: payload type %T, want map[string]string", e.Payload)
		}
		if got := payload["workflow_run_id"]; got != "wfr_pub_test" {
			t.Errorf("functions.job.created: want workflow_run_id=%q in payload, got %q", "wfr_pub_test", got)
		}
		return
	}
	t.Errorf("no functions.job.created event seen; got: %v", pub.events)
}

func TestRetrying_CanStartAgain(t *testing.T) {
	mgr := newTestManager()
	ctx := context.Background()
	job := mustCreateJob(t, mgr)
	job, _ = mgr.StartJob(ctx, testAgencyID, job.ID)
	job, _ = mgr.FailJob(ctx, testAgencyID, job.ID, "transient")

	if job.Status != JobStatusRetrying {
		t.Fatalf("want retrying, got %q", job.Status)
	}

	// retrying → running is valid
	restarted, err := mgr.StartJob(ctx, testAgencyID, job.ID)
	if err != nil {
		t.Fatalf("StartJob from retrying: %v", err)
	}
	if restarted.Status != JobStatusRunning {
		t.Errorf("want running, got %q", restarted.Status)
	}
}
