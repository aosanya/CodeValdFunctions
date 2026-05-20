// Package functions holds the in-process function registry and pre-built handlers.
package functions

import (
	"context"

	codevaldelfunctions "github.com/aosanya/CodeValdFunctions"
)

// FunctionHandler executes a registered function for a given Job.
type FunctionHandler func(ctx context.Context, job codevaldelfunctions.Job, payload []byte) (result []byte, err error)

// Registry maps platform event names to their handlers.
type Registry interface {
	Register(eventName string, fn FunctionHandler)
	Lookup(eventName string) (FunctionHandler, bool)
}

type registry struct {
	handlers map[string]FunctionHandler
}

// NewRegistry returns an empty Registry.
func NewRegistry() Registry {
	return &registry{handlers: make(map[string]FunctionHandler)}
}

func (r *registry) Register(eventName string, fn FunctionHandler) {
	r.handlers[eventName] = fn
}

func (r *registry) Lookup(eventName string) (FunctionHandler, bool) {
	fn, ok := r.handlers[eventName]
	return fn, ok
}
