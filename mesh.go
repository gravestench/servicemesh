package servicemesh

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	ee "github.com/gravestench/eventemitter"
)

const (
	dependencyResolutionDwellDuration = 10 * time.Millisecond
	dependencyResolutionTimeout       = 30 * time.Second
)

// New creates a new instance of a service mesh. Optionally, strings can be
// supplied as arguments which are concatenated to form the name of the service
// mesh during logging.
func New(args ...string) Mesh {
	name := "baseService Mesh"

	if len(args) > 0 {
		name = strings.Join(args, " ")
	}

	r := &mesh{
		name:      name,
		events:    ee.New(),
		logOutput: os.Stdout,
		logLevel:  slog.LevelInfo,
	}
	r.Init(nil)

	// the service mesh itself is a service
	// that binds handlers to its own events
	r.Add(r).Wait()

	return r
}

var _ Mesh = &mesh{}

// mesh represents a collection of service mesh services.
type mesh struct {
	initOnce     sync.Once
	mu           sync.RWMutex
	logMu        sync.Mutex
	name         string
	quit         chan os.Signal
	ctx          context.Context
	cancel       context.CancelFunc
	services     []Service
	logger       *slog.Logger
	logOutput    io.Writer
	logLevel     slog.Level
	logHandler   slog.Handler
	events       *ee.EventEmitter
	shuttingDown bool
}

func (m *mesh) Init(_ Mesh) {
	m.initOnce.Do(func() {
		if m.events == nil {
			m.events = ee.New()
		}
		m.ctx, m.cancel = context.WithCancel(context.Background())
		m.quit = make(chan os.Signal, 1)
		logger := m.newLogger(m)
		m.logMu.Lock()
		m.logger = logger
		m.logMu.Unlock()
		logger.Debug("initializing")
		signal.Notify(m.quit, os.Interrupt)
	})
}

// Add a single service to the mesh.
func (m *mesh) Add(service Service) *sync.WaitGroup {
	m.Init(nil) // always ensure service mesh is init
	if service == nil {
		return &sync.WaitGroup{}
	}

	defer func() {
		m.bindEventHandlerInterfaces(service)
		m.events.Emit(EventServiceEventsBound, service).Wait()
	}()

	var wg sync.WaitGroup

	if service != m {
		m.meshLogger().Debug("preparing service", "service", service.Name())
	}

	// Check if the service uses a logger
	if candidate, ok := service.(HasLogger); ok {
		wg.Add(1)
		candidate.SetLogger(m.newLogger(service))
		m.events.Emit(EventServiceLoggerBound, service).Wait()
		wg.Done()
	}

	m.mu.Lock()
	m.services = append(m.services, service)
	m.mu.Unlock()
	m.events.Emit(EventServiceAdded, service).Wait()

	// Check if the service is a HasDependencies
	if resolver, ok := service.(HasDependencies); ok {
		// Resolve dependencies before initialization
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.resolveDependenciesAndInit(resolver)
		}()
	} else {
		// No dependencies to resolve, directly initialize the service
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.initService(service)
		}()
	}

	return &wg
}

func (m *mesh) resolveDependenciesAndInit(resolver HasDependencies) {
	m.events.Emit(EventDependencyResolutionStarted, resolver).Wait()
	ticker := time.NewTicker(dependencyResolutionDwellDuration)
	defer ticker.Stop()
	timer := time.NewTimer(dependencyResolutionTimeout)
	defer timer.Stop()

	for {
		if resolver.DependenciesResolved() {
			break
		}
		resolver.ResolveDependencies(m.Services())
		select {
		case <-m.ctx.Done():
			m.meshLogger().Warn("dependency resolution canceled", "service", resolver.Name())
			return
		case <-timer.C:
			m.meshLogger().Error("dependency resolution timed out", "service", resolver.Name(), "timeout", dependencyResolutionTimeout)
			return
		case <-ticker.C:
		}
	}

	m.events.Emit(EventDependencyResolutionEnded, resolver).Wait()

	m.initService(resolver)
}

// initService initializes a service after being added to the mesh.
func (m *mesh) initService(service Service) {
	if l, ok := service.(HasLogger); ok && l.Logger() != nil {
		l.Logger().Debug("initializing")
	} else {
		m.newLogger(service).Debug("initializing")
	}

	service.Init(m)

	m.events.Emit(EventServiceInitialized, service).Wait()
}

// Services returns a snapshot of the services managed by the mesh.
func (m *mesh) Services() (list []Service) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append(list, m.services...)
}

// Remove a specific service from the mesh.
func (m *mesh) Remove(service Service) *sync.WaitGroup {
	m.mu.Lock()
	removed := false
	for i, svc := range m.services {
		if svc == service {
			m.meshLogger().Debug("removing service", "service", service.Name())
			m.services = append(m.services[:i], m.services[i+1:]...)
			removed = true
			break
		}
	}
	m.mu.Unlock()

	if !removed {
		return &sync.WaitGroup{}
	}
	return m.events.Emit(EventServiceRemoved, service)
}

