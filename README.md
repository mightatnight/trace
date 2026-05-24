# trace

[![Go Report Card](https://goreportcard.com/badge/github.com/mightatnight/trace)](https://goreportcard.com/report/github.com/mightatnight/trace)
[![Go.Dev reference](https://img.shields.io/badge/go.dev-reference-blue?logo=go&logoColor=white)](https://pkg.go.dev/github.com/mightatnight/trace)

A simple, fast, and structured logging library for Go, built as a wrapper around `go.uber.org/zap`.

## Features

- **Simple API**: An intuitive and easy-to-use logging interface.
- **High Performance**: Built on `zap` for blazing-fast, low-allocation logging.
- **Structured Logging**: Log messages with key-value pairs for better machine-readability.
- **Flexible Output**: Log to `stdout`, a file, or both simultaneously.
- **No-Op Logger**: Easily disable logging for tests or specific environments with `NoopLogger`.
- **Global Default Logger**: Use package-level functions for convenience.
- **Goroutine-Safe**: Safe for concurrent use.

## Installation

```sh
go get github.com/mightatnight/trace
```

## Usage

### Basic Logging

Create a new logger and start logging messages. Logs are written to `stdout` by default.

```go
package main

import (
    "github.com/mightatnight/trace"
)

func main() {
    // Create a new logger with Info level
    logger := trace.New(trace.InfoLevel, "my-app", nil)

    logger.Info("Application started", trace.Str("version", "1.0.0"))
    logger.Warn("Something might be wrong")
}
```

### Logging to a File

To log to a file, provide an `*os.File` handle to the `New` function. Logs will be written to both `stdout` and the specified file.

```go
package main

import (
    "log"
    "os"
    "github.com/mightatnight/trace"
)

func main() {
    logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        log.Fatalf("Failed to open log file: %v", err)
    }
    defer logFile.Close()

    // Create a logger that writes to both stdout and app.log
    logger := trace.New(trace.InfoLevel, "my-app", logFile)

    logger.Error("This is an error message", trace.Err(err))
}
```

### Using the Default Logger

For convenience, you can set a global default logger and use package-level logging functions. This is typically done once at application startup.

```go
package main

import (
    "github.com/mightatnight/trace"
)

func main() {
    // Configure and set the default logger
    logger := trace.New(trace.InfoLevel, "my-app", nil)
    trace.SetDefaultLogger(logger)

    // Now use package-level functions anywhere in your app
    trace.Info("This message comes from the default logger")
}
```

### Structured Fields

Use the provided helper functions to add structured context to your logs.

```go
trace.Info("User logged in",
    trace.Int("userID", 12345),
    trace.Str("username", "testuser"),
    trace.Bool("isAdmin", false),
)
```

### Log Hooks

Attach a callback that runs whenever a log entry is emitted — useful for metrics, alerting, or forwarding logs to another sink. The callback receives the log level, the message, and the structured fields.

- `OnLog` runs the hook **synchronously**, in the calling goroutine.
- `OnLogAsync` runs the hook in a **separate goroutine**.

Both return a *new* logger (like `With`/`Named`), so use the returned value. Hooks fire only for entries that pass the configured level, and they propagate to child loggers created with `With`, `Named`, and `NewChildLogger`.

```go
package main

import (
    "github.com/mightatnight/trace"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func main() {
    logger := trace.New(trace.InfoLevel, "my-app", nil).
        OnLog(func(level zapcore.Level, msg string, fields []zap.Field) {
            if level >= trace.ErrorLevel {
                // e.g. increment an error counter, page on-call, etc.
            }
        })

    // The hook is carried over to child loggers.
    reqLog := logger.Named("http").With(zap.String("request_id", "abc123"))
    reqLog.Error("request failed") // hook fires here too
}
```

> **Note:** the `fields` slice passed to a hook must not be retained or mutated after the hook returns. Async hooks triggered by `Fatal` may not finish before the process exits.

### Disabling Logs

To disable all logging, you can use the `DisabledLevel` or the `NoopLogger`.

```go
// Using DisabledLevel to suppress logs
logger := trace.New(trace.DisabledLevel(), "", nil)
logger.Info("This will not be printed")

// The default logger is a NoopLogger until explicitly set
trace.Info("This will also not be printed")

// You can also explicitly set the NoopLogger
trace.SetDefaultLogger(&trace.NoopLogger{})
trace.Warn("This is also suppressed")
```
