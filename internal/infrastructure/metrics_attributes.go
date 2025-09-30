package infrastructure

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

const (
	httpMethodKey     = "http.method"
	httpPathKey       = "http.path"
	httpStatusCodeKey = "http.status_code"
	statusKey         = "status"
	errorTypeKey      = "error.type"
	priorityKey       = "priority"
	linkTypeKey       = "link.type"
)

func HTTPMethodAttr(method string) attribute.KeyValue {
	return attribute.String(httpMethodKey, method)
}

func HTTPPathAttr(path string) attribute.KeyValue {
	return attribute.String(httpPathKey, path)
}

func HTTPStatusCodeAttr(code int) attribute.KeyValue {
	return attribute.String(httpStatusCodeKey, fmt.Sprintf("%d", code))
}

func StatusAttr(status string) attribute.KeyValue {
	return attribute.String(statusKey, status)
}

func ErrorTypeAttr(errorType string) attribute.KeyValue {
	return attribute.String(errorTypeKey, errorType)
}

func PriorityAttr(priority string) attribute.KeyValue {
	return attribute.String(priorityKey, priority)
}

func LinkTypeAttr(linkType string) attribute.KeyValue {
	return attribute.String(linkTypeKey, linkType)
}
