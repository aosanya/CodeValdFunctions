// Package codevaldfunctions implements the CodeValdFunctions gRPC service.
package codevaldfunctions

import "errors"

var (
	// ErrJobNotFound is returned when a Job entity cannot be located.
	ErrJobNotFound = errors.New("job not found")

	// ErrInvalidJobTransition is returned when a state transition is not permitted
	// by the Job state machine.
	ErrInvalidJobTransition = errors.New("invalid job state transition")

	// ErrFunctionNotFound is returned when no handler is registered for the
	// requested function name.
	ErrFunctionNotFound = errors.New("function not found")

	// ErrWorkflowRunIDRequired is returned by [FunctionsManager.RollbackByWorkflowRun]
	// when the caller passes an empty workflow_run_id. A global "rollback every
	// Job" sweep is intentionally not supported.
	ErrWorkflowRunIDRequired = errors.New("workflow_run_id is required")
)
