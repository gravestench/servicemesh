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

var _ Mesh = &mesh{}

// mesh represents a collection of service mesh services.
type mesh struct {
	name         string
	quit         chan os.Signal
	services     []Service
	logger       *slog.Logger
	logOutput    io.Writer
	logLevel     int
	logHandler   slog.Handler
	events       *ee.EventEmitter
	shuttingDown bool
}

// New creates a new instance of a mesh.
func New(args ...string) Mesh {
	name := "Service Mesh"

	if len(args) > 0 {
		name = strings.Join(args, " ")
	}

	r := &mesh{
		name:      name,
		events:    ee.New(),
		logOutput: os.Stdout,
		logLevel:  int(slog.LevelInfo),
	}

	// the service mesh itself is a service that binds handlers to its own events
	r.Add(r)

	return r
}

func (m *mesh) Init(_ Mesh) {
	if m.services != nil {
		return
	}

	m.logger = m.newLogger(m)

	m.logger.Info("initializing")

	m.quit = make(chan os.Signal, 1)
	signal.Notify(m.quit, os.Interrupt)

	m.services = make([]Service, 0)
}

// Add a single service to the mesh.
func (m *mesh) Add(service Service) *sync.WaitGroup {
	m.Init(nil) // always ensure service mesh is init
	m.bindEventHandlerInterfaces(service)

	var wg sync.WaitGroup

	if service != m {
		m.logger.Info("preparing service", "service", service.Name())
	}

	// Check if the service uses a logger
	if loggerUser, ok := service.(HasLogger); ok {
		wg.Add(1)
		loggerUser.SetLogger(m.newLogger(service))
		m.events.Emit(EventServiceLoggerBound, service).Wait()
		wg.Done()
	}

	m.services = append(m.services, service)

	// Check if the service is a HasDependencies
	if resolver, ok := service.(HasDependencies); ok {
		wg.Add(1)
		// Resolve dependencies before initialization
		go func() {
			m.resolveDependenciesAndInit(resolver)
			m.events.Emit(EventServiceAdded, service)
			wg.Done()
		}()
	} else {
		wg.Add(1)
		// No dependencies to resolve, directly initialize the service
		go func() {
			m.initService(service)
			m.events.Emit(EventServiceAdded, service)
			wg.Done()
		}()
	}

	return &wg
}

func (m *mesh) resolveDependenciesAndInit(resolver HasDependencies) {
	m.events.Emit(EventDependencyResolutionStarted, resolver)

	// Check if all dependencies are resolved
	for !resolver.DependenciesResolved() {
		resolver.ResolveDependencies(m)
		time.Sleep(time.Millisecond * 10)
	}

	m.events.Emit(EventDependencyResolutionEnded, resolver)

	// All dependencies resolved, initialize the service
	m.initService(resolver)
}

// initService initializes a service and adds it to the mesh.
func (m *mesh) initService(service Service) {
	if l, ok := service.(HasLogger); ok && l.Logger() != nil {
		l.Logger().Debug("initializing")
	} else {
		m.newLogger(service).Debug("initializing")
	}

	// Initialize the service
	service.Init(m)

	m.events.Emit(EventServiceInitialized, service)
}

// Services returns a pointer to a slice of interfaces representing the services managed by the mesh.
func (m *mesh) Services() []Service {
	duplicate := append([]Service{}, m.services...)
	return duplicate
}

// Remove a specific service from the mesh.
func (m *mesh) Remove(service Service) *sync.WaitGroup {
	wg := m.events.Emit(EventServiceRemoved)

	for i, svc := range m.services {
		if svc == service {
			m.logger.Info("removing service", "service", service.Name())
			m.services = append(m.services[:i], m.services[i+1:]...)
			break
		}
	}

	return wg
}

// Shutdown sends an interrupt signal to the mesh, indicating it should exit.
func (m *mesh) Shutdown() *sync.WaitGroup {
	if m.shuttingDown {
		return &sync.WaitGroup{}
	}

	m.quit <- syscall.SIGINT
	m.shuttingDown = true

	wg := m.events.Emit(EventServiceMeshShutdownInitiated)

	for _, service := range m.services {
		if quitter, ok := service.(HasGracefulShutdown); ok {

			if l, ok := quitter.(HasLogger); ok && l.Logger() != nil {
				l.Logger().Info("shutting down")
			} else {
				m.logger.Info("shutting down service", "service", service.Name())
			}

			quitter.OnShutdown()
		}
	}

	m.logger.Info("exiting")

	return wg
}

// Name returns the name of the mesh.
func (m *mesh) Name() string {
	return m.name
}

// Run starts the mesh and waits for an interrupt signal to exit.
func (m *mesh) Run() {
	m.events.Emit(EventServiceMeshRunLoopInitiated)

	<-m.quit              // blocks until signal is recieved
	fmt.Printf("\033[2D") // Remove ^C from stdout

	m.Shutdown().Wait()
	time.Sleep(time.Second)
}

// Events yields the global event bus for the service mesh
func (m *mesh) Events() *ee.EventEmitter {
	return m.events
}

