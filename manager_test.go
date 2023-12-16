package servicemesh

import (
	"log/slog"
	"testing"
	"time"
)

func TestRuntime(t *testing.T) {
	rt := New()

	go func() {
		time.Sleep(time.Second * 3)
		rt.Shutdown().Wait()
	}()

	rt.Add(&exampleService{})

	rt.Run()
}

type exampleService struct {
	logger *slog.Logger
}

func (e *exampleService) SetLogger(logger *slog.Logger) {
	e.logger = logger
}

func (e *exampleService) Logger() *slog.Logger {
	return e.logger
}

func (e *exampleService) Init(rt Mesh) {

}

func (e *exampleService) Name() string {
	return "example"
}

func (e *exampleService) OnShutdown() {
	time.Sleep(time.Second * 3)
	e.logger.Info("graceful shutdown completed")
}
