package codevaldelfunctions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// maxRetries is the number of times a Job may be retried before transitioning
// to failed instead of retrying.
const maxRetries = 3

// jobTypeID is the TypeDefinition name used when creating Job entities.
const jobTypeID = "Job"

// functionsManager is the concrete implementation of [FunctionsManager].
type functionsManager struct {
	dm       entitygraph.DataManager
	pub      CrossPublisher
	agencyID string
}

// NewFunctionsManager constructs a FunctionsManager backed by the provided
// DataManager. agencyID is the agency this instance is scoped to.
// pub may be nil (disables event publishing).
func NewFunctionsManager(dm entitygraph.DataManager, pub CrossPublisher, agencyID string) FunctionsManager {
	return &functionsManager{dm: dm, pub: pub, agencyID: agencyID}
}

// CreateJob creates a new Job entity in the pending state.
func (m *functionsManager) CreateJob(ctx context.Context, agencyID, functionName, triggerEvent, payload, taskID string) (Job, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	entity, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   jobTypeID,
		Properties: map[string]any{
			"status":          string(JobStatusPending),
			"function_name":   functionName,
			"trigger_event":   triggerEvent,
			"trigger_payload": payload,
			"task_id":         taskID,
			"result":          "",
			"error":           "",
			"retry_count":     0,
			"created_at":      now,
			"started_at":      "",
			"completed_at":    "",
		},
	})
	if err != nil {
		return Job{}, fmt.Errorf("CreateJob: %w", err)
	}
	job := jobFromEntity(entity)
	m.publish(ctx, "functions.job.created", agencyID, job.ID)
	return job, nil
}

// GetJob retrieves a single Job by ID.
func (m *functionsManager) GetJob(ctx context.Context, agencyID, jobID string) (Job, error) {
	entity, err := m.dm.GetEntity(ctx, agencyID, jobID)
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return Job{}, ErrJobNotFound
		}
		return Job{}, fmt.Errorf("GetJob: %w", err)
	}
	if entity.TypeID != jobTypeID {
		return Job{}, ErrJobNotFound
	}
	return jobFromEntity(entity), nil
}

// ListJobs returns all Job entities for the agency, optionally filtered.
func (m *functionsManager) ListJobs(ctx context.Context, agencyID string, filter JobFilter) ([]Job, error) {
	props := map[string]any{}
	if filter.Status != "" {
		props["status"] = string(filter.Status)
	}
	if filter.FunctionName != "" {
		props["function_name"] = filter.FunctionName
	}
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     jobTypeID,
		Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("ListJobs: %w", err)
	}
	jobs := make([]Job, len(entities))
	for i, e := range entities {
		jobs[i] = jobFromEntity(e)
	}
	return jobs, nil
}

// StartJob transitions a Job from pending or retrying → running.
func (m *functionsManager) StartJob(ctx context.Context, agencyID, jobID string) (Job, error) {
	job, err := m.GetJob(ctx, agencyID, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != JobStatusPending && job.Status != JobStatusRetrying {
		return Job{}, fmt.Errorf("%w: %s → running", ErrInvalidJobTransition, job.Status)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	updated, err := m.dm.UpdateEntity(ctx, agencyID, jobID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"status":     string(JobStatusRunning),
			"started_at": now,
		},
	})
	if err != nil {
		return Job{}, fmt.Errorf("StartJob: %w", err)
	}
	return jobFromEntity(updated), nil
}

// CompleteJob transitions a running Job → completed.
func (m *functionsManager) CompleteJob(ctx context.Context, agencyID, jobID, result string) (Job, error) {
	job, err := m.GetJob(ctx, agencyID, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != JobStatusRunning {
		return Job{}, fmt.Errorf("%w: %s → completed", ErrInvalidJobTransition, job.Status)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	updated, err := m.dm.UpdateEntity(ctx, agencyID, jobID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"status":       string(JobStatusCompleted),
			"result":       result,
			"completed_at": now,
		},
	})
	if err != nil {
		return Job{}, fmt.Errorf("CompleteJob: %w", err)
	}
	out := jobFromEntity(updated)
	m.publish(ctx, "functions.job.completed", agencyID, jobID)
	return out, nil
}

// FailJob transitions a running Job → failed (when retries exhausted) or
// → retrying (when retry_count < maxRetries).
func (m *functionsManager) FailJob(ctx context.Context, agencyID, jobID, errMsg string) (Job, error) {
	job, err := m.GetJob(ctx, agencyID, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != JobStatusRunning {
		return Job{}, fmt.Errorf("%w: %s → failed/retrying", ErrInvalidJobTransition, job.Status)
	}
	newCount := job.RetryCount + 1
	newStatus := JobStatusRetrying
	if newCount >= maxRetries {
		newStatus = JobStatusFailed
	}
	props := map[string]any{
		"status":      string(newStatus),
		"error":       errMsg,
		"retry_count": newCount,
	}
	if newStatus == JobStatusFailed {
		props["completed_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	updated, err := m.dm.UpdateEntity(ctx, agencyID, jobID, entitygraph.UpdateEntityRequest{
		Properties: props,
	})
	if err != nil {
		return Job{}, fmt.Errorf("FailJob: %w", err)
	}
	out := jobFromEntity(updated)
	if newStatus == JobStatusFailed {
		m.publish(ctx, "functions.job.failed", agencyID, jobID)
	}
	return out, nil
}

// CancelJob transitions a pending or running Job → cancelled.
func (m *functionsManager) CancelJob(ctx context.Context, agencyID, jobID string) (Job, error) {
	job, err := m.GetJob(ctx, agencyID, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != JobStatusPending && job.Status != JobStatusRunning {
		return Job{}, fmt.Errorf("%w: %s → cancelled", ErrInvalidJobTransition, job.Status)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	updated, err := m.dm.UpdateEntity(ctx, agencyID, jobID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"status":       string(JobStatusCancelled),
			"completed_at": now,
		},
	})
	if err != nil {
		return Job{}, fmt.Errorf("CancelJob: %w", err)
	}
	return jobFromEntity(updated), nil
}

// publish fires an event if a publisher is wired. Errors are logged but do not
// affect the caller — the Job has already been persisted.
func (m *functionsManager) publish(ctx context.Context, topic, agencyID, jobID string) {
	if m.pub == nil {
		return
	}
	_ = m.pub.Publish(ctx, eventbus.Event{
		Topic:    topic,
		AgencyID: agencyID,
		Payload:  map[string]string{"job_id": jobID},
	})
}
