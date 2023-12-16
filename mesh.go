package servicemesh

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	ee "github.com/gravestench/eventemitter"
)

var _ M = &Mesh{}

// Mesh represents a collection of service mesh services.
type Mesh struct {
	name         string
	quit         chan os.Signal
	services     []Service
	logger       *slog.Logger
	logOutput    io.Writer
	logLevel     int
	events       *ee.EventEmitter
	shuttingDown bool
}

// New creates a new instance of a Mesh.
func New(args ...string) *Mesh {
	name := "Service Mesh"

	if len(args) > 0 {
		name = strings.Join(args, " ")
	}

	r := &Mesh{
		name:      name,
		events:    ee.New(),
		logOutput: os.Stdout,
		logLevel:  int(slog.LevelInfo),
	}

	// the service mesh itself is a service that binds handlers to its own events
	r.Add(r)

	return r
}

func (r *Mesh) Init(_ M) {
	if r.services != nil {
		return
	}

	r.logger = r.newLogger(r)

	r.logger.Info("initializing")

	r.quit = make(chan os.Signal, 1)
	signal.Notify(r.quit, os.Interrupt)

	r.services = make([]Service, 0)
}

// Add a single service to the Mesh.
func (r *Mesh) Add(service Service) *sync.WaitGroup {
	r.Init(nil) // always ensure service mesh is init
	r.bindEventHandlerInterfaces(service)

	var wg sync.WaitGroup

	if service != r {
		r.logger.Info("adding service to mesh", "service", service.Name())
	}

	// Check if the service uses a logger
	if loggerUser, ok := service.(HasLogger); ok {
		wg.Add(1)
		loggerUser.SetLogger(r.newLogger(service))
		r.events.Emit(EventServiceLoggerBound, service).Wait()
		wg.Done()
	}

	r.services = append(r.services, service)

	// Check if the service is a HasDependencies
	if resolver, ok := service.(HasDependencies); ok {
		wg.Add(1)
		// Resolve dependencies before initialization
		go func() {
			r.resolveDependenciesAndInit(resolver)
			r.events.Emit(EventServiceAdded, service)
			wg.Done()
		}()
	} else {
		wg.Add(1)
		// No dependencies to resolve, directly initialize the service
		go func() {
			r.initService(service)
			r.events.Emit(EventServiceAdded, service)
			wg.Done()
		}()
	}

	return &wg
}

func (r *Mesh) resolveDependenciesAndInit(resolver HasDependencies) {
	r.events.Emit(EventDependencyResolutionStarted, resolver)

	// Check if all dependencies are resolved
	for !resolver.DependenciesResolved() {
		resolver.ResolveDependencies(r)
		time.Sleep(time.Millisecond * 10)
	}

	r.events.Emit(EventDependencyResolutionEnded, resolver)

	// All dependencies resolved, initialize the service
	r.initService(resolver)
}

// initService initializes a service and adds it to the Mesh.
func (r *Mesh) initService(service Service) {
	if l, ok := service.(HasLogger); ok && l.Logger() != nil {
		l.Logger().Debug("initializing")
	} else {
		r.newLogger(service).Debug("initializing")
	}

	// Initialize the service
	service.Init(r)

	r.events.Emit(EventServiceInitialized, service)
}

// Services returns a pointer to a slice of interfaces representing the services managed by the Mesh.
func (r *Mesh) Services() []Service {
	duplicate := append([]Service{}, r.services...)
	return duplicate
}

// Remove a specific service from the Mesh.
func (r *Mesh) Remove(service Service) *sync.WaitGroup {
	wg := r.events.Emit(EventServiceRemoved)

	for i, svc := range r.services {
		if svc == service {
			r.logger.Info("removing service", "service", service.Name())
			r.services = append(r.services[:i], r.services[i+1:]...)
			break
		}
	}

	return wg
}

// Shutdown sends an interrupt signal to the Mesh, indicating it should exit.
func (r *Mesh) Shutdown() *sync.WaitGroup {
	if r.shuttingDown {
		return &sync.WaitGroup{}
	}

	r.quit <- syscall.SIGINT
	r.shuttingDown = true

	wg := r.events.Emit(EventRuntimeShutdownInitiated)

	for _, service := range r.services {
		if quitter, ok := service.(HasGracefulShutdown); ok {

			if l, ok := quitter.(HasLogger); ok && l.Logger() != nil {
				l.Logger().Info("shutting down")
			} else {
				r.logger.Info("shutting down service", "service", service.Name())
			}

			quitter.OnShutdown()
		}
	}

	r.logger.Info("exiting")

	return wg
}

// Name returns the name of the Mesh.
func (r *Mesh) Name() string {
	return r.name
}

// Run starts the Mesh and waits for an interrupt signal to exit.
func (r *Mesh) Run() {
	r.events.Emit(EventRuntimeRunLoopInitiated)

	<-r.quit              // blocks until signal is recieved
	fmt.Printf("\033[2D") // Remove ^C from stdout

	r.Shutdown().Wait()
	time.Sleep(time.Second)
}

