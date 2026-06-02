package server

import "testing"

// TestExtractWorkflowRunID covers the start-pipeline back-fill path:
// the dispatcher reads workflow_run_id from a function's JSON result so it
// can stamp it on the originating Job (whose trigger had no run ID).
func TestExtractWorkflowRunID(t *testing.T) {
	cases := []struct {
		name   string
		result string
		want   string
	}{
		{"empty result", "", ""},
		{"non-json", "not json", ""},
		{"json without field", `{"status":"ok"}`, ""},
		{"json with empty field", `{"workflow_run_id":""}`, ""},
		{"start-pipeline minted run", `{"workflow_run_id":"wfr_01J","status":"ok"}`, "wfr_01J"},
		{"nested-only field is not picked up", `{"data":{"workflow_run_id":"wfr_x"}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractWorkflowRunID(tc.result); got != tc.want {
				t.Errorf("extractWorkflowRunID(%q) = %q, want %q", tc.result, got, tc.want)
			}
		})
	}
}
