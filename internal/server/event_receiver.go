package server

import (
	"context"
	"encoding/json"
	"log"
	"time"

	codevaldelfunctions "github.com/aosanya/CodeValdFunctions"
	"github.com/aosanya/CodeValdFunctions/internal/functions"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	sharedev1 "github.com/aosanya/CodeValdSharedLib/gen/go/codevaldshared/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EventReceiverServer implements sharedev1.EventReceiverServiceServer.
// Cross calls NotifyEvent to push a subscribed event; the handler writes a
// ReceivedEvent record for deduplication before returning.
// Dispatch is fired asynchronously after the ACK.
type EventReceiverServer struct {
	sharedev1.UnimplementedEventReceiverServiceServer
	backend    entitygraph.DataManager
	agencyID   string
	dispatcher *FunctionsDispatcher
}

// NewEventReceiver constructs an EventReceiverServer.
func NewEventReceiver(backend entitygraph.DataManager, agencyID string, dispatcher *FunctionsDispatcher) *EventReceiverServer {
	return &EventReceiverServer{backend: backend, agencyID: agencyID, dispatcher: dispatcher}
}

// NotifyEvent receives a pushed event from Cross.
// Deduplicates by event_id; fires dispatcher asynchronously on new events.
func (s *EventReceiverServer) NotifyEvent(ctx context.Context, req *sharedev1.NotifyEventRequest) (*sharedev1.NotifyEventResponse, error) {
	eventID := req.GetEventId()

	if eventID != "" {
		existing, err := s.backend.ListEntities(ctx, entitygraph.EntityFilter{
			AgencyID: s.agencyID,
			TypeID:   "ReceivedEvent",
			Properties: map[string]any{
				"event_id": eventID,
			},
		})
		if err == nil && len(existing) > 0 {
			log.Printf("codevaldelfunctions: NotifyEvent: duplicate event_id=%s topic=%s — skipping", eventID, req.GetTopic())
			return &sharedev1.NotifyEventResponse{}, nil
		}
	}

	_, err := s.backend.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: s.agencyID,
		TypeID:   "ReceivedEvent",
		Properties: map[string]any{
			"event_id":    eventID,
			"topic":       req.GetTopic(),
			"agency_id":   req.GetAgencyId(),
			"source":      req.GetSource(),
			"payload":     req.GetPayload(),
			"received_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("codevaldelfunctions: NotifyEvent: write ReceivedEvent: %v", err)
		return nil, status.Errorf(codes.Internal, "log received event: %v", err)
	}
	log.Printf("codevaldelfunctions: NotifyEvent: ACK event_id=%s topic=%s source=%s",
		eventID, req.GetTopic(), req.GetSource())

	if s.dispatcher != nil {
		go s.dispatcher.Dispatch(context.Background(), req.GetTopic(), req.GetPayload())
	}

	return &sharedev1.NotifyEventResponse{}, nil
}

// FunctionsDispatcher discovers and executes function binaries when an event arrives.
type FunctionsDispatcher struct {
	mgr      codevaldelfunctions.FunctionsManager
	runner   *functions.BinaryRunner
	agencyID string
}

// NewFunctionsDispatcher constructs a FunctionsDispatcher.
func NewFunctionsDispatcher(mgr codevaldelfunctions.FunctionsManager, runner *functions.BinaryRunner, agencyID string) *FunctionsDispatcher {
	return &FunctionsDispatcher{mgr: mgr, runner: runner, agencyID: agencyID}
}

// Dispatch looks up the binary for topic, creates a Job, runs the binary,
// then marks the Job completed or failed based on the output.
func (d *FunctionsDispatcher) Dispatch(ctx context.Context, topic, payload string) {
	name, binPath, ok := d.runner.Lookup(topic, payload)
	if !ok {
		return
	}

	var meta struct {
		TaskID string `json:"task_id"`
	}
	_ = json.Unmarshal([]byte(payload), &meta)

	job, err := d.mgr.CreateJob(ctx, d.agencyID, name, topic, payload, meta.TaskID)
	if err != nil {
		log.Printf("codevaldelfunctions: Dispatch: CreateJob topic=%s: %v", topic, err)
		return
	}

	job, err = d.mgr.StartJob(ctx, d.agencyID, job.ID)
	if err != nil {
		log.Printf("codevaldelfunctions: Dispatch: StartJob job=%s: %v", job.ID, err)
		return
	}

	out, runErr := d.runner.Run(ctx, binPath, functions.Input{
		JobID:    job.ID,
		AgencyID: d.agencyID,
		TaskID:   meta.TaskID,
		Payload:  payload,
	})
	if runErr != nil {
		log.Printf("codevaldelfunctions: Dispatch: run binary job=%s: %v", job.ID, runErr)
		if _, err := d.mgr.FailJob(ctx, d.agencyID, job.ID, runErr.Error()); err != nil {
			log.Printf("codevaldelfunctions: Dispatch: FailJob job=%s: %v", job.ID, err)
		}
		return
	}

	if out.Status == "error" {
		if _, err := d.mgr.FailJob(ctx, d.agencyID, job.ID, out.Error); err != nil {
			log.Printf("codevaldelfunctions: Dispatch: FailJob job=%s: %v", job.ID, err)
		}
		return
	}

	if _, err := d.mgr.CompleteJob(ctx, d.agencyID, job.ID, out.Result); err != nil {
		log.Printf("codevaldelfunctions: Dispatch: CompleteJob job=%s: %v", job.ID, err)
	}
}
