package trace

var defaultLogger = NewNoopLogger()

func SetDefaultLogger(logger Logger) {
	defaultLogger = logger
}

func GetDefaultLogger() Logger {
	return defaultLogger
}
