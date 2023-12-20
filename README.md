# About

This module provides an implementation of a "service mesh", which represents a
collection of "services."

Included in this module are abstract interfaces for the service mesh and 
services (see the `Mesh` and `Service` interfaces), as well as other 
integration interfaces for logging, dependency injection, and graceful shutdown.

When using this module as a library for developing software, it necessitates the
architecture of any given application be a composition, or mesh, of services.
The mesh itself governs the lifecycle of the services, and therefore, the 
application as a whole.

At the highest level, typically in a `main.go` file, an application would look
something like this:
```golang
func main() {
    app := servicemesh.New("My App")
	
    app.Add(&foo.Service{})
    app.Add(&bar.Service{})
    // add your other services here

    app.Run()
}
```

We can see that no particular service is responsible for invoking the run-loop
of the service mesh; we invoke this run-loop one time in the `main` func of the 
application. We also dont manually assign any dependencies, or invoke the `Init` 
method of a service. This is all managed by the service mesh. This allows the
mesh to perform dependency-injection, standard logger instantiation, 
and event-handler callback binding.

# Examples
For examples see [the examples repo](https://github.com/gravestench/servicemesh-examples).


## Usage
This is the contract that all services must honor:
```golang
type Service interface {
    Init(mesh Mesh)
    Name() string
}
```
Here is a trivial example service:
```golang
// minimal service implementation
type fooService struct {}

func (s *fooService) Init(manager servicemesh.Mesh) {
	// Initialization logic for your service
}

func (s *fooService) Name() string {
	return "Foo"
}
```
```golang
// main.go would look like this
func main() {
    // Create the mesh instance
    mesh := servicemesh.New()
	
    // Add the service
    mesh.Add(&fooService{}) 
    
    // invoke the run-loop (blocking call)
    mesh.Run()
}
```

## Adding Services

To add a service to the mesh, you need to create a struct that implements the
`Service` interface. This interface requires the implementation of the
`Init()` and `Name()` methods.

```go
type Service interface {
	Init(mesh M)
	Name() string
}
```

The `Init()` method is called during the initialization phase of the service and allows
you to perform any necessary setup. The `Name()` method returns the name of the service.

You can then add your service to the Manager using the `Add()` method:

```go
mesh.Add(service)
```

## Graceful Shutdown

The Manager supports graceful shutdown by listening for the interrupt signal
(`os.Interrupt`). When the interrupt signal is received, the manager initiates the
shutdown process and allows the services to perform cleanup operations. You can trigger
the shutdown by pressing `Ctrl+C` in the console.

```go
mesh.Run() // this is blocking until the interrupt fires
```

The `Run()` method blocks until the interrupt signal is
received. Once the signal is received, the mesh calls the `OnShutdown()` method of each
service, allowing them to perform any necessary cleanup. You can implement the cleanup
logic within the `OnShutdown()` method of your service.

```go
func (s *MyService) OnShutdown() {
	// Cleanup logic for your service
}
```

## Logging Integration

The Manager integrates with the `slog` logging module to provide logging 
capabilities for your services. The manager automatically initializes a logger 
and passes it to the services that implement the `HasLogger` interface.

To use the logger within your service, you need to implement the `HasLogger` 
interface. The manager will invoke the `SetLogger` method automatically when the
service is added to the mesh.

```go
type HasLogger interface {
    Service
    SetLogger(logger *slog.Logger)
    Logger() *slog.Logger
}
```

```go
func (s *MyService) SetLogger(logger *slog.Logger) {
	// Assign the logger to your service
	s.logger = logger
}
```

With the logger assigned, you can use it within your service to log messages:

```go
myService.logger.Info("foo")
```

## Interfaces

This module provides several interfaces that define the contracts for managing
services within the mesh and implementing specific functionalities. These
interfaces are designed to promote modularity and extensibility in your codebase.

### Mesh

The `Mesh` interface describes the contract of the service mesh. The concrete
implementation of this interface is defined in this module, but it is not 
exported. All you need to know about the Mesh, as a user of this module, is the 
following interface:

```go
type Mesh interface {
    Add(Service) *sync.WaitGroup
    Remove(Service) *sync.WaitGroup
    Run()
    Shutdown() *sync.WaitGroup
    
	Services() []Service
    
	SetLogHandler(handler slog.Handler)
    SetLogLevel(level slog.Level)
    SetLogDestination(dst io.Writer)
    
    Events() *ee.EventEmitter
}
```

### Service

The `Service` interface represents a generic service within the
`Mesh` interface. It defines methods for initializing the service, retrieving
its name, and a method that returns whether the service is ready to be used.

```go
type Service interface {
    Init(Mesh)
    Name() string
    Ready() bool
}
```

### HasDependencies

The `HasDependencies` interface extends the `Service` interface and
adds methods for managing dependencies. It allows services to declare their 
dependencies, and to declare when they are resolved. The concrete implementation
of the `Mesh` interface will use this `HasDependencies` interface to resolves 
any dependencies before the `Init()` method of a given service is invoked. This 
is an optional interface, your services do not need to implement this.

```go
type HasDependencies interface {
	Service
    DependenciesResolved() bool
    ResolveDependencies(services []servicemesh.Service)
}
```

### HasLogger

The `HasLogger` interface represents services that depend on a logger for 
logging purposes. It defines methods for setting the logger instance and 
retrieving the logger. This is an optional interface, your services do not need 
to implement this.

```go
type HasLogger interface {
    SetLogger(logger *slog.Logger)
    Logger() *slog.Logger
}
```

This interface can be implemented by your services to define their behavior and
interactions with the service mesh. They enable flexible dependency resolution,
logging integration, and more.

Make sure to import the `log/slog` library for using the `slog.Logger`
type in your service implementations.

### HasGracefulShutdown

The `HasGracefulShutdown` interface is an extension of the `Service`
interface that provides a standardized way to handle graceful shutdown for 
services. It defines the `OnShutdown()` method, which allows services to perform
custom actions before they are stopped during the shutdown process.

To use the `HasGracefulShutdown` interface, implement it in your service struct 
and provide the implementation for the `OnShutdown()` method.

```go
type MyService struct {
	// Service fields
}

func (s *MyService) Init(m Mesh) {
	// Initialization logic for your service
}

func (s *MyService) Name() string {
    return "MyService"
}

func (s *MyService) Ready() bool {
    return true
}

func (s *MyService) OnShutdown() {
	// Custom shutdown logic for your service
}
```

## Events
The mesh comes integrated with an [event emitter](https://github.com/gravestench/eventemitter), 
which is modeled after the `ee3` implementation in javascript. This is a 
singleton instance and is referred to as the "event bus." There is a single 
method of the `Mesh` (`Events()`) that will yield this singleton event emitter 
instance, and all services will have an opportunity to use or store a reference 
to this event emitter during their `Init` methods.

The `Mesh` has a list of events that it will emit during normal operation:
```golang
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
```

As opposed to forcing direct usage of the event emitter instance, there are a 
handful of integration interfaces which can be optionally implemented by a 
service. These can be found in `interfaces.go`. The concrete implementation
of the mesh found in this module dog-foods the event-bus and event handler
integration interfaces, and is actually a `Service` too. Much of the logging 
functionality is implemented through event handlers for events it is emitting.

### NOTE
Notice that the `Add`, `Remove`, and `Shutdown` methods of the `Mesh` each 
yield a `sync.Waitgroup` instance. This allows the caller an opportunity to wait 
for event-handler callbacks to finish executing:
```golang
mesh := servicemesh.New()
mesh.Add(&foo.Service{}).Wait() // blocking call
```

This functionality can be especially handy in a scenario where you have services
that are responsible for managing instances of subordinate services.

## Contributing

Contributions are welcome! If you have any ideas, suggestions, or bug reports, 
please open an issue or submit a pull request. Let's make this package even 
better together.

## License

This project is licensed under the [MIT License](LICENSE).