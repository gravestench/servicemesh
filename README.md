# About

This module provides an implementation of a "service mesh", which is just a 
wrapper for a collection of "services."

The `M` interface itself merely manages the service lifecycle. Included in this
module are abstract interfaces for the service mesh (`M` interface) and service mesh 
services (`Service`), as well as other interfaces for logging, 
dependency injection, and graceful shutdown. A concrete implementation of the 
`M` interface is provided (see `Mesh`).

# Examples
For examples see [the examples repo](https://github.com/gravestench/servicemesh-examples).


## Usage
```go
import "github.com/gravestench/servicemesh"
```
```go
// compilation error if we dont implement servicemesh.Service
var _ servicemesh.Service = &MyService{}
```
```golang
// minimal service implementation
type MyService struct {
	// Your service fields
}

func (s *MyService) Init(manager servicemesh.Mesh) {
	// Initialization logic for your service
}

func (s *MyService) Name() string {
	return "MyService"
}
```
```golang
// main.go should look like this
func main() {
	// Create a new instance of the Manager
	manager := servicemesh.New("My App")

	// Create an instance of your service
	service := &MyService{}

	// Add the service to the Manager
	manager.Add(service)

	// Run the Manager
	manager.Run()
}
```

In this example, we: 
1) create a mesh manager instance using `servicemesh.New`
2) add our service using `mesh.Add`
3) and then run the manager with `mesh.Run`.


The manager takes care of initializing the service and managing its lifecycle.

## Adding Services

To add a service to the mesh, you need to create a struct that implements the
`Service` interface. This interface requires the implementation of the
`Init()` and `Name()` methods.

```go
type Service interface {
	Init(m Mesh)
	Name() string
}
```

The `Init()` method is called during the initialization phase of the service and allows
you to perform any necessary setup. The `Name()` method returns the name of the service.

You can then add your service to the Manager using the `Add()` method:

```go
manager.Add(service)
```

## Graceful Shutdown

The Manager supports graceful shutdown by listening for the interrupt signal
(`os.Interrupt`). When the interrupt signal is received, the manager initiates the
shutdown process and allows the services to perform cleanup operations. You can trigger
the shutdown by pressing `Ctrl+C` in the console.

```go
manager.Run() // this is blocking until the interrupt fires
```

The `Run()` method blocks until the interrupt signal is
received. Once the signal is received, the manager calls the `OnShutdown()` method of each
service, allowing them to perform any necessary cleanup. You can implement the cleanup
logic within the `OnShutdown()` method of your service.

```go
func (s *MyService) OnShutdown() {
	// Cleanup logic for your service
}
```

## Logging Integration

The Manager integrates with the `slog` logging module to provide logging
capabilities for your services. The manager automatically initializes a logger and passes
it to the services that implement the `HasLogger` interface.

To use the logger within your service, you need to implement the `HasLogger` interface.
The manager will invoke the `SetLogger` method automatically when the service
is added to the mesh.

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

The `M` interface describes the contract of the concrete service `Mesh` 
implementation that is included in this repo.

```go
type Mesh interface {
    Add(Service)
    Remove(Service)
    Services() *[]Service
    Shutdown()
}
```

### Service

The `Service` interface represents a generic service within the
`M` interface. It defines methods for initializing the service and retrieving its name.

```go
type Service interface {
    Init(Mesh)
    Name() string
}
```

### HasDependencies

The `HasDependencies` interface extends the `Service` interface and
adds methods for managing dependencies. It allows services to declare their 
dependencies, and to declare when they are resolved. The concrete implementation
of the `M` interface will use this `HasDependencies` interface to resolves any 
dependencies before the `Init()` method of a given service is invoked. This is 
an optional interface, your services do not need to implement this.

```go
type HasDependencies interface {
	Service
    DependenciesResolved() bool
    ResolveDependencies(mesh M)
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

func (s *MyService) OnShutdown() {
	// Custom shutdown logic for your service
}
```

## Contributing

Contributions are welcome! If you have any ideas, suggestions, or bug reports, 
please open an issue or submit a pull request. Let's make this package even 
better together.

## License

This project is licensed under the [MIT License](LICENSE).