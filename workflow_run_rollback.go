// workflow_run_rollback.go — CodeValdFunctions leg of the WorkflowRun rollback
// coordinator (FEAT-20260602-004 Phase 2 — Functions service).
//
// CodeValdWork's coordinator calls RollbackByWorkflowRun on every service in
// the affected closure. The Functions rule from the FEAT spec is:
//
//   - In-flight Jobs (pending, running, retrying) transition to
//     JobStatusCancelled.
//   - Completed or failed Jobs transition to JobStatusRolledBack as a frozen
//     audit record — function outputs (compile logs, merge artefacts) are
//     preserved for debugging.
//   - Already-cancelled / already-rolled_back Jobs are skipped (idempotent).
package codevaldfunctions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// RollbackByWorkflowRun implements [FunctionsManager.RollbackByWorkflowRun].
func (m *functionsManager) RollbackByWorkflowRun(ctx context.Context, agencyID, workflowRunID, reason string) (RollbackByWorkflowRunResult, error) {
	if workflowRunID == "" {
		return RollbackByWorkflowRunResult{}, ErrWorkflowRunIDRequired
	}

	jobs, err := m.ListJobs(ctx, agencyID, JobFilter{WorkflowRunID: workflowRunID})
	if err != nil {
		return RollbackByWorkflowRunResult{}, fmt.Errorf("RollbackByWorkflowRun %s: %w", workflowRunID, err)
	}

	result := RollbackByWorkflowRunResult{WorkflowRunID: workflowRunID}
	now := time.Now().UTC().Format(time.RFC3339)

	for _, job := range jobs {
		switch job.Status {
		case JobStatusCancelled, JobStatusRolledBack:
			result.SkippedJobIDs = append(result.SkippedJobIDs, job.ID)
			continue

		case JobStatusPending, JobStatusRunning, JobStatusRetrying:
			previous := job.Status
			if err := m.applyRollbackStatus(ctx, agencyID, job.ID, JobStatusCancelled, reason, now); err != nil {
				return result, fmt.Errorf("RollbackByWorkflowRun %s: cancel job %s: %w", workflowRunID, job.ID, err)
			}
			result.CancelledJobIDs = append(result.CancelledJobIDs, job.ID)
			m.publishJSON(ctx, TopicJobCancelled, agencyID, JobCancelledPayload{
				JobID:          job.ID,
				FunctionName:   job.FunctionName,
				WorkflowRunID:  workflowRunID,
				PreviousStatus: previous,
				Reason:         reason,
			})

		case JobStatusCompleted, JobStatusFailed:
			previous := job.Status
			if err := m.applyRollbackStatus(ctx, agencyID, job.ID, JobStatusRolledBack, reason, now); err != nil {
				return result, fmt.Errorf("RollbackByWorkflowRun %s: rollback job %s: %w", workflowRunID, job.ID, err)
			}
			result.RolledBackJobIDs = append(result.RolledBackJobIDs, job.ID)
			m.publishJSON(ctx, TopicJobRolledBack, agencyID, JobRolledBackPayload{
				JobID:          job.ID,
				FunctionName:   job.FunctionName,
				WorkflowRunID:  workflowRunID,
				PreviousStatus: previous,
				Reason:         reason,
			})

		default:
			// Unknown status — log and skip to avoid corrupting an entity in
			// an unexpected state.
			log.Printf("codevaldfunctions: RollbackByWorkflowRun %s: skipping job %s with unknown status %q",
				workflowRunID, job.ID, job.Status)
			result.SkippedJobIDs = append(result.SkippedJobIDs, job.ID)
		}
	}

	return result, nil
}

// applyRollbackStatus patches the job entity with the new terminal status and
// records rollback_reason. completed_at is filled only if not already set, so
// the original terminal timestamp is preserved on completed/failed jobs.
// result, error, retry_count are intentionally left untouched so the audit
// trail and function outputs remain intact.
func (m *functionsManager) applyRollbackStatus(ctx context.Context, agencyID, jobID string, next JobStatus, reason, now string) error {
	patch := map[string]any{
		"status": string(next),
	}
	if reason != "" {
		patch["rollback_reason"] = reason
	}

	current, err := m.dm.GetEntity(ctx, agencyID, jobID)
	if err != nil {
		return fmt.Errorf("get entity: %w", err)
	}
	if s, _ := current.Properties["completed_at"].(string); s == "" {
		patch["completed_at"] = now
	}

	if _, err := m.dm.UpdateEntity(ctx, agencyID, jobID, entitygraph.UpdateEntityRequest{
		Properties: patch,
	}); err != nil {
		return fmt.Errorf("update entity: %w", err)
	}
	return nil
}

// publishJSON marshals v to JSON and publishes it on the given topic. Publish
// errors are logged and swallowed — the FEAT spec is explicit that event
// publishing must never fail a domain mutation.
func (m *functionsManager) publishJSON(ctx context.Context, topic, agencyID string, v any) {
	if m.pub == nil {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("codevaldfunctions: publishJSON: marshal %s: %v", topic, err)
		return
	}
	if err := m.pub.Publish(ctx, eventbus.Event{
		Topic:    topic,
		AgencyID: agencyID,
		Payload:  string(b),
	}); err != nil {
		log.Printf("codevaldfunctions: publishJSON: publish %s: %v", topic, err)
	}
}
