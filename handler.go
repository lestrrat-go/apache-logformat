package apachelog

import (
	"net/http"
	"time"
)

/*

LoggingWriter is a convenience struct that comes with go-apache-logformat
which allows you to easily integrate net/http and go-apache-logformat.

It's easy to wrap an existing handler:

  logger := apachelog.NewApacheLog(os.Stderr, ...)
  http.ListenAndServe(addr, apachelog.WrapLoggingWriter(lw.ServeHTTP, logger))

Or you can call it manually inside your handler:

  func ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ...

    logger := apachelog.NewApacheLog(os.Stderr, ...)
    lw := apachelog.NewLoggingWriter(w, r, logger)
    defer lw.EmitLog()

    // Use lw instead of w from here on
    ...
  }

*/
type LoggingWriter struct {
	logger         *ApacheLog
	responseWriter http.ResponseWriter
	request        *http.Request
	reqtime        time.Time
	status         int
	responseBytes  int64
}

/*
WrapLoggingWriter wraps http.HandlerFunc to use the ApacheLog logger
*/
func WrapLoggingWriter(h http.HandlerFunc, logger *ApacheLog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lw := NewLoggingWriter(w, r, logger)
		defer lw.EmitLog()
		h(lw, r)
	}
}

func NewLoggingWriter(w http.ResponseWriter, r *http.Request, logger *ApacheLog) *LoggingWriter {
	return &LoggingWriter{
		logger,
		w,
		r,
		time.Now(),
		200,
		0,
	}
}

func (lw *LoggingWriter) Header() http.Header {
	return lw.responseWriter.Header()
}

func (lw *LoggingWriter) Write(p []byte) (int, error) {
	written, err := lw.responseWriter.Write(p)
	lw.responseBytes += int64(written)
	return written, err
}

func (lw *LoggingWriter) WriteHeader(status int) {
	lw.status = status
	lw.responseWriter.WriteHeader(status)
}

func (lw *LoggingWriter) EmitLog() {
	lw.logger.LogLine(
		lw.request,
		lw.status,
		lw.responseWriter.Header(),
		time.Since(lw.reqtime),
	)
}
