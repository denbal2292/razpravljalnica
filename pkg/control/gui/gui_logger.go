package gui

import (
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/hashicorp/go-hclog"
)

// SlogWriter wraps slog.Logger to implement io.Writer for Raft transport.
type SlogWriter struct {
	logger *slog.Logger
}

func (w *SlogWriter) Write(p []byte) (n int, err error) {
	w.logger.Info(string(p))
	return len(p), nil
}

// slogAdapter adapts slog.Logger to hclog.Logger interface for Raft.
type slogAdapter struct {
	logger *slog.Logger
	name   string
}

func NewSlogAdapter(logger *slog.Logger, name string) hclog.Logger {
	return &slogAdapter{logger: logger, name: name}
}

func NewSlogWriter(logger *slog.Logger) *SlogWriter {
	return &SlogWriter{logger: logger}
}

func (a *slogAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Trace, hclog.Debug:
		a.logger.Debug(msg, args...)
	case hclog.Info:
		a.logger.Info(msg, args...)
	case hclog.Warn:
		a.logger.Warn(msg, args...)
	case hclog.Error:
		a.logger.Error(msg, args...)
	}
}

func (a *slogAdapter) Trace(msg string, args ...interface{}) { a.logger.Debug(msg, args...) }
func (a *slogAdapter) Debug(msg string, args ...interface{}) { a.logger.Debug(msg, args...) }
func (a *slogAdapter) Info(msg string, args ...interface{})  { a.logger.Info(msg, args...) }
func (a *slogAdapter) Warn(msg string, args ...interface{})  { a.logger.Warn(msg, args...) }
func (a *slogAdapter) Error(msg string, args ...interface{}) { a.logger.Error(msg, args...) }

func (a *slogAdapter) IsTrace() bool { return true }
func (a *slogAdapter) IsDebug() bool { return true }
func (a *slogAdapter) IsInfo() bool  { return true }
func (a *slogAdapter) IsWarn() bool  { return true }
func (a *slogAdapter) IsError() bool { return true }

func (a *slogAdapter) ImpliedArgs() []interface{}            { return nil }
func (a *slogAdapter) With(args ...interface{}) hclog.Logger { return a }
func (a *slogAdapter) Name() string                          { return a.name }
func (a *slogAdapter) Named(name string) hclog.Logger        { return NewSlogAdapter(a.logger, name) }
func (a *slogAdapter) ResetNamed(name string) hclog.Logger   { return NewSlogAdapter(a.logger, name) }
func (a *slogAdapter) SetLevel(level hclog.Level)            {}
func (a *slogAdapter) GetLevel() hclog.Level                 { return hclog.Debug }
func (a *slogAdapter) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}
func (a *slogAdapter) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return os.Stderr
}
