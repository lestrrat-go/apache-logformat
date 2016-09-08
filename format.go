package apachelog

import (
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkg/errors"
)

func (f FormatWriteFunc) WriteTo(dst io.Writer, ctx *LogCtx) error {
	return f(dst, ctx)
}

var emptyValue = []byte{'-'}

func valueOf(s string) []byte {
	if s == "" {
		return emptyValue
	}
	return []byte(s)
}

type fixedByteSequence []byte

func (seq fixedByteSequence) WriteTo(dst io.Writer, _ *LogCtx) error {
	if _, err := dst.Write([]byte(seq)); err != nil {
		return errors.Wrapf(err, "failed to write fixed byte sequence %s", seq)
	}
	return nil
}

type requestHeader string

func (h requestHeader) WriteTo(dst io.Writer, ctx *LogCtx) error {
	v := ctx.Request.Header.Get(string(h))
	if _, err := dst.Write(valueOf(v)); err != nil {
		return errors.Wrap(err, "failed to write request header value")
	}
	return nil
}

type responseHeader string

func (h responseHeader) WriteTo(dst io.Writer, ctx *LogCtx) error {
	v := ctx.ResponseHeader.Get(string(h))
	if _, err := dst.Write(valueOf(v)); err != nil {
		return errors.Wrap(err, "failed to write response header value")
	}
	return nil
}

func makeElapsedTime(base time.Duration) FormatWriter {
	return FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
		var str string
		if elapsed := ctx.ElapsedTime; elapsed > 0 {
			str = strconv.Itoa(int(elapsed / base))
		}
		if _, err := dst.Write(valueOf(str)); err != nil {
			return errors.Wrap(err, "failed to write elapsed time")
		}
		return nil
	})
}

var elapsedTimeMicroSeconds = makeElapsedTime(time.Microsecond)
var elapsedTimeSeconds = makeElapsedTime(time.Second)

var requestHttpMethod = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	v := valueOf(ctx.Request.Method)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request method")
	}
	return nil
})

var requestHttpProto = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	v := valueOf(ctx.Request.Proto)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request HTTP request proto")
	}
	return nil
})

var requestRemoteAddr = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	v := valueOf(ctx.Request.RemoteAddr)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request remote address")
	}
	return nil
})

var pid = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	v := valueOf(strconv.Itoa(os.Getpid()))
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write pid")
	}
	return nil
})

var rawQuery = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	q := ctx.Request.URL.RawQuery
	if q != "" {
		q = "?" + q
	}
	v := valueOf(q)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write raw request query")
	}
	return nil
})

var requestLine = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	buf := getLogBuffer()
	defer releaseLogBuffer(buf)

	r := ctx.Request
	buf.WriteString(r.Method)
	buf.WriteByte(' ')
	buf.WriteString(r.URL.String())
	buf.WriteByte(' ')
	buf.WriteString(r.Proto)
	if _, err := buf.WriteTo(dst); err != nil {
		return errors.Wrap(err, "failed to write request line")
	}
	return nil
})

var httpStatus = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	v := valueOf(strconv.Itoa(ctx.ResponseStatus))
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write response status")
	}
	return nil
})

var requestTime = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	v := valueOf(ctx.RequestTime.Format("02/Jan/2006:15:04:05 -0700"))
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request time")
	}
	return nil
})

var urlPath = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	v := valueOf(ctx.Request.URL.Path)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request URL path")
	}
	return nil
})

var username = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	var v []byte
	if u := ctx.Request.URL.User; u != nil {
		v = valueOf(u.Username())
	} else {
		v = emptyValue
	}
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write username")
	}
	return nil
})

var requestHost = FormatWriteFunc(func(dst io.Writer, ctx *LogCtx) error {
	var v []byte
	h := ctx.Request.Host
	if strings.IndexByte(h, ':') > 0 {
		v = valueOf(strings.Split(ctx.Request.Host, ":")[0])
	} else {
		v = valueOf(h)
	}
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request host")
	}
	return nil
})

