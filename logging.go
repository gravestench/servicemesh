package servicemesh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// newLogger is a factory function that generates a slog instance
func (m *mesh) newLogger(service Service) *slog.Logger {
	name := service.Name()

	opts := &slog.HandlerOptions{
		Level: slog.Level(m.logLevel),
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

func (m *mesh) SetLogHandler(handler slog.Handler) { // Change level type as appropriate
	m.logHandler = handler
	m.logger = m.newLogger(m)

	m.updateServiceLoggers()
}

func (m *mesh) SetLogLevel(level int) { // Change level type as appropriate
	m.logLevel = level
	m.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf("setting log level to %d", level))
	m.logger = m.newLogger(m)

	m.updateServiceLoggers()
}

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
