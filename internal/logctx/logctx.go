package logctx

import (
	"net/http"
	"sync"
	"time"

	"github.com/lestrrat/go-apache-logformat/internal/httputil"
)

type clock interface {
	Now() time.Time
}

type defaultClock struct{}

func (_ defaultClock) Now() time.Time {
	return time.Now()
}

var Clock clock = defaultClock{}

type Context struct {
	elapsedTime           time.Duration
	request               *http.Request
	requestTime           time.Time
	responseContentLength int64
	responseHeader        http.Header
	responseStatus        int
	responseTime          time.Time
}

var pool = sync.Pool{New: allocCtx}

func allocCtx() interface{} {
	return &Context{}
}

func Get(r *http.Request) *Context {
	ctx := pool.Get().(*Context)
	ctx.request = r
	ctx.requestTime = Clock.Now()
	return ctx
}

func Release(ctx *Context) {
	ctx.Reset()
	pool.Put(ctx)
}

func (ctx *Context) ElapsedTime() time.Duration {
	return ctx.elapsedTime
}

func (ctx *Context) Request() *http.Request {
	return ctx.request
}

func (ctx *Context) RequestTime() time.Time {
	return ctx.requestTime
}

func (ctx *Context) ResponseContentLength() int64 {
	return ctx.responseContentLength
}

func (ctx *Context) ResponseHeader() http.Header {
	return ctx.responseHeader
}

func (ctx *Context) ResponseStatus() int {
	return ctx.responseStatus
}

func (ctx *Context) ResponseTime() time.Time {
	return ctx.responseTime
}

func (ctx *Context) Reset() {
	ctx.elapsedTime = time.Duration(0)
	ctx.request = nil
	ctx.requestTime = time.Time{}
	ctx.responseContentLength = 0
	ctx.responseHeader = http.Header{}
	ctx.responseStatus = http.StatusOK
	ctx.responseTime = time.Time{}
}

func (ctx *Context) Finalize(wrapped *httputil.ResponseWriter) {
	ctx.responseTime = Clock.Now()
	ctx.elapsedTime = ctx.responseTime.Sub(ctx.requestTime)
	ctx.responseContentLength = wrapped.ContentLength()
	ctx.responseHeader = wrapped.Header()
	ctx.responseStatus = wrapped.StatusCode()
}
