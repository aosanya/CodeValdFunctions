// Package functions provides the binary function runner for CodeValdFunctions.
//
// Functions are standalone executables stored in a directory (default /opt/functions).
// Each function has a JSON manifest sidecar describing its name and trigger event.
// The runner discovers functions by scanning manifests and invokes them via subprocess.
//
// Protocol:
//
//	stdin:  JSON { "job_id": "...", "agency_id": "...", "task_id": "...", "payload": "..." }
//	stdout: JSON { "status": "ok"|"issues-found"|"error", "result": "...", "error": "..." }
//	exit 0: job completes; exit non-zero: job fails (infrastructure error)
package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FunctionManifest is the sidecar JSON file alongside each function binary.
type FunctionManifest struct {
	Name         string            `json:"name"`
	Trigger      string            `json:"trigger"`
	Description  string            `json:"description,omitempty"`
	PayloadMatch map[string]string `json:"payload_match,omitempty"`
}

// Input is the JSON payload sent to a function binary on stdin.
type Input struct {
	JobID    string `json:"job_id"`
	AgencyID string `json:"agency_id"`
	TaskID   string `json:"task_id"`
	Payload  string `json:"payload"`
}

// Output is the JSON response expected from a function binary on stdout.
type Output struct {
	Status string `json:"status"` // "ok", "issues-found", or "error"
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// BinaryRunner discovers and executes function binaries from a directory.
// It rescans manifests on every Lookup so new functions are hot-picked without restart.
type BinaryRunner struct {
	dir string
}

// NewBinaryRunner returns a BinaryRunner that reads functions from dir.
func NewBinaryRunner(dir string) *BinaryRunner {
	return &BinaryRunner{dir: dir}
}

// BinaryPath returns the absolute path where a named function binary is stored.
func (r *BinaryRunner) BinaryPath(name string) string {
	return filepath.Join(r.dir, name)
}

// Lookup scans the functions directory for a manifest whose trigger matches
// triggerEvent and whose payload_match qualifiers (if any) all appear in
// payload. Returns the first match.
func (r *BinaryRunner) Lookup(triggerEvent, payload string) (name, binaryPath string, ok bool) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		log.Printf("functions: runner: read dir %s: %v", r.dir, err)
		return "", "", false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			continue
		}
		var m FunctionManifest
		if err := json.Unmarshal(data, &m); err != nil || m.Trigger != triggerEvent {
			continue
		}
		if !payloadMatches(payload, m.PayloadMatch) {
			continue
		}
		bin := filepath.Join(r.dir, m.Name)
		if info, err := os.Stat(bin); err == nil && !info.IsDir() {
			return m.Name, bin, true
		}
	}
	return "", "", false
}

// payloadMatches returns true when every key=value pair in required appears
// as a literal substring in payload. An empty or nil required always matches.
func payloadMatches(payload string, required map[string]string) bool {
	for k, v := range required {
		needle := `"` + k + `":"` + v + `"`
		if !strings.Contains(payload, needle) {
			return false
		}
	}
	return true
}

// Run executes the binary at binaryPath, feeding input as JSON on stdin.
// It returns the parsed Output regardless of exit code; a non-zero exit with
// unparseable stdout is treated as an infrastructure failure (returned as error).
func (r *BinaryRunner) Run(ctx context.Context, binaryPath string, input Input) (Output, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return Output{}, fmt.Errorf("marshal input: %w", err)
	}

	cmd := exec.Command(binaryPath)
	cmd.Stdin = bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	if stderr.Len() > 0 {
		log.Printf("functions: %s stderr: %s", filepath.Base(binaryPath), strings.TrimSpace(stderr.String()))
	}

	var out Output
	if jsonErr := json.Unmarshal(stdout.Bytes(), &out); jsonErr != nil {
		if runErr != nil {
			return Output{}, fmt.Errorf("binary %s exited %v with unparseable output: %s",
				filepath.Base(binaryPath), runErr, stdout.String())
		}
		return Output{}, fmt.Errorf("parse output from %s: %w\nstdout: %s",
			filepath.Base(binaryPath), jsonErr, stdout.String())
	}
	return out, nil
}

// Deploy writes a function binary and its manifest to the functions directory.
// The binary is made executable. Existing files are overwritten (hot-deploy).
func (r *BinaryRunner) Deploy(name, trigger, description string, binary []byte) error {
	if err := os.MkdirAll(r.dir, 0755); err != nil {
		return fmt.Errorf("deploy: mkdir %s: %w", r.dir, err)
	}
	binPath := filepath.Join(r.dir, name)
	if err := os.WriteFile(binPath, binary, 0755); err != nil {
		return fmt.Errorf("deploy: write binary %s: %w", name, err)
	}
	manifest := FunctionManifest{Name: name, Trigger: trigger, Description: description}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("deploy: marshal manifest: %w", err)
	}
	manifestPath := filepath.Join(r.dir, name+".json")
	if err := os.WriteFile(manifestPath, manifestJSON, 0644); err != nil {
		return fmt.Errorf("deploy: write manifest %s: %w", name+".json", err)
	}
	log.Printf("functions: deployed %s (trigger=%s) to %s", name, trigger, r.dir)
	return nil
}
