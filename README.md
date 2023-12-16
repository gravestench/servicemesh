# About

This module provides an implementation of a "runtime", which is just a wrapper 
for a collection of "services."

The `Runtime` itself merely manages the service lifecycle. Included in this
module are abstract interfaces for the runtime (`IsRuntime`) and runtime 
services (`IsRuntimeService`), as well as other interfaces for logging, 
dependency injection, and graceful shutdown.

## Examples & Usage
```go
import "github.com/gravestench/servicemesh"
```
```go
// compilation error if we dont implement servicemesh.Service
var _ runtime.Service = &MyService{}
```
```golang
// minimal service implementation
type MyService struct {
	// Your service fields
}

func (s *MyService) Init(manager runtime.IsRuntime) {
	// Initialization logic for your service
}

func (s *MyService) Name() string {
	return "MyService"
}
```
```golang
// main.go should look like this
func main() {
	// Create a new instance of the Runtime Manager
	r := runtime.New("My Runtime")

	// Create an instance of your service
	service := &MyService{}

	// Add the service to the Runtime Manager
	r.Add(service)

	// Run the Runtime Manager
	r.Run()
}
```

In this example, we create a new instance of the `Runtime` manager using `runtime.New()`,
add our service using `runtime.Add()`, and then run the manager with `runtime.Run()`.
The manager takes care of initializing the service and managing its lifecycle.

for more examples see [the examples repo](https://github.com/gravestench/runtime-examples).

## Adding Services

To add a service to the Runtime Manager, you need to create a struct that implements the
`IsRuntimeService` interface. This interface requires the implementation of the
`Init()` and `Name()` methods.

```go
type IsRuntimeService interface {
	Init(manager IsRuntimeService)
	Name() string
}
```

The `Init()` method is called during the initialization phase of the service and allows
you to perform any necessary setup. The `Name()` method returns the name of the service.

You can then add your service to the Runtime Manager using the `Add()` method:

```go
runtime.Add(service)
```

## Graceful Shutdown

The Runtime Manager supports graceful shutdown by listening for the interrupt signal
(`os.Interrupt`). When the interrupt signal is received, the manager initiates the
shutdown process and allows the services to perform cleanup operations. You can trigger
the shutdown by pressing `Ctrl+C` in the console.

```go
runtime.Run()
```

During the shutdown process, the `Run()` method blocks until the interrupt signal is
received. Once the signal is received, the manager calls the `OnQuit()` method of each
service, allowing them to perform any necessary cleanup. You can implement the cleanup
logic within the `Quit()` method of your service.

```go
func (s *MyService) Quit() {
	// Cleanup logic for your service
}
```

## Logging Integration

The Runtime Manager integrates with the `zerolog` logging library to provide logging
capabilities for your services. The manager automatically initializes a logger and passes
it to the services that implement the `HasLogger` interface.

To use the logger within your service, you need to implement the `HasLogger` interface
and assign the logger to your service using the `UseLogger()` method:

```go
type HasLogger interface {
    IsRuntimeService
    BindLogger(logger *zerolog.Logger)
    Logger() *zerolog.Logger
}
```

```go
func (s *MyService) BindLogger(logger *zerolog.Logger) {
	// Assign the logger to your service
	s.logger = logger
}
```

With the logger assigned, you can use it within your service to log messages:

```go
myService.logger.Info().Msg("foo")
```

Make sure to import the `zerolog` library and create a logger instance within your
service.

## Interfaces

The `pkg` package provides several interfaces that define the contracts for managing
services within the IsRuntime and implementing specific functionalities. These
interfaces are designed to promote modularity and extensibility in your codebase.

### IsRuntime

The `IsRuntime` describes the contract of the concrete Runtime implementation that
is included in this repo.

```go
type IsRuntime interface {
    Add(IsRuntimeService)
    Remove(IsRuntimeService)
    Services() *[]IsRuntimeService
    Shutdown()
}
```

### IsRuntimeService

The `IsRuntimeService` interface represents a generic service within the
`IsRuntime`. It defines methods for initializing the service and retrieving its
name.

```go
type IsRuntimeService interface {
    Init(IsRuntime)
    Name() string
}
```

### HasDependencies

The `HasDependencies` interface extends the `IsRuntimeService` interface and
adds methods for managing dependencies. It allows services to declare their dependencies,
and to declare when they are resolved. The concrete implementation of the `Runtime` will
use this `HasDependencies` interface to resolves any dependencies before the `Init()`
method of a given service is invoked. This is an optional interface, your services do not
need to implement this.

```go
type HasDependencies interface {
	IsRuntimeService
    DependenciesResolved() bool
    ResolveDependencies(IsRuntime)
}
```

### HasLogger

The `HasLogger` interface represents components that depend on a logger for logging
purposes. It defines methods for setting the logger instance and retrieving the logger.
This is an optional interface, your services do not need to implement this.

```go
type HasLogger interface {
    UseLogger(logger *zerolog.Logger)
    Logger() *zerolog.Logger
}
```

This interface can be implemented by your services to define their behavior and
interactions with the runtime. They enable flexible dependency resolution,
logging integration, and more.

Make sure to import the `github.com/rs/zerolog` library for using the `zerolog.Logger`
type in your service implementations.

### HasGracefulShutdown

The `HasGracefulShutdown` interface is an extension of the `IsRuntimeService`
interface that provides a standardized way to handle graceful shutdown for services. It
defines the `OnShutdown()` method, which allows services to perform custom actions before
they are stopped during the shutdown process.

To use the `HasGracefulShutdown` interface, implement it in your service struct and
provide the implementation for the `OnShutdown()` method.

```go
type MyService struct {
	// Service fields
}

func (s *MyService) Init(manager IsRuntime) {
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

Contributions are welcome! If you have any ideas, suggestions, or bug reports, please
open an issue or submit a pull request. Let's make this package even better together.

## License

This project is licensed under the [MIT License](LICENSE).