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
	job, err := mgr.CreateJob(context.Background(), testAgencyID, "compile", "work.task.completed", `{}`, "")
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
