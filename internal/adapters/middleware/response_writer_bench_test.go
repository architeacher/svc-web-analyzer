package middleware

import (
	"net/http/httptest"
	"testing"
)

func BenchmarkFlushableResponseWriter_Write(b *testing.B) {
	recorder := httptest.NewRecorder()
	wrapped := NewFlushableResponseWriter(recorder)
	data := []byte("benchmark data")

	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _ = wrapped.Write(data)
	}
}

func BenchmarkFlushableResponseWriter_Flush(b *testing.B) {
	mock := &mockFlusher{ResponseWriter: httptest.NewRecorder()}
	wrapped := NewFlushableResponseWriter(mock)

	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		wrapped.Flush()
	}
}
