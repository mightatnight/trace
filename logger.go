package trace

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ Logger = &sugarLogger{}

// sugarLogger implements the LoggerInterface with a real zap logger
type sugarLogger struct {
	Log *zap.Logger
	// onLog holds hooks run synchronously in the calling goroutine.
	onLog []LogHook
	// onLogAsync holds hooks run in a separate goroutine.
	onLogAsync []LogHook
}

// New creates the fastest possible logger configuration
// level: minimum log level (e.g., zapcore.InfoLevel)
// prefix: logger name prefix for all messages
// logFile: optional file to write logs to (pass nil to log to stdout only)
// To disable logging completely, use zapcore.Level(127)
func New(level zapcore.Level, prefix string, logFile *os.File) Logger {
	// Fastest possible encoder config
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "ts",
		NameKey:        "logger",
		CallerKey:      "",                                                 // disabled for speed
		StacktraceKey:  "",                                                 // disabled for speed
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,                   // colored level in caps
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"), // human readable time
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	// Create stdout writer
	stdoutSink := zapcore.Lock(os.Stdout)

	var core zapcore.Core

	// If logFile is provided, create a multi-output core
	if logFile != nil {
		// Create file sink
		fileSink := zapcore.Lock(logFile)

		// Create a core that writes to both stdout and file
		core = zapcore.NewTee(
			zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), stdoutSink, level),
			zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), fileSink, level),
		)
	} else {
		// Standard stdout-only core
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			stdoutSink,
			level,
		)
	}

	if prefix != "" {
		return &sugarLogger{
			Log: zap.New(core).Named(prefix),
		}
	}

	// Build the logger with minimal options for speed
	return &sugarLogger{
		Log: zap.New(core),
	}
}

func NewChildLogger(parent Logger, prefix string) Logger {
	if parent == nil || parent.Zap() == nil {
		return NewNoopLogger()
	}

	// Carry over any hooks registered on the parent (best effort: only the
	// concrete sugarLogger exposes them).
	child := &sugarLogger{Log: parent.Zap()}
	if sl, ok := parent.(*sugarLogger); ok {
		child.onLog = sl.onLog
		child.onLogAsync = sl.onLogAsync
	}

	if prefix != "" {
		child.Log = child.Log.Named(prefix)
	}

	return child
}

// Debug logs a debug message
func (l *sugarLogger) Debug(msg string, fields ...zap.Field) {
	if l.Log != nil {
		l.Log.Debug(msg, fields...)
		l.fire(DebugLevel, msg, fields)
	}
}

// Info logs an info message
func (l *sugarLogger) Info(msg string, fields ...zap.Field) {
	if l.Log != nil {
		l.Log.Info(msg, fields...)
		l.fire(InfoLevel, msg, fields)
	}
}

// Warn logs a warning message
func (l *sugarLogger) Warn(msg string, fields ...zap.Field) {
	if l.Log != nil {
		l.Log.Warn(msg, fields...)
		l.fire(WarnLevel, msg, fields)
	}
}

// Error logs an error message
func (l *sugarLogger) Error(msg string, fields ...zap.Field) {
	if l.Log != nil {
		l.Log.Error(msg, fields...)
		l.fire(ErrorLevel, msg, fields)
	}
}

// Fatal logs a fatal message and exits
func (l *sugarLogger) Fatal(msg string, fields ...zap.Field) {
	if l.Log != nil {
		// Fire hooks before the underlying call: zap.Fatal terminates the
		// process, so anything deferred until after would never run.
		l.fire(FatalLevel, msg, fields)
		l.Log.Fatal(msg, fields...)
	}
}

