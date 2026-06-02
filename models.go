package codevaldfunctions

import (
	"context"
	"time"

	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// CrossPublisher is implemented by the registrar; it lets domain logic publish
// events to CodeValdCross without importing the registrar package directly.
type CrossPublisher interface {
	Publish(ctx context.Context, e eventbus.Event) error
}

// JobStatus is the lifecycle state of a Job.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusRunning    JobStatus = "running"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
	JobStatusRetrying   JobStatus = "retrying"
	JobStatusRolledBack JobStatus = "rolled_back"
)

// JobFilter narrows ListJobs results. Zero values are ignored.
type JobFilter struct {
	Status        JobStatus // filter by status; zero means all statuses
	FunctionName  string    // filter by function name; empty means all functions
	WorkflowRunID string    // filter by originating WorkflowRun ID; empty means all runs
}

// Job represents a single function execution triggered by a platform event.
type Job struct {
	// ID is the entitygraph-assigned identifier.
	ID string

	// AgencyID is the agency this job belongs to.
	AgencyID string

	// Status is the current lifecycle state.
	Status JobStatus

	// FunctionName is the name of the pre-built handler that ran (or is running).
	FunctionName string

	// TriggerEvent is the platform event name that created this job
	// (e.g. "work.task.completed").
	TriggerEvent string

	// TriggerPayload is the JSON-encoded event payload received from Cross.
	TriggerPayload string

	// TaskID is the platform task ID extracted from the trigger payload,
	// used to correlate this Job with a CodeValdWork task.
	TaskID string

	// WorkflowRunID is the originating WorkflowRun ID, copied from the inbound
	// trigger event under the platform-wide chain-through rule. Empty for jobs
	// triggered by non-pipeline events (orphan v1 policy).
	WorkflowRunID string

	// Result is the JSON-encoded function output on successful completion.
	Result string

	// Error is the error message when the job reaches a failed state.
	Error string

	// RetryCount is the number of execution attempts made so far.
	RetryCount int

	// CreatedAt is when the job entity was created.
	CreatedAt time.Time

	// StartedAt is when execution began. Zero value means not yet started.
	StartedAt time.Time

	// CompletedAt is when the job reached a terminal state. Zero value means
	// the job has not yet terminated.
	CompletedAt time.Time
}

// RollbackByWorkflowRunResult summarises the per-Job outcomes of
// [FunctionsManager.RollbackByWorkflowRun]. Every Job anchored to the
// requested WorkflowRun appears in exactly one of the three slices.
type RollbackByWorkflowRunResult struct {
	// WorkflowRunID echoes the requested anchor.
	WorkflowRunID string `json:"workflow_run_id"`

	// CancelledJobIDs are Jobs that were in-flight (pending, running, or
	// retrying) at rollback time and were transitioned to
	// [JobStatusCancelled].
	CancelledJobIDs []string `json:"cancelled_job_ids,omitempty"`

	// RolledBackJobIDs are Jobs that had already reached completed or
	// failed and were transitioned to [JobStatusRolledBack] as a frozen
	// audit record. Function outputs (result, error, compile logs) are
	// preserved for debugging.
	RolledBackJobIDs []string `json:"rolled_back_job_ids,omitempty"`

	// SkippedJobIDs are Jobs that were already in a rollback-terminal
	// state (cancelled or rolled_back) — the call is idempotent for these.
	SkippedJobIDs []string `json:"skipped_job_ids,omitempty"`
}
