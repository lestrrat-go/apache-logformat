package apachelog

import (
	"io"
	"net/http"
	"time"
)

type ApacheLog struct {
	logger   io.Writer
	format   string
	compiled func(io.Writer, Context) error
}

type Context interface {
	Header() http.Header
	Status() int
	Request() *http.Request
	ElapsedTime() time.Duration
}
