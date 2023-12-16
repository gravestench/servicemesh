package servicemesh

const (
	EventServiceAdded       = "service added"
	EventServiceRemoved     = "service removed"
	EventServiceInitialized = "service initialized"
	EventServiceEventsBound = "service events bound"
	EventServiceLoggerBound = "service logger bound"

	EventRuntimeRunLoopInitiated  = "runtime begin"
	EventRuntimeShutdownInitiated = "runtime shutdown"

	EventDependencyResolutionStarted = "runtime dependency resolution start"
	EventDependencyResolutionEnded   = "runtime dependency resolution end"
)
