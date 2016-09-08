package apachelog

import "errors"

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
