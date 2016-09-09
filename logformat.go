package apachelog

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
)

// New creates a new ApacheLog instance from the given
// format. It will return an error if the format fails to compile.
func New(format string) (*ApacheLog, error) {
	var f Format
	if err := f.compile(format); err != nil {
		return nil, errors.Wrap(err, "failed to compile log format")
	}

	return &ApacheLog{format: &f}, nil
}

// WriteLog generates a log line using the format associated with the
// ApacheLog instance, using the values from ctx. The result is written
// to dst
func (al *ApacheLog) WriteLog(dst io.Writer, ctx *LogCtx) error {
	buf := getLogBuffer()
	defer releaseLogBuffer(buf)

	if err := al.format.WriteTo(buf, ctx); err != nil {
		return errors.Wrap(err, "failed to format log line")
	}

	b := buf.Bytes()
	if b[len(b)-1] != '\n' {
		buf.Write([]byte{'\n'})
	}

	if _, err := buf.WriteTo(dst); err != nil {
		return errors.Wrap(err, "failed to write formated line to destination")
	}
	return nil
}

type absorbingResponseWriter struct {
	w   http.ResponseWriter
	ctx *LogCtx
}

func (w *absorbingResponseWriter) Write(buf []byte) (int, error) {
	return w.w.Write(buf)
}

func (w *absorbingResponseWriter) Header() http.Header {
	return w.w.Header()
}

func (w *absorbingResponseWriter) WriteHeader(status int) {
	w.ctx.ResponseStatus = status
	w.w.WriteHeader(status)
}

// Wrap creates a new http.Handler that logs a formatted log line
// to dst.
func (al *ApacheLog) Wrap(h http.Handler, dst io.Writer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := getLogCtx()
		defer releaseLogCtx(ctx)

		ctx.Request = r
		ctx.RequestTime = time.Now()
		ctx.ResponseStatus = http.StatusOK

		w2 := getResponseWriter(w, ctx)
		defer releaseResponseWriter(w2)

		defer func() {
			ctx.ResponseHeader = w2.Header()
			ctx.ElapsedTime = time.Since(ctx.RequestTime)
			if err := al.WriteLog(dst, ctx); err != nil {
				// Hmmm... no where to log except for stderr
				os.Stderr.Write([]byte(err.Error()))
				return
			}
		}()

		h.ServeHTTP(w2, r)
	})
}
