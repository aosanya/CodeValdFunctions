package codevaldfunctions

import (
	"context"
	"fmt"
	"sync"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/google/uuid"
)

// fakeDataManager is an in-memory entitygraph.DataManager for unit tests.
type fakeDataManager struct {
	mu       sync.Mutex
	entities map[string]entitygraph.Entity // key: agencyID+"/"+entityID
}

func newFakeDataManager() *fakeDataManager {
	return &fakeDataManager{entities: make(map[string]entitygraph.Entity)}
}

func (f *fakeDataManager) key(agencyID, entityID string) string {
	return agencyID + "/" + entityID
}

func (f *fakeDataManager) CreateEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	props := req.Properties
	if props == nil {
		props = map[string]any{}
	}
	e := entitygraph.Entity{
		ID:         uuid.NewString(),
		AgencyID:   req.AgencyID,
		TypeID:     req.TypeID,
		Properties: props,
	}
	f.entities[f.key(req.AgencyID, e.ID)] = e
	return e, nil
}

func (f *fakeDataManager) GetEntity(_ context.Context, agencyID, entityID string) (entitygraph.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entities[f.key(agencyID, entityID)]
	if !ok {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	return e, nil
}

func (f *fakeDataManager) UpdateEntity(_ context.Context, agencyID, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entities[f.key(agencyID, entityID)]
	if !ok {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	for k, v := range req.Properties {
		e.Properties[k] = v
	}
	f.entities[f.key(agencyID, entityID)] = e
	return e, nil
}

func (f *fakeDataManager) DeleteEntity(_ context.Context, agencyID, entityID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.entities, f.key(agencyID, entityID))
	return nil
}

func (f *fakeDataManager) ListEntities(_ context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []entitygraph.Entity
	for _, e := range f.entities {
		if e.AgencyID != filter.AgencyID {
			continue
		}
		if filter.TypeID != "" && e.TypeID != filter.TypeID {
			continue
		}
		match := true
		for k, v := range filter.Properties {
			if fmt.Sprintf("%v", e.Properties[k]) != fmt.Sprintf("%v", v) {
				match = false
				break
			}
		}
		if match {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeDataManager) UpsertEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	return f.CreateEntity(ctx, req)
}

func (f *fakeDataManager) CreateRelationship(_ context.Context, _ entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	return entitygraph.Relationship{}, nil
}

func (f *fakeDataManager) GetRelationship(_ context.Context, _, _ string) (entitygraph.Relationship, error) {
	return entitygraph.Relationship{}, entitygraph.ErrRelationshipNotFound
}

func (f *fakeDataManager) DeleteRelationship(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeDataManager) ListRelationships(_ context.Context, _ entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	return nil, nil
}

func (f *fakeDataManager) TraverseGraph(_ context.Context, _ entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	return entitygraph.TraverseGraphResult{}, nil
}
