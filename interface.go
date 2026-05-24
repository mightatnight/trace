package trace

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogHook is a callback invoked when a log entry is emitted. It receives the
// level of the entry, its message, and the structured fields that were logged.
//
// The fields slice must not be retained or mutated after the hook returns:
// for synchronous hooks it aliases the caller's slice, and for goroutine hooks
// it is a copy that is safe only for the lifetime of that goroutine.
type LogHook func(level zapcore.Level, msg string, fields []zap.Field)

// Logger defines the logging methods
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)
	// With returns a child logger with additional structured fields included in every log.
	// Registered log hooks are carried over to the child.
	With(fields ...zap.Field) Logger
	// Named returns a child logger with a name scope (logger name prefix).
	// Registered log hooks are carried over to the child.
	Named(name string) Logger
	// OnLog returns a child logger that runs hook synchronously, in the calling
	// goroutine, whenever an entry is emitted. The hook fires only for entries
	// that pass the configured level. Hooks propagate to With/Named children.
	OnLog(hook LogHook) Logger
	// OnLogAsync returns a child logger that runs hook in a separate goroutine
	// whenever an entry is emitted. Behaves like OnLog otherwise.
	// Note: async hooks on Fatal may not complete before the process exits.
	OnLogAsync(hook LogHook) Logger
	// Zap returns the underlying zap.Logger.
	Zap() *zap.Logger
}

// NoopLogger implements LoggerInterface with no-op operations
type NoopLogger struct{}

// NewNoopLogger creates a no-op logger that safely discards all log messages
func NewNoopLogger() Logger {
	return &NoopLogger{}
}

// NoopLogger implementation methods

func (n *NoopLogger) Debug(msg string, fields ...zap.Field) {}
func (n *NoopLogger) Info(msg string, fields ...zap.Field)  {}
func (n *NoopLogger) Warn(msg string, fields ...zap.Field)  {}
func (n *NoopLogger) Error(msg string, fields ...zap.Field) {}
func (n *NoopLogger) Fatal(msg string, fields ...zap.Field) {}
func (n *NoopLogger) With(fields ...zap.Field) Logger       { return n }
func (n *NoopLogger) Named(name string) Logger              { return n }
func (n *NoopLogger) OnLog(hook LogHook) Logger             { return n }
func (n *NoopLogger) OnLogAsync(hook LogHook) Logger        { return n }
func (n *NoopLogger) Zap() *zap.Logger                      { return zap.NewNop() }
