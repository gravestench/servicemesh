package servicemesh

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAddInitializesService(t *testing.T) {
	m := New()
	s := &testService{name: "example"}

	m.Add(s).Wait()

	if !s.initialized.Load() {
		t.Fatal("service was not initialized")
	}
	if got := len(m.Services()); got != 2 { // the mesh registers itself
		t.Fatalf("Services() returned %d services, want 2", got)
	}
}

func TestRemoveEmitsRemovedService(t *testing.T) {
	m := New()
	observer := &removalObserver{testService: testService{name: "observer"}}
	target := &testService{name: "target"}
	m.Add(observer).Wait()
	m.Add(target).Wait()

	m.Remove(target).Wait()

	if observer.removed != target {
		t.Fatalf("removed event contained %v, want target service", observer.removed)
	}
	if got := len(m.Services()); got != 2 {
		t.Fatalf("Services() returned %d services after removal, want 2", got)
	}
}

func TestLogSettingsRebuildDefaultHandler(t *testing.T) {
	m := New()
	s := &loggingService{testService: testService{name: "logger"}}
	m.Add(s).Wait()

	var dst bytes.Buffer
	m.SetLogDestination(&dst)
	m.SetLogLevel(slog.LevelDebug)
	s.Logger().Debug("debug is enabled")

	if got := dst.String(); !strings.Contains(got, "debug is enabled") {
		t.Fatalf("updated logger did not write a debug message to destination: %q", got)
	}
}

func TestShutdownCancelsDependencyResolution(t *testing.T) {
	m := New()
	s := &unresolvedService{testService: testService{name: "unresolved"}}
	wg := m.Add(s)

	m.Shutdown().Wait()
	completed := make(chan struct{})
	go func() {
		wg.Wait()
		close(completed)
	}()

	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("dependency resolution did not stop during shutdown")
	}
	if s.initialized.Load() {
		t.Fatal("service initialized despite unresolved dependencies")
	}
}

func TestConcurrentServiceAccess(t *testing.T) {
	m := New()
	const count = 25
	services := make([]*testService, count)
	var wg sync.WaitGroup
	for i := range services {
		services[i] = &testService{name: "concurrent"}
		wg.Add(1)
		go func(s *testService) {
			defer wg.Done()
			m.Add(s).Wait()
			_ = m.Services()
			m.Remove(s).Wait()
		}(services[i])
	}
	wg.Wait()

	if got := len(m.Services()); got != 1 {
		t.Fatalf("Services() returned %d services, want only the mesh", got)
	}
}

type testService struct {
	name        string
	initialized atomic.Bool
}

func (s *testService) Init(Mesh)    { s.initialized.Store(true) }
func (s *testService) Name() string { return s.name }

type removalObserver struct {
	testService
	removed Service
}

func (s *removalObserver) OnServiceRemoved(service Service) { s.removed = service }

type loggingService struct {
	testService
	mu     sync.RWMutex
	logger *slog.Logger
}

func (s *loggingService) SetLogger(logger *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
}

func (s *loggingService) Logger() *slog.Logger {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logger
}

type unresolvedService struct{ testService }

func (s *unresolvedService) DependenciesResolved() bool    { return false }
func (s *unresolvedService) ResolveDependencies([]Service) {}
