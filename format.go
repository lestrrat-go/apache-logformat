package apachelog

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	strftime "github.com/lestrrat/go-strftime"
	"github.com/pkg/errors"
)

func (f FormatWriteFunc) WriteTo(dst io.Writer, ctx LogCtx) error {
	return f(dst, ctx)
}

var dashValue = []byte{'-'}
var emptyValue = []byte(nil)

func valueOf(s string, replacement []byte) []byte {
	if s == "" {
		return replacement
	}
	return []byte(s)
}

type fixedByteSequence []byte

func (seq fixedByteSequence) WriteTo(dst io.Writer, _ LogCtx) error {
	if _, err := dst.Write([]byte(seq)); err != nil {
		return errors.Wrapf(err, "failed to write fixed byte sequence %s", seq)
	}
	return nil
}

type requestHeader string

func (h requestHeader) WriteTo(dst io.Writer, ctx LogCtx) error {
	v := ctx.Request().Header.Get(string(h))
	if _, err := dst.Write(valueOf(v, dashValue)); err != nil {
		return errors.Wrap(err, "failed to write request header value")
	}
	return nil
}

type responseHeader string

func (h responseHeader) WriteTo(dst io.Writer, ctx LogCtx) error {
	v := ctx.ResponseHeader().Get(string(h))
	if _, err := dst.Write(valueOf(v, dashValue)); err != nil {
		return errors.Wrap(err, "failed to write response header value")
	}
	return nil
}

func makeRequestTimeBegin(s string) (FormatWriter, error) {
	f, err := strftime.New(s)
	if err != nil {
		return nil, errors.Wrap(err, `failed to compile strftime pattern`)
	}

	return FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
		return f.Format(dst, ctx.RequestTime())
	}), nil
}

func makeRequestTimeEnd(s string) (FormatWriter, error) {
	f, err := strftime.New(s)
	if err != nil {
		return nil, errors.Wrap(err, `failed to compile strftime pattern`)
	}

	return FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
		return f.Format(dst, ctx.ResponseTime())
	}), nil
}

func timeFormatter(key string) (FormatWriter, error) {
	var formatter FormatWriter
	switch key {
	case "sec":
		formatter = requestTimeSecondsSinceEpoch
	case "msec":
		formatter = requestTimeMillisecondsSinceEpoch
	case "usec":
		formatter = requestTimeMicrosecondsSinceEpoch
	case "msec_frac":
		formatter = requestTimeMillisecondsFracSinceEpoch
	case "usec_frac":
		formatter = requestTimeMicrosecondsFracSinceEpoch
	default:
		const beginPrefix = "begin:"
		const endPrefix = "end:"
		if strings.HasPrefix(key, beginPrefix) {
			return makeRequestTimeBegin(key[len(beginPrefix):])
		} else if strings.HasPrefix(key, endPrefix) {
			return makeRequestTimeEnd(key[len(endPrefix):])
		}
		// if specify the format of strftime(3) without begin: or end:, same as bigin:
		// FYI https://httpd.apache.org/docs/current/en/mod/mod_log_config.html
		return makeRequestTimeBegin(key)
	}
	return formatter, nil
}

var epoch = time.Unix(0, 0)

func makeRequestTimeSinceEpoch(base time.Duration) FormatWriter {
	return FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
		dur := ctx.RequestTime().Sub(epoch)
		s := strconv.FormatInt(dur.Nanoseconds()/int64(base), 10)
		if _, err := dst.Write(valueOf(s, dashValue)); err != nil {
			return errors.Wrap(err, `failed to write request time since epoch`)
		}
		return nil
	})
}

func makeRequestTimeFracSinceEpoch(base time.Duration) FormatWriter {
	return FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
		dur := ctx.RequestTime().Sub(epoch)

		s := fmt.Sprintf("%g", float64(dur.Nanoseconds()%int64(base*1000))/float64(base))
		if _, err := dst.Write(valueOf(s, dashValue)); err != nil {
			return errors.Wrap(err, `failed to write request time since epoch`)
		}
		return nil
	})
}

func makeElapsedTime(base time.Duration, fraction int) FormatWriter {
	return FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
		var str string
		if elapsed := ctx.ElapsedTime(); elapsed > 0 {
			switch fraction {
			case timeNotFraction:
				str = strconv.Itoa(int(elapsed / base))
			case timeMicroFraction:
				str = fmt.Sprintf("%03d", int((elapsed%time.Millisecond)/base))
			case timeMilliFraction:
				str = fmt.Sprintf("%03d", int((elapsed%time.Second)/base))
			default:
				return errors.New("failed to write elapsed time")
			}
		}
		if _, err := dst.Write(valueOf(str, dashValue)); err != nil {
			return errors.Wrap(err, "failed to write elapsed time")
		}
		return nil
	})
}

const (
	timeNotFraction = iota
	timeMicroFraction
	timeMilliFraction
)