// Events yields the global event bus for the service mesh
func (r *Mesh) Events() *ee.EventEmitter {
	return r.events
}

func (r *Mesh) bindEventHandlerInterfaces(service Service) {
	if handler, ok := service.(EventHandlerServiceAdded); ok {
		if service != r {
			r.logger.Info("bound 'EventServiceAdded' event handler", "service", service.Name())
		}
		r.Events().On(EventServiceAdded, handler.OnServiceAdded)
	}

	if handler, ok := service.(EventHandlerServiceRemoved); ok {
		if service != r {
			r.logger.Info("bound 'EventServiceRemoved' event handler", "service", service.Name())
		}
		r.Events().On(EventServiceRemoved, handler.OnServiceRemoved)
	}

	if handler, ok := service.(EventHandlerServiceInitialized); ok {
		if service != r {
			r.logger.Info("bound 'EventServiceInitialized' event handler", "service", service.Name())
		}
		r.Events().On(EventServiceInitialized, handler.OnServiceInitialized)
	}

	if handler, ok := service.(EventHandlerServiceEventsBound); ok {
		if service != r {
			r.logger.Info("bound 'EventServiceEventsBound' event handler", "service", service.Name())
		}
		r.Events().On(EventServiceEventsBound, handler.OnServiceEventsBound)
	}

	if handler, ok := service.(EventHandlerServiceLoggerBound); ok {
		if service != r {
			r.logger.Info("bound 'EventServiceLoggerBound' event handler", "service", service.Name())
		}
		r.Events().On(EventServiceLoggerBound, handler.OnServiceLoggerBound)
	}

	if handler, ok := service.(EventHandlerRuntimeRunLoopInitiated); ok {
		if service != r {
			r.logger.Info("bound 'EventRuntimeRunLoopInitiated' event handler", "service", service.Name())
		}
		r.Events().On(EventRuntimeRunLoopInitiated, handler.OnRuntimeRunLoopInitiated)
	}

	if handler, ok := service.(EventHandlerRuntimeShutdownInitiated); ok {
		if service != r {
			r.logger.Info("bound 'EventRuntimeShutdownInitiated' event handler", "service", service.Name())
		}
		r.Events().On(EventRuntimeShutdownInitiated, handler.OnRuntimeShutdownInitiated)
	}

	if handler, ok := service.(EventHandlerDependencyResolutionStarted); ok {
		if service != r {
			r.logger.Info("bound 'EventDependencyResolutionStarted' event handler", "service", service.Name())
		}
		r.Events().On(EventDependencyResolutionStarted, handler.OnDependencyResolutionStarted)
	}

	if handler, ok := service.(EventHandlerDependencyResolutionEnded); ok {
		if service != r {
			r.logger.Info("bound 'EventDependencyResolutionEnded' event handler", "service", service.Name())
		}
		r.Events().On(EventDependencyResolutionEnded, handler.OnDependencyResolutionEnded)
	}
}

func (r *Mesh) OnServiceAdded(args ...any) {
	if len(args) < 1 {
		return
	}

	if service, ok := args[0].(Service); ok {
		if service != r {
			r.logger.Info("service added", "service", service.Name())
		}
	}
}

func (r *Mesh) OnRuntimeShutdownInitiated(_ ...any) {
	r.logger.Warn("initiating graceful shutdown")
}

func (r *Mesh) OnServiceRemoved(args ...any) {
	if len(args) < 1 {
		return
	}

	if service, ok := args[0].(Service); ok {
		r.logger.Debug("removed service", "service", service.Name())
	}
}

func (r *Mesh) OnServiceInitialized(args ...any) {
	if len(args) < 1 {
		return
	}

	if service, ok := args[0].(Service); ok {
		r.logger.Debug("service initialized", "service", service.Name())
	}
}

func (r *Mesh) OnServiceEventsBound(args ...any) {
	if len(args) < 1 {
		return
	}

	if service, ok := args[0].(Service); ok {
		r.logger.Debug("events bound", "service", service.Name())
	}
}

func (r *Mesh) OnServiceLoggerBound(args ...any) {
	if len(args) < 1 {
		return
	}

	if service, ok := args[0].(Service); ok {
		r.logger.Debug("logger bound", "service", service.Name())
	}
}

func (r *Mesh) OnRuntimeRunLoopInitiated(_ ...any) {
	r.logger.Debug("run loop started")
}

func (r *Mesh) OnDependencyResolutionStarted(args ...any) {
	if len(args) < 1 {
		return
	}

	if service, ok := args[0].(Service); ok {
		r.logger.Debug("dependency resolution started", "service", service.Name())
	}
}

func (r *Mesh) OnDependencyResolutionEnded(args ...any) {
	if len(args) < 1 {
		return
	}

	if service, ok := args[0].(Service); ok {
		r.logger.Debug("dependency resolution completed", "service", service.Name())
	}
}
