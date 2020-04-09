package logger

// Logger - Interface to pass into Proxy for it to log messages
type ILogger interface {
	Trace(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	GetLevel() int
}

const LOG_LEVEL_ALL int = 0;
const LOG_LEVEL_INFO int = 1;
const LOG_LEVEL_WARN int = 2;
const LOG_LEVEL_NONE int = 3;
