package logger

// NullLogger - An empty logger that ignores everything
type nilLogger struct{}

// Trace - no-op
func (logger *nilLogger) Trace(format string, args ...interface{}) {}

// Debug - no-op
func (logger *nilLogger) Debug(format string, args ...interface{}) {}

// Info - no-op
func (logger *nilLogger) Info(format string, args ...interface{}) {}

// Warn - no-op
func (logger *nilLogger) Warn(format string, args ...interface{}) {}

// Warn - no-op
func (logger *nilLogger) Error(format string, args ...interface{}) {}

func (logger *nilLogger) GetLevel() int {
	return LOG_LEVEL_NONE
}

var NilLogger = &nilLogger{}
