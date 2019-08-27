package httputil

import (
	"net/http"
	"sync"
)

var responseWriterPool sync.Pool

func init() {
	responseWriterPool.New = allocResponseWriter
}

type ResponseWriter struct {
	responseContentLength int64
	responseStatus        int
	responseWriter        http.ResponseWriter
}

func GetResponseWriter(w http.ResponseWriter) *ResponseWriter {
	rw := responseWriterPool.Get().(*ResponseWriter)
	rw.responseWriter = w
	return rw
}

func ReleaseResponseWriter(rw *ResponseWriter) {
	rw.Reset()
	responseWriterPool.Put(rw)
}

func allocResponseWriter() interface{} {
	rw := &ResponseWriter{}
	rw.Reset()
	return rw
}

func (rw ResponseWriter) ContentLength() int64 {
	return rw.responseContentLength
}

func (rw ResponseWriter) StatusCode() int {
	return rw.responseStatus
}

func (rw *ResponseWriter) Reset() {
	rw.responseContentLength = 0
	rw.responseStatus = http.StatusOK
	rw.responseWriter = nil
}

func (rw *ResponseWriter) Write(buf []byte) (int, error) {
	n, err := rw.responseWriter.Write(buf)
	rw.responseContentLength += int64(n)
	return n, err
}

func (rw *ResponseWriter) Header() http.Header {
	return rw.responseWriter.Header()
}

func (rw *ResponseWriter) WriteHeader(status int) {
	rw.responseStatus = status
	rw.responseWriter.WriteHeader(status)
}

func (rw *ResponseWriter) Flush() {
	if f, ok := rw.responseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
