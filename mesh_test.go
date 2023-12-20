package servicemesh

import (
	"log/slog"
	"testing"
	"time"
)

func TestRuntime(t *testing.T) {
	m := New()
	s := &exampleService{}

	go func() {
		time.Sleep(time.Second * 3)
		s.ready = true
		time.Sleep(time.Second * 3)
		m.Shutdown().Wait()
	}()

	m.Add(s)

	m.Run()
}

type exampleService struct {
	logger *slog.Logger
	ready  bool
}

func (e *exampleService) SetLogger(logger *slog.Logger) {
	e.logger = logger
}

func (e *exampleService) Logger() *slog.Logger {
	return e.logger
}

func (e *exampleService) Init(_ Mesh) {
	// noop
}

func (e *exampleService) Name() string {
	return "example"
}

func (e *exampleService) Ready() bool {
	return e.ready
}

func (e *exampleService) OnShutdown() {
	time.Sleep(time.Second * 3)
	e.logger.Info("graceful shutdown completed")
}
