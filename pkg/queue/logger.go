package queue

// Logger defines a simple logging interface to avoid circular dependencies
type Logger interface {
	Info() LogEvent
	Error() LogEvent
	Debug() LogEvent
}

// LogEvent defines a simple log event interface
type LogEvent interface {
	Msg(string)
	Err(error) LogEvent
	Str(string, string) LogEvent
}

// InfoLogEvent defines an info-specific log event interface
type InfoLogEvent interface {
	LogEvent
}

// ErrorLogEvent defines an error-specific log event interface
type ErrorLogEvent interface {
	LogEvent
}

// DebugLogEvent defines a debug-specific log event interface
type DebugLogEvent interface {
	LogEvent
}