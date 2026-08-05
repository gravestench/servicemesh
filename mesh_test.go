package servicemesh

import (
	"bytes"
	"errors"
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
	observer := newDependencyFailureObserver()
	s := &unresolvedService{testService: testService{name: "unresolved"}}
	m.Add(observer).Wait()
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
	assertDependencyFailure(t, observer, s, ErrDependencyResolutionCanceled)
}

func TestDependencyResolutionTimeout(t *testing.T) {
	m := New()
	m.SetDependencyResolutionTimeout(20 * time.Millisecond)
	observer := newDependencyFailureObserver()
	s := &unresolvedService{testService: testService{name: "unresolved"}}
	m.Add(observer).Wait()

	m.Add(s).Wait()

	if s.initialized.Load() {
		t.Fatal("service initialized despite unresolved dependencies")
	}
	assertDependencyFailure(t, observer, s, ErrDependencyResolutionTimeout)
}

func TestConcurrentShutdownSharesCompletion(t *testing.T) {
	m := New()
	s := &blockingShutdownService{
		testService: testService{name: "blocking"},
		started:     make(chan struct{}),
		release:     make(chan struct{}),
	}
	m.Add(s).Wait()

	firstResult := make(chan *sync.WaitGroup, 1)
	go func() { firstResult <- m.Shutdown() }()
	<-s.started

	second := m.Shutdown()
	secondDone := make(chan struct{})
	go func() {
		second.Wait()
		close(secondDone)
	}()
	select {
	case <-secondDone:
		t.Fatal("concurrent shutdown completed before graceful shutdown")
	case <-time.After(20 * time.Millisecond):
	}

	close(s.release)
	first := <-firstResult
	second.Wait()
	if first != second {
		t.Fatal("shutdown callers received different completion handles")
	}
	if got := s.shutdownCalls.Load(); got != 1 {
		t.Fatalf("OnShutdown called %d times, want 1", got)
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

type dependencyFailure struct {
	service Service
	err     error
}

type dependencyFailureObserver struct {
	testService
	failures chan dependencyFailure
}

func newDependencyFailureObserver() *dependencyFailureObserver {
	return &dependencyFailureObserver{
		testService: testService{name: "failure observer"},
		failures:    make(chan dependencyFailure, 1),
	}
}

func (s *dependencyFailureObserver) OnDependencyResolutionFailed(service Service, err error) {
	s.failures <- dependencyFailure{service: service, err: err}
}

func assertDependencyFailure(t *testing.T, observer *dependencyFailureObserver, service Service, want error) {
	t.Helper()
	select {
	case failure := <-observer.failures:
		if failure.service != service {
			t.Fatalf("failure service = %v, want %v", failure.service, service)
		}
		if !errors.Is(failure.err, want) {
			t.Fatalf("failure error = %v, want %v", failure.err, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for dependency failure %v", want)
	}
}

type blockingShutdownService struct {
	testService
	started       chan struct{}
	release       chan struct{}
	shutdownCalls atomic.Int32
}

func (s *blockingShutdownService) OnShutdown() {
	s.shutdownCalls.Add(1)
	close(s.started)
	<-s.release
}
