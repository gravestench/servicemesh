package servicemesh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// newLogger is a factory function that generates a slog instance for a service.
func (m *mesh) newLogger(service Service) *slog.Logger {
	name := service.Name()

	opts := &slog.HandlerOptions{
		Level: m.logLevel,
	}

	if m.logOutput == nil {
		m.logOutput = os.Stdout
	}

	if m.logHandler == nil {
		m.logHandler = slog.NewTextHandler(m.logOutput, opts) // or NewJSONHandler for JSON output
	}

	logger := slog.New(m.logHandler)

	if service != m {
		logger = logger.With(slog.String("service", name))
	}

	return logger
}

// SetLogHandler sets the slog log handler interface for the service mesh and
// all existing services, as well as any services added in the future.
func (m *mesh) SetLogHandler(handler slog.Handler) {
	m.logHandler = handler
	m.logger = m.newLogger(m)

	m.updateServiceLoggers()
}

// SetLogLevel sets the slog logger log level for the service mesh and
// all existing services, as well as any services added in the future.
func (m *mesh) SetLogLevel(level slog.Level) { // Change level type as appropriate
	m.logLevel = level
	m.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf("setting log level to %d", level))
	m.logger = m.newLogger(m)

	m.updateServiceLoggers()
}

// SetLogDestination sets the slog logger destination for the service mesh and
// all existing services, as well as any services added in the future.
func (m *mesh) SetLogDestination(dst io.Writer) {
	m.logOutput = dst

	newLogger := m.newLogger(m)
	m.logger = newLogger

	m.updateServiceLoggers()
}

func (m *mesh) updateServiceLoggers() {
	// set the log level for each service that has a logger
	for _, service := range m.Services() {
		candidate, ok := service.(HasLogger)
		if !ok {
			continue
		}

		candidate.SetLogger(m.newLogger(candidate))
	}
}