// Shutdown sends an interrupt signal to the mesh, indicating it should exit.
func (m *mesh) Shutdown() *sync.WaitGroup {
	m.mu.Lock()
	if m.shuttingDown {
		m.mu.Unlock()
		return &sync.WaitGroup{}
	}
	m.shuttingDown = true
	services := append([]Service(nil), m.services...)
	m.mu.Unlock()

	m.cancel()
	signal.Stop(m.quit)
	select {
	case m.quit <- os.Interrupt:
	default:
	}

	// we will give all shutdown event handlers a chance to respond
	wg := m.events.Emit(EventServiceMeshShutdownInitiated)

	for _, service := range services {
		if quitter, ok := service.(HasGracefulShutdown); ok {

			if l, ok := quitter.(HasLogger); ok && l.Logger() != nil {
				l.Logger().Debug("shutting down")
			} else {
				m.meshLogger().Debug("shutting down service", "service", service.Name())
			}

			quitter.OnShutdown()
		}
	}

	m.meshLogger().Warn("exiting")

	// allow the caller to wait for the event handlers to finish
	return wg
}

// Name returns the name of the mesh.
func (m *mesh) Name() string {
	return m.name
}

// Run starts the mesh and waits for an interrupt signal to exit.
func (m *mesh) Run() {
	m.events.Emit(EventServiceMeshRunLoopInitiated).Wait()

	<-m.quit
	m.Shutdown().Wait()
}

// Events yields the global event bus for the service mesh
func (m *mesh) Events() *ee.EventEmitter {
	m.Init(nil)
	return m.events
}

// bindEventHandlerInterfaces provides the syntactic sugar for services that
// want to bind event handlers to the event bus for specific service mesh
// events. These are just wrappers for binding callbacks with the event emitter.
// This allows other services to implement the event bus integration interfaces
// without needing to know how to use the event emitter.
func (m *mesh) bindEventHandlerInterfaces(service Service) {
	if handler, ok := service.(EventHandlerServiceAdded); ok {
		if service != m {
			m.meshLogger().Debug("bound 'EventServiceAdded' event handler", "service", service.Name())
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
			m.meshLogger().Debug("bound 'EventServiceRemoved' event handler", "service", service.Name())
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
			m.meshLogger().Debug("bound 'EventServiceInitialized' event handler", "service", service.Name())
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
			m.meshLogger().Debug("bound 'EventServiceEventsBound' event handler", "service", service.Name())
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
			m.meshLogger().Debug("bound 'EventServiceLoggerBound' event handler", "service", service.Name())
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
			m.meshLogger().Debug("bound 'EventServiceMeshRunLoopInitiated' event handler", "service", service.Name())
		}
		m.Events().On(EventServiceMeshRunLoopInitiated, func(_ ...any) {
			handler.OnServiceMeshRunLoopInitiated()
		})
	}

	if handler, ok := service.(EventHandlerServiceMeshShutdownInitiated); ok {
		if service != m {
			m.meshLogger().Debug("bound 'EventServiceMeshShutdownInitiated' event handler", "service", service.Name())
		}
		m.Events().On(EventServiceMeshShutdownInitiated, func(_ ...any) {
			handler.OnServiceMeshShutdownInitiated()
		})
	}

	if handler, ok := service.(EventHandlerDependencyResolutionStarted); ok {
		if service != m {
			m.meshLogger().Debug("bound 'EventDependencyResolutionStarted' event handler", "service", service.Name())
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
			m.meshLogger().Debug("bound 'EventDependencyResolutionEnded' event handler", "service", service.Name())
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

// The following methods implement the event handler integration interfaces
// found in interfaces.go. We dog-food our own event bus to log the various
// service mesh events.

func (m *mesh) OnServiceAdded(service Service) {
	if service != m {
		m.meshLogger().Debug("service added", "service", service.Name())
	}
}

func (m *mesh) OnServiceMeshShutdownInitiated() {
	m.meshLogger().Warn("initiating graceful shutdown")
}

func (m *mesh) OnServiceRemoved(service Service) {
	if service != m {
		m.meshLogger().Debug("removed service", "service", service.Name())
	}
}

func (m *mesh) OnServiceInitialized(service Service) {
	if service != m {
		m.meshLogger().Debug("service initialized", "service", service.Name())
	}
}

func (m *mesh) OnServiceEventsBound(service Service) {
	if service != m {
		m.meshLogger().Debug("events bound", "service", service.Name())
	}
}

func (m *mesh) OnServiceLoggerBound(service Service) {
	if service != m {
		m.meshLogger().Debug("logger bound", "service", service.Name())
	}
}

func (m *mesh) OnServiceMeshRunLoopInitiated() {
	m.meshLogger().Debug("run loop started")
}

func (m *mesh) OnDependencyResolutionStarted(service Service) {
	if service != m {
		m.meshLogger().Debug("dependency resolution started", "service", service.Name())
	}
}

func (m *mesh) OnDependencyResolutionEnded(service Service) {
	if service != m {
		m.meshLogger().Debug("dependency resolution completed", "service", service.Name())
	}
}
