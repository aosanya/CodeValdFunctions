package codevaldelfunctions

import (
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// jobFromEntity reconstructs a Job from an entitygraph Entity.
func jobFromEntity(e entitygraph.Entity) Job {
	return Job{
		ID:             e.ID,
		AgencyID:       e.AgencyID,
		Status:         JobStatus(entitygraph.StringProp(e.Properties, "status")),
		FunctionName:   entitygraph.StringProp(e.Properties, "function_name"),
		TriggerEvent:   entitygraph.StringProp(e.Properties, "trigger_event"),
		TriggerPayload: entitygraph.StringProp(e.Properties, "trigger_payload"),
		Result:         entitygraph.StringProp(e.Properties, "result"),
		Error:          entitygraph.StringProp(e.Properties, "error"),
		RetryCount:     intProp(e.Properties, "retry_count"),
		CreatedAt:      parseTime(entitygraph.StringProp(e.Properties, "created_at")),
		StartedAt:      parseTime(entitygraph.StringProp(e.Properties, "started_at")),
		CompletedAt:    parseTime(entitygraph.StringProp(e.Properties, "completed_at")),
	}
}

// intProp reads a numeric property and returns it as int. ArangoDB decodes all
// JSON numbers as float64, so we accept both int and float64.
func intProp(props map[string]any, key string) int {
	v, ok := props[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// parseTime parses an RFC 3339 timestamp string into time.Time.
// Returns the zero value for empty or unparseable strings.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