// fire invokes the registered hooks for an emitted entry. It is a no-op when
// no hooks are registered, and respects the configured level so hooks only run
// for entries that the underlying core would actually emit.
func (l *sugarLogger) fire(level zapcore.Level, msg string, fields []zap.Field) {
	if len(l.onLog) == 0 && len(l.onLogAsync) == 0 {
		return
	}
	if l.Log == nil || !l.Log.Core().Enabled(level) {
		return
	}

	for _, h := range l.onLog {
		h(level, msg, fields)
	}

	if len(l.onLogAsync) > 0 {
		// Copy fields: the caller's variadic backing array may be reused or
		// mutated once the log call returns, but the goroutine outlives it.
		cp := make([]zap.Field, len(fields))
		copy(cp, fields)
		for _, h := range l.onLogAsync {
			go h(level, msg, cp)
		}
	}
}

// With returns a child logger with additional structured fields included in every log.
// Registered hooks are carried over to the child.
func (l *sugarLogger) With(fields ...zap.Field) Logger {
	if l == nil || l.Log == nil {
		return l
	}
	return &sugarLogger{
		Log:        l.Log.With(fields...),
		onLog:      l.onLog,
		onLogAsync: l.onLogAsync,
	}
}

// Named returns a child logger with a name scope (logger name prefix).
// Registered hooks are carried over to the child.
func (l *sugarLogger) Named(name string) Logger {
	if l == nil || l.Log == nil {
		return l
	}
	return &sugarLogger{
		Log:        l.Log.Named(name),
		onLog:      l.onLog,
		onLogAsync: l.onLogAsync,
	}
}

// OnLog returns a child logger that runs hook synchronously whenever an entry
// is emitted. The returned logger must be used for the hook to take effect; the
// receiver is left unchanged. Hooks accumulate and propagate to With/Named children.
func (l *sugarLogger) OnLog(hook LogHook) Logger {
	if l == nil || l.Log == nil || hook == nil {
		return l
	}
	return &sugarLogger{
		Log:        l.Log,
		onLog:      appendHook(l.onLog, hook),
		onLogAsync: l.onLogAsync,
	}
}

// OnLogAsync returns a child logger that runs hook in a separate goroutine
// whenever an entry is emitted. See OnLog for usage; async hooks on Fatal
// may not complete before the process exits.
func (l *sugarLogger) OnLogAsync(hook LogHook) Logger {
	if l == nil || l.Log == nil || hook == nil {
		return l
	}
	return &sugarLogger{
		Log:        l.Log,
		onLog:      l.onLog,
		onLogAsync: appendHook(l.onLogAsync, hook),
	}
}

// appendHook returns a new slice with hook appended, copying existing so that
// slices shared with parent/child loggers are never mutated in place.
func appendHook(existing []LogHook, hook LogHook) []LogHook {
	out := make([]LogHook, len(existing)+1)
	copy(out, existing)
	out[len(existing)] = hook
	return out
}

// Zap returns the underlying zap logger if needed
func (l *sugarLogger) Zap() *zap.Logger {
	return l.Log
}

// Log level constants
var (
	DebugLevel = zapcore.DebugLevel
	InfoLevel  = zapcore.InfoLevel
	WarnLevel  = zapcore.WarnLevel
	ErrorLevel = zapcore.ErrorLevel
	FatalLevel = zapcore.FatalLevel
)

// DisabledLevel returns a level that disables all logging
func DisabledLevel() zapcore.Level {
	return zapcore.Level(127)
}

// Context scoping helpers

type loggerCtxKey struct{}

// LoggerToContext attaches the provided logger to the context.
func LoggerToContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// LoggerFromContext retrieves a logger from context or returns a no-op logger if absent.
// Typical usage:
//   - component-scoped: base := root.Named("http").With(zap.String("component","http"))
//   - request-scoped:  reqLog := base.With(zap.String("request_id", rid))
//   - ctx = WithLogger(ctx, reqLog)
func LoggerFromContext(ctx context.Context) Logger {
	if v := ctx.Value(loggerCtxKey{}); v != nil {
		if l, ok := v.(Logger); ok && l != nil && l.Zap() != nil {
			return l
		}
	}
	return NewNoopLogger()
}
