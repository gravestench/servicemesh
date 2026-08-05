package servicemesh

import "errors"

var (
	// ErrDependencyResolutionCanceled indicates that mesh shutdown interrupted
	// dependency resolution.
	ErrDependencyResolutionCanceled = errors.New("dependency resolution canceled")

	// ErrDependencyResolutionTimeout indicates that a service did not resolve
	// its dependencies before the configured timeout.
	ErrDependencyResolutionTimeout = errors.New("dependency resolution timed out")
)
