package servicemesh

import (
	"io"
	"log/slog"
	"sync"

	ee "github.com/gravestench/eventemitter"
)

// Mesh is the abstract idea of the service mesh, an interface.
//
// The Mesh interface defines the operations that can be performed with
// services, such as adding, removing, and retrieving services. It acts as a
// container for services and uses other interfaces like HasDependencies to
// work with them and do things automatically on their behalf.
type Mesh interface {
	// Add a single service to the Mesh.
	Add(Service) *sync.WaitGroup

	// Remove a specific service from the Mesh.
	Remove(Service) *sync.WaitGroup

	// Services returns a pointer to a slice of interfaces representing the
	// services currently managed by the service Mesh.
	Services() []Service

	Events() *ee.EventEmitter

	Run()
	Shutdown() *sync.WaitGroup

	slogLoggerMethods
}

type slogLoggerMethods interface {
	SetLogHandler(handler slog.Handler)
	SetLogLevel(level slog.Level)
	SetLogDestination(dst io.Writer)
}

// Service represents a generic service within a service mesh.
//
// The Service interface defines the contract that all services in the
// service mesh must adhere to. It provides methods for initializing the service and
// retrieving its name.
type Service interface {
	// Init initializes the service and establishes a connection to the
	// service Mesh.
	Init(mesh Mesh)

	// Name returns the name of the service.
	Name() string
}

// HasDependencies represents a service that can resolve its dependencies.
//
// The HasDependencies interface extends the Service interface and adds
// methods for managing dependencies. It allows services to declare whether
// their dependencies are resolved, as well as a method that attempts to resolve
// those dependencies with the given service mesh.
//
// The mesh will use this interface automatically when a service is added.
// You do not need to implement this interface, it is optional. You would want
// to do this when you have services that depend upon each other to operate
type HasDependencies interface {
	Service

	// DependenciesResolved returns true if all dependencies are resolved. This
	// is up to the service.
	DependenciesResolved() bool

	// ResolveDependencies attempts to resolve the dependencies of the
	// service using the provided Mesh.
	ResolveDependencies(mesh Mesh)
}

// HasLogger is an interface for services that require a logger instance.
//
// The HasLogger interface represents components that depend on a logger for
// logging purposes. It defines a method to set the logger instance.
type HasLogger interface {
	Service
	// UseLogger sets the logger instance for the component.
	SetLogger(l *slog.Logger)
	// Logger yields the logger instance for the component.
	Logger() *slog.Logger
}

// HasGracefulShutdown is an interface for services that require graceful shutdown handling.
//
// The HasGracefulShutdown interface extends the Service interface and adds
// a method for performing custom actions during graceful shutdown.
type HasGracefulShutdown interface {
	Service

	// OnShutdown is called during the graceful shutdown process to perform
	// custom actions before the service is stopped.
	OnShutdown()
}

// EventHandlerServiceAdded is an optional interface. If implemented, it will automatically bind to the
// "Service Added" service mesh event, allowing the handler to respond when a new service is added.
type EventHandlerServiceAdded interface {
	OnServiceAdded(service Service)
}

// EventHandlerServiceRemoved is an optional interface. If implemented, it will automatically bind to the
// "Service Removed" service mesh event, enabling the implementor to respond when a service is removed.
type EventHandlerServiceRemoved interface {
	OnServiceRemoved(service Service)
}

// EventHandlerServiceInitialized is an optional interface. If implemented, it will automatically bind to the
// "Service Initialized" service mesh event, enabling the implementor to respond when a service is initialized.
// When the event is emitted, the declared method will be called and passed the arguments from the emitter.
type EventHandlerServiceInitialized interface {
	OnServiceInitialized(service Service)
}

// EventHandlerServiceEventsBound is an optional interface. If implemented, it will automatically bind to the
// "Service Events Bound" service mesh event, enabling the implementor to respond when events are bound to a service.
// When the event is emitted, the declared method will be called and passed the arguments from the emitter.
type EventHandlerServiceEventsBound interface {
	OnServiceEventsBound(service Service)
}

// EventHandlerServiceLoggerBound is an optional interface. If implemented, it will automatically bind to the
// "Service Logger Bound" service mesh event, enabling the implementor to respond when a logger is bound to a service.
// When the event is emitted, the declared method will be called and passed the arguments from the emitter.
type EventHandlerServiceLoggerBound interface {
	OnServiceLoggerBound(service Service)
}

// EventHandlerServiceMeshRunLoopInitiated is an optional interface. If implemented, it will automatically bind to the
// "mesh Run Loop Initiated" service mesh event, enabling the implementor to respond when the service mesh run loop is initiated.
// When the event is emitted, the declared method will be called and passed the arguments from the emitter.
type EventHandlerServiceMeshRunLoopInitiated interface {
	OnServiceMeshRunLoopInitiated()
}

// EventHandlerServiceMeshShutdownInitiated is an optional interface. If implemented, it will automatically bind to the
// "mesh Shutdown Initiated" service mesh event, enabling the implementor to respond when the service mesh is preparing to shut down.
// When the event is emitted, the declared method will be called and passed the arguments from the emitter.
type EventHandlerServiceMeshShutdownInitiated interface {
	OnServiceMeshShutdownInitiated()
}

// EventHandlerDependencyResolutionStarted is an optional interface. If implemented, it will automatically bind to the
// "Dependency Resolution Started" service mesh event, enabling the implementor to respond when dependency resolution starts.
// When the event is emitted, the declared method will be called and passed the arguments from the emitter.
type EventHandlerDependencyResolutionStarted interface {
	OnDependencyResolutionStarted(service Service)
}

// EventHandlerDependencyResolutionEnded is an optional interface. If implemented, it will automatically bind to the
// "Dependency Resolution Ended" service mesh event, enabling the implementor to respond when dependency resolution ends.
// When the event is emitted, the declared method will be called and passed the arguments from the emitter.
type EventHandlerDependencyResolutionEnded interface {
	OnDependencyResolutionEnded(service Service)
}
