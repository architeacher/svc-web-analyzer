package ports

import "go.opentelemetry.io/otel/trace"

type Tracer interface {
	trace.TracerProvider
}