func (m *mesh) bindEventHandlerInterfaces(service Service) {
	if handler, ok := service.(EventHandlerServiceAdded); ok {
		if service != m {
			m.logger.Debug("bound 'EventServiceAdded' event handler", "service", service.Name())
		}

		m.Events().On(EventServiceAdded, func(args ...any) {
			if len(args) < 1 {
				return
			}

			if serviceArg, ok := args[0].(Service); ok {
				handler.OnServiceAdded(serviceArg)
			}
		})
	}

	if handler, ok := service.(EventHandlerServiceRemoved); ok {
		if service != m {
			m.logger.Debug("bound 'EventServiceRemoved' event handler", "service", service.Name())
		}
		m.Events().On(EventServiceRemoved, func(args ...any) {
			if len(args) < 1 {
				return
			}

			if serviceArg, ok := args[0].(Service); ok {
				handler.OnServiceRemoved(serviceArg)
			}
		})
	}

	if handler, ok := service.(EventHandlerServiceInitialized); ok {
		if service != m {
			m.logger.Debug("bound 'EventServiceInitialized' event handler", "service", service.Name())
		}
		m.Events().On(EventServiceInitialized, func(args ...any) {
			if len(args) < 1 {
				return
			}

			if serviceArg, ok := args[0].(Service); ok {
				handler.OnServiceInitialized(serviceArg)
			}
		})
	}

	if handler, ok := service.(EventHandlerServiceEventsBound); ok {
		if service != m {
			m.logger.Debug("bound 'EventServiceEventsBound' event handler", "service", service.Name())
		}
		m.Events().On(EventServiceEventsBound, func(args ...any) {
			if len(args) < 1 {
				return
			}

			if serviceArg, ok := args[0].(Service); ok {
				handler.OnServiceEventsBound(serviceArg)
			}
		})
	}

	if handler, ok := service.(EventHandlerServiceLoggerBound); ok {
		if service != m {
			m.logger.Debug("bound 'EventServiceLoggerBound' event handler", "service", service.Name())
		}
		m.Events().On(EventServiceLoggerBound, func(args ...any) {
			if len(args) < 1 {
				return
			}

			if serviceArg, ok := args[0].(Service); ok {
				handler.OnServiceLoggerBound(serviceArg)
			}
		})
	}

	if handler, ok := service.(EventHandlerServiceMeshRunLoopInitiated); ok {
		if service != m {
			m.logger.Debug("bound 'EventServiceMeshRunLoopInitiated' event handler", "service", service.Name())
		}
		m.Events().On(EventServiceMeshRunLoopInitiated, func(_ ...any) {
			handler.OnServiceMeshRunLoopInitiated()
		})
	}

	if handler, ok := service.(EventHandlerServiceMeshShutdownInitiated); ok {
		if service != m {
			m.logger.Debug("bound 'EventServiceMeshShutdownInitiated' event handler", "service", service.Name())
		}
		m.Events().On(EventServiceMeshShutdownInitiated, func(_ ...any) {
			handler.OnServiceMeshShutdownInitiated()
		})
	}

	if handler, ok := service.(EventHandlerDependencyResolutionStarted); ok {
		if service != m {
			m.logger.Debug("bound 'EventDependencyResolutionStarted' event handler", "service", service.Name())
		}
		m.Events().On(EventDependencyResolutionStarted, func(args ...any) {
			if len(args) < 1 {
				return
			}

			if serviceArg, ok := args[0].(Service); ok {
				handler.OnDependencyResolutionStarted(serviceArg)
			}
		})
	}

	if handler, ok := service.(EventHandlerDependencyResolutionEnded); ok {
		if service != m {
			m.logger.Debug("bound 'EventDependencyResolutionEnded' event handler", "service", service.Name())
		}
		m.Events().On(EventDependencyResolutionEnded, func(args ...any) {
			if len(args) < 1 {
				return
			}

			if serviceArg, ok := args[0].(Service); ok {
				handler.OnDependencyResolutionEnded(serviceArg)
			}
		})
	}
}

func (m *mesh) OnServiceAdded(service Service) {
	if service == m {
		return
	}

	m.logger.Info("service added", "service", service.Name())
}

func (m *mesh) OnServiceMeshShutdownInitiated() {
	m.logger.Warn("initiating graceful shutdown")
}

func (m *mesh) OnServiceRemoved(service Service) {
	m.logger.Debug("removed service", "service", service.Name())
}

func (m *mesh) OnServiceInitialized(service Service) {
	m.logger.Debug("service initialized", "service", service.Name())
}

func (m *mesh) OnServiceEventsBound(service Service) {
	m.logger.Debug("events bound", "service", service.Name())
}

func (m *mesh) OnServiceLoggerBound(service Service) {
	m.logger.Debug("logger bound", "service", service.Name())
}

func (m *mesh) OnServiceMeshRunLoopInitiated() {
	m.logger.Debug("run loop started")
}

func (m *mesh) OnDependencyResolutionStarted(service Service) {
	m.logger.Debug("dependency resolution started", "service", service.Name())
}

func (m *mesh) OnDependencyResolutionEnded(service Service) {
	m.logger.Debug("dependency resolution completed", "service", service.Name())
}
