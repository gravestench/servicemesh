package servicemesh

const (
	EventServiceAdded       = "service added"
	EventServiceRemoved     = "service removed"
	EventServiceInitialized = "service initialized"
	EventServiceEventsBound = "service events bound"
	EventServiceLoggerBound = "service logger bound"

	EventServiceMeshRunLoopInitiated  = "run-loop initiated"
	EventServiceMeshShutdownInitiated = "shutdown initiated"

	EventDependencyResolutionStarted = "dependency resolution start"
	EventDependencyResolutionEnded   = "dependency resolution end"
)
