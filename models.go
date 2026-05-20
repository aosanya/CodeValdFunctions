package codevaldelfunctions

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
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusRetrying  JobStatus = "retrying"
)

// JobFilter narrows ListJobs results. Zero values are ignored.
type JobFilter struct {
	Status       JobStatus // filter by status; zero means all statuses
	FunctionName string    // filter by function name; empty means all functions
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
