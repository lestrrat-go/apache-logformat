package apachelog

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

var logBufferPool sync.Pool
var logCtxPool sync.Pool
var responseWriterPool sync.Pool

func init() {
	logBufferPool.New = allocLogBuffer
	logCtxPool.New = allocLogCtx
	responseWriterPool.New = allocResponseWriter
}

func allocLogBuffer() interface{} {
	return &bytes.Buffer{}
}

func getLogBuffer() *bytes.Buffer {
	return logBufferPool.Get().(*bytes.Buffer)
}

func releaseLogBuffer(v *bytes.Buffer) {
	v.Reset()
	logBufferPool.Put(v)
}

func allocLogCtx() interface{} {
	return &LogCtx{}
}

func getLogCtx() *LogCtx {
	return logCtxPool.Get().(*LogCtx)
}

func releaseLogCtx(v *LogCtx) {
	v.Request = nil
	v.RequestTime = time.Time{}
	v.ResponseStatus = http.StatusOK
	v.ResponseHeader = nil
	v.ElapsedTime = 0
	logCtxPool.Put(v)
}

func allocResponseWriter() interface{} {
	return &absorbingResponseWriter{}
}

func getResponseWriter(w http.ResponseWriter, ctx *LogCtx) *absorbingResponseWriter {
	w2 := responseWriterPool.Get().(*absorbingResponseWriter)
	w2.w = w
	w2.ctx = ctx
	return w2
}

func releaseResponseWriter(v *absorbingResponseWriter) {
	v.w = nil
	v.ctx = nil
	responseWriterPool.Put(v)
}
