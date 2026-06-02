package codevaldfunctions

import "context"

// FunctionsManager is the top-level interface for the CodeValdFunctions domain.
// It manages Job entities and their lifecycle transitions.
type FunctionsManager interface {
	// CreateJob creates a new Job in the pending state. workflowRunID is the
	// originating WorkflowRun ID extracted from the trigger payload (empty for
	// non-pipeline triggers — see chain-through rule).
	CreateJob(ctx context.Context, agencyID, functionName, triggerEvent, payload, taskID, workflowRunID string) (Job, error)

	// GetJob retrieves a single Job by ID.
	GetJob(ctx context.Context, agencyID, jobID string) (Job, error)

	// ListJobs returns all Jobs for an agency, optionally filtered.
	ListJobs(ctx context.Context, agencyID string, filter JobFilter) ([]Job, error)

	// StartJob transitions a Job from pending or retrying → running,
	// recording started_at.
	StartJob(ctx context.Context, agencyID, jobID string) (Job, error)

	// CompleteJob transitions a running Job → completed, recording the result
	// and completed_at.
	CompleteJob(ctx context.Context, agencyID, jobID, result string) (Job, error)

	// FailJob transitions a running Job → failed or retrying depending on the
	// current retry_count vs the per-manager max retries.
	FailJob(ctx context.Context, agencyID, jobID, errMsg string) (Job, error)

	// CancelJob transitions a pending or running Job → cancelled.
	// Returns ErrInvalidJobTransition for jobs in any other state.
	CancelJob(ctx context.Context, agencyID, jobID string) (Job, error)

	// SetJobWorkflowRunID back-fills the WorkflowRun ID on a Job. Used by the
	// dispatcher for functions that mint the run themselves (`start-pipeline`):
	// their own Job row is created before the run exists, so the run ID is
	// stamped on the Job after the function returns it in its result.
	// Returns ErrJobNotFound if the job does not exist.
	SetJobWorkflowRunID(ctx context.Context, agencyID, jobID, workflowRunID string) (Job, error)
}
