package queue

// LoggerAdapter adapts any logger to the queue logger interface
type LoggerAdapter struct {
	logger any
}

// NewLoggerAdapter creates a new logger adapter
func NewLoggerAdapter(logger any) *LoggerAdapter {
	return &LoggerAdapter{logger: logger}
}

// Info returns an info log event
func (l *LoggerAdapter) Info() LogEvent {
	if infoLogger, ok := l.logger.(interface {
		Info() interface {
			Msg(string)
			Str(string, string) interface{ Msg(string) }
		}
	}); ok {
		infoEvent := infoLogger.Info()
		return &LogEventAdapter{
			event:     infoEvent,
			infoEvent: infoEvent,
		}
	}
	return &LogEventAdapter{}
}

// Error returns an error log event
func (l *LoggerAdapter) Error() LogEvent {
	if errorLogger, ok := l.logger.(interface {
		Error() interface {
			Msg(string)
			Err(error) any
			Str(string, string) any
		}
	}); ok {
		errorEvent := errorLogger.Error()
		return &LogEventAdapter{
			event:      errorEvent,
			errorEvent: errorEvent,
		}
	}
	return &LogEventAdapter{}
}

// Debug returns a debug log event
func (l *LoggerAdapter) Debug() LogEvent {
	if debugLogger, ok := l.logger.(interface {
		Debug() interface{ Msg(string) }
	}); ok {
		debugEvent := debugLogger.Debug()
		return &LogEventAdapter{
			event:      debugEvent,
			debugEvent: debugEvent,
		}
	}
	return &LogEventAdapter{}
}

// LogEventAdapter adapts log events to the queue log event interface
type LogEventAdapter struct {
	event      any
	infoEvent  any
	errorEvent any
	debugEvent any
}

// Msg logs a message
func (l *LogEventAdapter) Msg(msg string) {
	if l.infoEvent != nil {
		if msgEvent, ok := l.infoEvent.(interface{ Msg(string) }); ok {
			msgEvent.Msg(msg)
		}
	} else if l.errorEvent != nil {
		if msgEvent, ok := l.errorEvent.(interface{ Msg(string) }); ok {
			msgEvent.Msg(msg)
		}
	} else if l.debugEvent != nil {
		if msgEvent, ok := l.debugEvent.(interface{ Msg(string) }); ok {
			msgEvent.Msg(msg)
		}
	} else if msgEvent, ok := l.event.(interface{ Msg(string) }); ok {
		msgEvent.Msg(msg)
	}
}

// Err adds an error to the log event
func (l *LogEventAdapter) Err(err error) LogEvent {
	// Use type switching for simpler interface checking
	switch {
	case l.errorEvent != nil:
		// Check if errorEvent has an Err method
		if errMethod := getErrMethod(l.errorEvent); errMethod != nil {
			newErrorEvent := errMethod(err)
			return &LogEventAdapter{
				event:      newErrorEvent,
				errorEvent: newErrorEvent,
			}
		}
	case l.event != nil:
		// Check if event has an Err method
		if errMethod := getErrMethod(l.event); errMethod != nil {
			return &LogEventAdapter{event: errMethod(err)}
		}
	}
	return l
}

// Helper function to extract Err method from any interface
func getErrMethod(event any) func(error) any {
	if errEvent, ok := event.(interface{ Err(error) any }); ok {
		return errEvent.Err
	}
	// Try alternative interface signatures
	if errEvent, ok := event.(interface {
		Err(error) interface {
			Msg(string)
			Str(string, string) interface{ Msg(string) }
		}
	}); ok {
		return func(e error) any {
			return errEvent.Err(e)
		}
	}
	return nil
}

// Str adds a string field to the log event
func (l *LogEventAdapter) Str(key, value string) LogEvent {
	// Use type switching for simpler interface checking
	switch {
	case l.infoEvent != nil:
		// Check if infoEvent has a Str method
		if strMethod := getStrMethod(l.infoEvent); strMethod != nil {
			newInfoEvent := strMethod(key, value)
			return &LogEventAdapter{
				event:     newInfoEvent,
				infoEvent: newInfoEvent,
			}
		}
	case l.errorEvent != nil:
		// Check if errorEvent has a Str method
		if strMethod := getStrMethod(l.errorEvent); strMethod != nil {
			newErrorEvent := strMethod(key, value)
			return &LogEventAdapter{
				event:      newErrorEvent,
				errorEvent: newErrorEvent,
			}
		}
	case l.event != nil:
		// Check if event has a Str method
		if strMethod := getStrMethod(l.event); strMethod != nil {
			return &LogEventAdapter{event: strMethod(key, value)}
		}
	}
	return l
}

// getStrMethod is a helper function to extract Str method from any interface.
func getStrMethod(event any) func(string, string) any {
	if strEvent, ok := event.(interface{ Str(string, string) any }); ok {
		return strEvent.Str
	}
	// Try alternative interface signatures
	if strEvent, ok := event.(interface {
		Str(string, string) interface{ Msg(string) }
	}); ok {
		return func(key, value string) any {
			return strEvent.Str(key, value)
		}
	}
	if strEvent, ok := event.(interface {
		Str(string, string) interface {
			Msg(string)
			Str(string, string) interface{ Msg(string) }
		}
	}); ok {
		return func(key, value string) any {
			return strEvent.Str(key, value)
		}
	}
	return nil
}