var (
	elapsedTimeMicroSeconds               = makeElapsedTime(time.Microsecond, timeNotFraction)
	elapsedTimeMilliSeconds               = makeElapsedTime(time.Millisecond, timeNotFraction)
	elapsedTimeMicroSecondsFrac           = makeElapsedTime(time.Microsecond, timeMicroFraction)
	elapsedTimeMilliSecondsFrac           = makeElapsedTime(time.Millisecond, timeMilliFraction)
	elapsedTimeSeconds                    = makeElapsedTime(time.Second, timeNotFraction)
	requestTimeMicrosecondsFracSinceEpoch = makeRequestTimeFracSinceEpoch(time.Microsecond)
	requestTimeMillisecondsFracSinceEpoch = makeRequestTimeFracSinceEpoch(time.Millisecond)
	requestTimeSecondsSinceEpoch          = makeRequestTimeSinceEpoch(time.Second)
	requestTimeMillisecondsSinceEpoch     = makeRequestTimeSinceEpoch(time.Millisecond)
	requestTimeMicrosecondsSinceEpoch     = makeRequestTimeSinceEpoch(time.Microsecond)
)

var requestHttpMethod = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	v := valueOf(ctx.Request().Method, emptyValue)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request method")
	}
	return nil
})

var requestHttpProto = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	v := valueOf(ctx.Request().Proto, emptyValue)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request HTTP request proto")
	}
	return nil
})

var requestRemoteAddr = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	addr := ctx.Request().RemoteAddr
	if i := strings.IndexByte(addr, ':'); i > -1 {
		addr = addr[:i]
	}
	v := valueOf(addr, dashValue)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request remote address")
	}
	return nil
})

var pid = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	v := valueOf(strconv.Itoa(os.Getpid()), emptyValue)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write pid")
	}
	return nil
})

var rawQuery = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	q := ctx.Request().URL.RawQuery
	if q != "" {
		q = "?" + q
	}
	v := valueOf(q, emptyValue)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write raw request query")
	}
	return nil
})

var requestLine = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	buf := getLogBuffer()
	defer releaseLogBuffer(buf)

	r := ctx.Request()
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

var httpStatus = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	var s string
	if st := ctx.ResponseStatus(); st != 0 { // can't really happen, but why not
		s = strconv.Itoa(st)
	}
	if _, err := dst.Write(valueOf(s, emptyValue)); err != nil {
		return errors.Wrap(err, "failed to write response status")
	}
	return nil
})

var requestTime = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	v := valueOf(ctx.RequestTime().Format("[02/Jan/2006:15:04:05 -0700]"), []byte{'[', ']'})
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request time")
	}
	return nil
})

var urlPath = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	v := valueOf(ctx.Request().URL.Path, emptyValue)
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request URL path")
	}
	return nil
})

var username = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	var v = dashValue
	if u := ctx.Request().URL.User; u != nil {
		v = valueOf(u.Username(), dashValue)
	}
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write username")
	}
	return nil
})

var requestHost = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	var v []byte
	h := ctx.Request().Host
	if strings.IndexByte(h, ':') > 0 {
		v = valueOf(strings.Split(ctx.Request().Host, ":")[0], dashValue)
	} else {
		v = valueOf(h, dashValue)
	}
	if _, err := dst.Write(v); err != nil {
		return errors.Wrap(err, "failed to write request host")
	}
	return nil
})

var responseContentLength = FormatWriteFunc(func(dst io.Writer, ctx LogCtx) error {
	var s string
	if cl := ctx.ResponseContentLength(); cl != 0 {
		s = strconv.FormatInt(cl, 10)
	}
	_, err := dst.Write(valueOf(s, dashValue))
	return err
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
			cbs = append(cbs, responseContentLength)
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
			cbs = append(cbs, fixedByteSequence(dashValue))
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
				case 't':
					// The time, in the form given by format, which should be in an
					// extended strftime(3) format (potentially localized). If the
					// format starts with begin: (default) the time is taken at the
					// beginning of the request processing. If it starts with end:
					// it is the time when the log entry gets written, close to the
					// end of the request processing. In addition to the formats
					// supported by strftime(3), the following format tokens are
					// supported:
					//
					// sec	number of seconds since the Epoch
					// msec	number of milliseconds since the Epoch
					// usec	number of microseconds since the Epoch
					// msec_frac	millisecond fraction
					// usec_frac	microsecond fraction
					//
					// These tokens can not be combined with each other or strftime(3)
					// formatting in the same format string. You can use multiple
					// %{format}t tokens instead.
					formatter, err := timeFormatter(key)
					if err != nil {
						return err
					}
					cbs = append(cbs, formatter)
				default:
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

func (f *Format) WriteTo(dst io.Writer, ctx LogCtx) error {
	for _, w := range f.writers {
		if err := w.WriteTo(dst, ctx); err != nil {
			return errors.Wrap(err, "failed to execute FormatWriter")
		}
	}
	return nil
}
