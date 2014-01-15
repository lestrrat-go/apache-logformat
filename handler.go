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
  http.ListenAndServe(addr, apachelog.WrapLoggingWriter(self.ServeHTTP, logger))

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
  logger                *ApacheLog
  responseWriter        http.ResponseWriter
  request               *http.Request
  reqtime               time.Time
  status                int
  responseBytes         int64
}

func WrapLoggingWriter(h http.HandlerFunc, logger *ApacheLog) http.HandlerFunc {
  return func (w http.ResponseWriter, r *http.Request) {
    lw := NewLoggingWriter(w, r, logger)
    defer lw.EmitLog()
    h(lw, r)
  }
}

func NewLoggingWriter(w http.ResponseWriter, r *http.Request, logger *ApacheLog) (*LoggingWriter) {
  return &LoggingWriter {
    logger,
    w,
    r,
    time.Now(),
    200,
    0,
  }
}

func (self *LoggingWriter) Header() http.Header {
  return self.responseWriter.Header()
}

func (self *LoggingWriter) Write(p []byte) (int, error) {
  written, err := self.responseWriter.Write(p)
  self.responseBytes += int64(written)
  return written, err
}

func (self *LoggingWriter) WriteHeader(status int) {
  self.status = status
  self.responseWriter.WriteHeader(status)
}

func (self *LoggingWriter) EmitLog() {
  self.logger.LogLine(
    self.request,
    self.status,
    self.responseWriter.Header(),
    time.Since(self.reqtime),
  )
}