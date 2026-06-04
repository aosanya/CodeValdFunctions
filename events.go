package codevaldfunctions

import "github.com/aosanya/CodeValdSharedLib/eventbus"

// Functions event topics — CodeValdFunctions publishes only functions.* events.
// No agencyID segment: each service instance is scoped to a single agency.
//
// The first batch of topics (created/started/completed/failed) is published
// inline as string literals from manager.go for historical reasons; only the
// rollback topics — which carry typed payloads — currently live as constants.
const (
	// TopicJobCancelled is published once per in-flight Job the rollback
	// coordinator transitions to cancelled. Recipients should treat the
	// Job as terminal.
	TopicJobCancelled = eventbus.DomainFunctions + "job.cancelled"

	// TopicJobRolledBack is published once per already-terminal Job the
	// rollback coordinator freezes as audit. The payload includes the
	// original terminal status so consumers can distinguish
	// completed-then-rolled-back from failed-then-rolled-back.
	TopicJobRolledBack = eventbus.DomainFunctions + "job.rolled_back"
)

// JobCancelledPayload is published on [TopicJobCancelled] once per in-flight
// Job the rollback coordinator transitions to cancelled.
type JobCancelledPayload struct {
	JobID          string    `json:"job_id"`
	FunctionName   string    `json:"function_name,omitempty"`
	WorkflowRunID  string    `json:"workflow_run_id"`
	PreviousStatus JobStatus `json:"previous_status"`
	Reason         string    `json:"reason,omitempty"`
}

// JobRolledBackPayload is published on [TopicJobRolledBack] once per
// completed-or-failed Job the rollback coordinator freezes as audit.
type JobRolledBackPayload struct {
	JobID          string    `json:"job_id"`
	FunctionName   string    `json:"function_name,omitempty"`
	WorkflowRunID  string    `json:"workflow_run_id"`
	PreviousStatus JobStatus `json:"previous_status"`
	Reason         string    `json:"reason,omitempty"`
}
