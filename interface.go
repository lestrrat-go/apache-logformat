package apachelog

import (
	"errors"
	"io"
	"net/http"
	"time"
)

type ApacheLog struct {
	format *Format
}

// Combined is a pre-defined ApacheLog struct to log "common" log format
var CommonLog, _ = New(`%h %l %u %t "%r" %>s %b`)

// Combined is a pre-defined ApacheLog struct to log "combined" log format
var CombinedLog, _ = New(`%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-agent}i"`)

var (
	ErrInvalidRuneSequence = errors.New("invalid rune sequence found in format")
	ErrUnimplemented       = errors.New("pattern unimplemented")
)

type LogCtx struct {
	Request        *http.Request
	RequestTime    time.Time
	ResponseStatus int
	ResponseHeader http.Header
	ElapsedTime    time.Duration
}

// Format describes an Apache log format. Given a logging context,
// it can create a log line.
type Format struct {
	writers []FormatWriter
}

type FormatWriter interface {
	WriteTo(io.Writer, *LogCtx) error
}

type FormatWriteFunc func(io.Writer, *LogCtx) error
