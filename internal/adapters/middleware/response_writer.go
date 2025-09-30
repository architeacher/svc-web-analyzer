package middleware

import (
	"bufio"
	"net"
	"net/http"
)

type FlushableResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	flusher      http.Flusher
	hijacker     http.Hijacker
	pusher       http.Pusher
}

func NewFlushableResponseWriter(w http.ResponseWriter) *FlushableResponseWriter {
	wrapper := &FlushableResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		bytesWritten:   0,
	}

	if flusher, ok := w.(http.Flusher); ok {
		wrapper.flusher = flusher
	}

	if hijacker, ok := w.(http.Hijacker); ok {
		wrapper.hijacker = hijacker
	}

	if pusher, ok := w.(http.Pusher); ok {
		wrapper.pusher = pusher
	}

	return wrapper
}

func (f *FlushableResponseWriter) WriteHeader(code int) {
	f.statusCode = code
	f.ResponseWriter.WriteHeader(code)
}

func (f *FlushableResponseWriter) Write(b []byte) (int, error) {
	n, err := f.ResponseWriter.Write(b)
	f.bytesWritten += int64(n)

	return n, err
}

func (f *FlushableResponseWriter) Flush() {
	if f.flusher != nil {
		f.flusher.Flush()
	}
}

func (f *FlushableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if f.hijacker != nil {
		return f.hijacker.Hijack()
	}

	return nil, nil, http.ErrNotSupported
}

func (f *FlushableResponseWriter) Push(target string, opts *http.PushOptions) error {
	if f.pusher != nil {
		return f.pusher.Push(target, opts)
	}

	return http.ErrNotSupported
}

func (f *FlushableResponseWriter) StatusCode() int {
	return f.statusCode
}

func (f *FlushableResponseWriter) BytesWritten() int64 {
	return f.bytesWritten
}

func (f *FlushableResponseWriter) Unwrap() http.ResponseWriter {
	return f.ResponseWriter
}
