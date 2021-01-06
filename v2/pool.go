package apachelog

import (
	"bytes"
	"sync"
)

var logBufferPool sync.Pool

func init() {
	logBufferPool.New = allocLogBuffer
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
