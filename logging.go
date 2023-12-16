package servicemesh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// newLogger is a factory function that generates a slog instance
func (r *Mesh) newLogger(service Service) *slog.Logger {
	name := service.Name()

	opts := &slog.HandlerOptions{
		Level: slog.Level(r.logLevel),
	}

	if r.logOutput == nil {
		r.logOutput = os.Stdout
	}

	handler := slog.NewTextHandler(r.logOutput, opts) // or NewJSONHandler for JSON output

	logger := slog.New(handler)

	if service != r {
		logger = logger.With(slog.String("service", name))
	}

	return logger
}

func (r *Mesh) SetLogLevel(level int) { // Change level type as appropriate
	r.logLevel = level
	r.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf("setting log level to %d", level))
	r.logger = r.newLogger(r)

	r.updateServiceLoggers()
}

func (r *Mesh) SetLogDestination(dst io.Writer) {
	r.logOutput = dst

	newLogger := r.newLogger(r)
	r.logger = newLogger

	r.updateServiceLoggers()
}

func (r *Mesh) updateServiceLoggers() {
	// set the log level for each service that has a logger
	for _, service := range r.Services() {
		candidate, ok := service.(HasLogger)
		if !ok {
			continue
		}

		candidate.SetLogger(r.newLogger(candidate))
	}
}