func (f *Format) compile(s string) error {
	var cbs []FormatWriter

	start := 0
	max := len(s)

	for i := 0; i < max; {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError {
			return errors.Wrap(ErrInvalidRuneSequence, "failed to compile format")
		}
		i += n

		// Not q sequence... go to next rune
		if r != '%' {
			continue
		}

		if start != i {
			// this *could* be the last element in string, in which case we just
			// say meh, just assume this was a literal percent.
			if i == max {
				cbs = append(cbs, fixedByteSequence(s[start:i]))
				start = i
				break
			}
			cbs = append(cbs, fixedByteSequence(s[start:i-1]))
		}

		// Find what we have next.

		r, n = utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError {
			return errors.Wrap(ErrInvalidRuneSequence, "failed to compile format")
		}
		i += n

		switch r {
		case '%':
			cbs = append(cbs, fixedByteSequence([]byte{'%'}))
			start = i + n - 1
		case 'b':
			cbs = append(cbs, requestHeader("Content-Length"))
			start = i + n - 1
		case 'D': // custom
			cbs = append(cbs, elapsedTimeMicroSeconds)
			start = i + n - 1
		case 'h':
			cbs = append(cbs, requestRemoteAddr)
			start = i + n - 1
		case 'H':
			cbs = append(cbs, requestHttpProto)
			start = i + n - 1
		case 'l':
			cbs = append(cbs, fixedByteSequence(emptyValue))
			start = i + n - 1
		case 'm':
			cbs = append(cbs, requestHttpMethod)
			start = i + n - 1
		case 'p':
			cbs = append(cbs, pid)
			start = i + n - 1
		case 'P':
			// Unimplemented
			return errors.Wrap(ErrUnimplemented, "failed to compile format")
		case 'q':
			cbs = append(cbs, rawQuery)
			start = i + n - 1
		case 'r':
			cbs = append(cbs, requestLine)
			start = i + n - 1
		case 's':
			cbs = append(cbs, httpStatus)
			start = i + n - 1
		case 't':
			cbs = append(cbs, requestTime)
			start = i + n - 1
		case 'T': // custom
			cbs = append(cbs, elapsedTimeSeconds)
			start = i + n - 1
		case 'u':
			cbs = append(cbs, username)
			start = i + n - 1
		case 'U':
			cbs = append(cbs, urlPath)
			start = i + n - 1
		case 'V', 'v':
			cbs = append(cbs, requestHost)
			start = i + n - 1
		case '>':
			if max >= i && s[i] == 's' {
				// "Last" status doesn't exist in our case, so it's the same as %s
				cbs = append(cbs, httpStatus)
				start = i + 1
				i = i + 1
			} else {
				// Otherwise we don't know what this is. just do a verbatim copy
				cbs = append(cbs, fixedByteSequence([]byte{'%', '>'}))
				start = i + n - 1
			}
		case '{':
			// Search the next }
			end := -1
			for j := i; j < max; j++ {
				if s[j] == '}' {
					end = j
					break
				}
			}

			if end != -1 && end < max-1 { // Found it!
				// check for suffix
				blockType := s[end+1]
				key := s[i:end]
				switch blockType {
				case 'i':
					cbs = append(cbs, requestHeader(key))
				case 'o':
					cbs = append(cbs, responseHeader(key))
				default: // case 't':
					return errors.Wrap(ErrUnimplemented, "failed to compile format")
				}

				start = end + 2
				i = end + 1
			} else {
				cbs = append(cbs, fixedByteSequence([]byte{'%', '{'}))
				start = i + n - 1
			}
		}
	}

	if start < max {
		cbs = append(cbs, fixedByteSequence(s[start:max]))
	}

	f.writers = cbs
	return nil
}

func (f *Format) WriteTo(dst io.Writer, ctx *LogCtx) error {
	for _, w := range f.writers {
		if err := w.WriteTo(dst, ctx); err != nil {
			return errors.Wrap(err, "failed to execute FormatWriter")
		}
	}
	return nil
}
