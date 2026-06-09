// Package codevaldfunctions — pre-delivered schema definition.
//
// This file exposes [DefaultFunctionsSchema], which returns the fixed
// [types.Schema] for CodeValdFunctions. internal/app seeds this schema
// idempotently on startup via entitygraph.SeedSchema.
//
// The schema declares one TypeDefinition:
//   - Job — tracks a single function execution triggered by a platform event
//
// Graph topology: none in MVP (edges added if inter-job dependencies are needed)
//
// Storage:
//   - Job → "functions_entities"   document collection
//   - All edges → "functions_relationships" edge collection
package codevaldfunctions

import (
	"github.com/aosanya/CodeValdSharedLib/eventreceiver"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// DefaultFunctionsSchema returns the pre-delivered [types.Schema] seeded on startup.
// The operation is idempotent — calling it multiple times with the same schema
// ID is safe.
func DefaultFunctionsSchema() types.Schema {
	return types.Schema{
		ID:      "functions-schema-v1",
		Version: 1,
		Tag:     "v1",
		Types: []types.TypeDefinition{
			{
				Name:              "Job",
				DisplayName:       "Job",
				PathSegment:       "jobs",
				EntityIDParam:     "jobId",
				StorageCollection: "functions_entities",
				PublishEvents:     true,
				Properties: []types.PropertyDefinition{
					// status is the current lifecycle state — see job state machine.
					// Well-known values: "pending", "running", "completed", "failed",
					// "cancelled", "retrying", "rolled_back".
					{Name: "status", Type: types.PropertyTypeString, Required: true},
					// function_name is the name of the pre-built handler that ran (or is running).
					{Name: "function_name", Type: types.PropertyTypeString, Required: true},
					// trigger_event is the full platform event name that created this job
					// (e.g. "task.completed").
					{Name: "trigger_event", Type: types.PropertyTypeString, Required: true},
					// trigger_payload is the JSON-encoded event payload received from Cross.
					{Name: "trigger_payload", Type: types.PropertyTypeString},
					// task_id is the platform task ID extracted from the trigger payload,
					// used to correlate Jobs with CodeValdWork tasks.
					{Name: "task_id", Type: types.PropertyTypeString},
					// workflow_run_id is the originating WorkflowRun ID, propagated from
					// the inbound trigger event under the platform-wide chain-through
					// rule (see CodeValdCross architecture-notes/workflow-run-propagation.md).
					// Empty for jobs triggered by non-pipeline events (orphan v1 policy).
					{Name: "workflow_run_id", Type: types.PropertyTypeString},
					// result is the JSON-encoded function output on successful completion.
					{Name: "result", Type: types.PropertyTypeString},
					// error is the error message when the job reaches a failed state.
					{Name: "error", Type: types.PropertyTypeString},
					// retry_count is the number of execution attempts made so far.
					{Name: "retry_count", Type: types.PropertyTypeInteger},
					// created_at is the RFC 3339 timestamp when the job entity was created.
					{Name: "created_at", Type: types.PropertyTypeString},
					// started_at is the RFC 3339 timestamp when execution began.
					{Name: "started_at", Type: types.PropertyTypeString},
					// completed_at is the RFC 3339 timestamp when the job reached a terminal state.
					{Name: "completed_at", Type: types.PropertyTypeString},
					// rollback_reason is the operator-supplied reason recorded when the
					// Job is transitioned to cancelled or rolled_back by the rollback
					// coordinator (FEAT-20260602-004 Phase 2).
					{Name: "rollback_reason", Type: types.PropertyTypeString},
				},
			},
			eventreceiver.ReceivedEventTypeDefinition("functions"),
		},
	}
}
