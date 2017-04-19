package apachelog

import (
	"io"
	"net/http"
	"os"

	"github.com/lestrrat/go-apache-logformat/internal/httputil"
	"github.com/lestrrat/go-apache-logformat/internal/logctx"
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
func (al *ApacheLog) WriteLog(dst io.Writer, ctx LogCtx) error {
	buf := getLogBuffer()
	defer releaseLogBuffer(buf)

	if err := al.format.WriteTo(buf, ctx); err != nil {
		return errors.Wrap(err, "failed to format log line")
	}

	b := buf.Bytes()
	if b[len(b)-1] != '\n' {
		buf.WriteByte('\n')
	}

	if _, err := buf.WriteTo(dst); err != nil {
		return errors.Wrap(err, "failed to write formated line to destination")
	}
	return nil
}

// Wrap creates a new http.Handler that logs a formatted log line
// to dst.
func (al *ApacheLog) Wrap(h http.Handler, dst io.Writer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := logctx.Get(r)
		defer logctx.Release(ctx)

		wrapped := httputil.GetResponseWriter(w)
		defer httputil.ReleaseResponseWriter(wrapped)

		defer func() {
			ctx.Finalize(wrapped)
			if err := al.WriteLog(dst, ctx); err != nil {
				// Hmmm... no where to log except for stderr
				os.Stderr.Write([]byte(err.Error()))
				return
			}
		}()

		h.ServeHTTP(wrapped, r)
	})
}
