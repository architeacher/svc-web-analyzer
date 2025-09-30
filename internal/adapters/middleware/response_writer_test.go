package middleware

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type (
	FlushableResponseWriterTestSuite struct {
		suite.Suite
		recorder *httptest.ResponseRecorder
		wrapped  *FlushableResponseWriter
	}

	mockFlusher struct {
		http.ResponseWriter
		flushed bool
	}

	mockHijacker struct {
		http.ResponseWriter
		hijacked bool
	}

	mockPusher struct {
		http.ResponseWriter
		pushed bool
	}
)

func (m *mockFlusher) Flush() {
	m.flushed = true
}

func (m *mockHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijacked = true

	return nil, nil, nil
}

func (m *mockPusher) Push(target string, opts *http.PushOptions) error {
	m.pushed = true

	return nil
}

func (s *FlushableResponseWriterTestSuite) SetupTest() {
	s.recorder = httptest.NewRecorder()
	s.wrapped = NewFlushableResponseWriter(s.recorder)
}

func (s *FlushableResponseWriterTestSuite) TestAlwaysImplementsStandardInterfaces() {
	var w interface{} = s.wrapped

	_, ok := w.(http.Flusher)
	s.Require().True(ok, "FlushableResponseWriter should always implement Flusher")

	_, ok = w.(http.Hijacker)
	s.Require().True(ok, "FlushableResponseWriter should always implement Hijacker")

	_, ok = w.(http.Pusher)
	s.Require().True(ok, "FlushableResponseWriter should always implement Pusher")
}

func (s *FlushableResponseWriterTestSuite) TestCorrectlyInitializesWithUnderlyingWriter() {
	s.Require().Equal(http.StatusOK, s.wrapped.StatusCode(), "expected default status code")
	s.Require().Equal(int64(0), s.wrapped.BytesWritten(), "expected bytes written to be 0")
	s.Require().Equal(s.recorder, s.wrapped.Unwrap(), "Unwrap should return the underlying response writer")
}

func (s *FlushableResponseWriterTestSuite) TestFlush_DelegatesToUnderlyingFlusher() {
	mock := &mockFlusher{ResponseWriter: httptest.NewRecorder()}
	wrapped := NewFlushableResponseWriter(mock)

	wrapped.Flush()

	s.Require().True(mock.flushed, "expected underlying flusher to be called")
}

func (s *FlushableResponseWriterTestSuite) TestFlush_NoPanicWhenUnderlyingWriterIsNotFlushable() {
	wrapped := NewFlushableResponseWriter(httptest.NewRecorder())

	s.Require().NotPanics(func() {
		wrapped.Flush()
	}, "Flush() should not panic")
}

func (s *FlushableResponseWriterTestSuite) TestHijack_DelegatesToUnderlyingHijacker() {
	mock := &mockHijacker{ResponseWriter: httptest.NewRecorder()}
	wrapped := NewFlushableResponseWriter(mock)

	_, _, _ = wrapped.Hijack()

	s.Require().True(mock.hijacked, "expected underlying hijacker to be called")
}

func (s *FlushableResponseWriterTestSuite) TestPush_DelegatesToUnderlyingPusher() {
	mock := &mockPusher{ResponseWriter: httptest.NewRecorder()}
	wrapped := NewFlushableResponseWriter(mock)

	_ = wrapped.Push("/test", nil)

	s.Require().True(mock.pushed, "expected underlying pusher to be called")
}

func (s *FlushableResponseWriterTestSuite) TestWriteHeader_TracksStatusCode() {
	s.wrapped.WriteHeader(http.StatusCreated)

	s.Require().Equal(http.StatusCreated, s.wrapped.StatusCode(), "expected status code")
}

func (s *FlushableResponseWriterTestSuite) TestWriteHeader_DefaultsTo200OK() {
	s.Require().Equal(http.StatusOK, s.wrapped.StatusCode(), "expected default status code")
}

func (s *FlushableResponseWriterTestSuite) TestWrite_TracksBytesWritten() {
	data := []byte("test data")
	n, err := s.wrapped.Write(data)

	s.Require().NoError(err, "unexpected error")
	s.Require().Equal(len(data), n, "expected to write correct number of bytes")
	s.Require().Equal(int64(len(data)), s.wrapped.BytesWritten(), "expected bytes written")
}

func (s *FlushableResponseWriterTestSuite) TestWrite_AccumulatesBytesWritten() {
	_, _ = s.wrapped.Write([]byte("first"))
	_, _ = s.wrapped.Write([]byte("second"))

	expected := int64(len("first") + len("second"))
	s.Require().Equal(expected, s.wrapped.BytesWritten(), "expected bytes written")
}

func (s *FlushableResponseWriterTestSuite) TestAccessLogger_PreservesFlusherInterface() {
	mock := &mockFlusher{ResponseWriter: httptest.NewRecorder()}
	wrapped := newResponseWriter(mock)

	var w interface{} = wrapped
	_, ok := w.(http.Flusher)
	s.Require().True(ok, "AccessLogger response writer should preserve Flusher interface")
}

func (s *FlushableResponseWriterTestSuite) TestMetricsMiddleware_PreservesFlusherInterface() {
	mock := &mockFlusher{ResponseWriter: httptest.NewRecorder()}
	wrapped := newMetricsResponseWriter(mock)

	var w interface{} = wrapped
	_, ok := w.(http.Flusher)
	s.Require().True(ok, "MetricsMiddleware response writer should preserve Flusher interface")
}

func TestFlushableResponseWriterTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(FlushableResponseWriterTestSuite))
}
