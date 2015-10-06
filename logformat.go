package apachelog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

/*
 * import("github.com/lestrrat/go-apache-logformat")
 * l := apachelog.CombinedLog
 * l.LogLine(req)
 */

// This is the pool used to store the byte.Buffer used as the temporary
// storage for log line
var bufferPool = sync.Pool{}

// This is the pool used to store replaceContext
var ctxPool = sync.Pool{}

func init() {
	bufferPool.New = func() interface{} {
		return &bytes.Buffer{}
	}
	ctxPool.New = func() interface{} {
		return &replaceContext{}
	}
}

type replaceContext struct {
	request    *http.Request
	reqtime    time.Duration
	respStatus int
	respHdrs   http.Header
}

func (c replaceContext) ElapsedTime() time.Duration {
	return c.reqtime
}
func (c replaceContext) Request() *http.Request {
	return c.request
}
func (c replaceContext) Status() int {
	return c.respStatus
}
func (c replaceContext) Header() http.Header {
	return c.respHdrs
}

// Combined is a pre-defined ApacheLog struct to log "common" log format
var CommonLog = NewApacheLog(os.Stderr, `%h %l %u %t "%r" %>s %b`)

// Combined is a pre-defined ApacheLog struct to log "combined" log format
var CombinedLog = NewApacheLog(os.Stderr, `%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-agent}i"`)

func NewApacheLog(w io.Writer, fmt string) *ApacheLog {
	return &ApacheLog{
		logger: w,
		format: fmt,
	}
}

/*
 * Create a new ApacheLog struct with same args as the target.
 * This is useful if you want to create an identical logger
 * but with a different output:
 *
 *    mylog := apachelog.CombinedLog.Clone()
 *    mylog.SetOutput(myOutput)
 *
 */
func (al *ApacheLog) Clone() *ApacheLog {
	return NewApacheLog(al.logger, al.format)
}

/*
 * SetOutput() can be used to send the output of LogLine to somewhere other
 * than os.Stderr
 */
func (al *ApacheLog) SetOutput(w io.Writer) {
	al.logger = w
}

func (al *ApacheLog) Output() io.Writer {
	return al.logger
}

/*
 * r is http.Request from client. status is the response status code.
 * respHeader is an http.Header of the response.
 *
 * reqtime is optional, and denotes the time taken to serve the request
 *
 */
func (al *ApacheLog) LogLine(
	r *http.Request,
	status int,
	respHeader http.Header,
	reqtime time.Duration,
) error {
	if err := al.Format(al.logger, r, status, respHeader, reqtime); err != nil {
		return err
	}
	al.logger.Write([]byte{'\n'})
	return nil
}

func defaultAppend(start *int, i *int, b *bytes.Buffer, str string) {
	b.WriteString(str)
	defaultAdvance(start, i)
}
func defaultAdvance(start *int, i *int) {
	*start = *i + 2
	*i = *i + 1
}

func (al *ApacheLog) FormatString(
	r *http.Request,
	status int,
	respHeader http.Header,
	reqtime time.Duration,
) (string, error) {
	b := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(b)

	b.Reset()

	if err := al.Format(b, r, status, respHeader, reqtime); err != nil {
		return "", err
	}
	return b.String(), nil
}

var (
	ErrAlreadyCompiled     = errors.New("pattern already compiled")
	ErrInvalidRuneSequence = errors.New("invalid rune sequence found in format")
	ErrUnimplemented       = errors.New("pattern unimplemented")
)

type fixedByteArraySequence []byte

func (f fixedByteArraySequence) Logit(out io.Writer, c Context) error {
	_, err := out.Write(f)
	return err
}

var emptyValue = []byte{'-'}

func valueOf(s string) []byte {
	if s == "" {
		return emptyValue
	}
	return []byte(s)
}

type header string

func (h header) Logit(out io.Writer, c Context) error {
	_, err := out.Write(valueOf(c.Request().Header.Get(string(h))))
	return err
}

type responseHeader string

func (h responseHeader) Logit(out io.Writer, c Context) error {
	_, err := out.Write(valueOf(c.Header().Get(string(h))))
	return err
}

func elapsedTimeMicroSeconds(out io.Writer, c Context) error {
	var str string
	if elapsed := c.ElapsedTime(); elapsed > 0 {
		str = strconv.Itoa(int(elapsed / time.Microsecond))
	}
	_, err := out.Write(valueOf(str))
	return err
}
func elapsedTimeSeconds(out io.Writer, c Context) error {
	var str string
	if elapsed := c.ElapsedTime(); elapsed > 0 {
		str = strconv.Itoa(int(elapsed / time.Second))
	}
	_, err := out.Write(valueOf(str))
	return err
}
func httpProto(out io.Writer, c Context) error {
	_, err := out.Write(valueOf(c.Request().Proto))
	return err
}
func remoteAddr(out io.Writer, c Context) error {
	_, err := out.Write(valueOf(c.Request().RemoteAddr))
	return err
}
func httpMethod(out io.Writer, c Context) error {
	_, err := out.Write(valueOf(c.Request().Method))
	return err
}
func pid(out io.Writer, c Context) error {
	_, err := out.Write([]byte(strconv.Itoa(os.Getpid())))
	return err
}
func rawQuery(out io.Writer, c Context) error {
	q := c.Request().URL.RawQuery
	if q != "" {
		q = "?" + q
	}
	out.Write(valueOf(q))
	return nil
}
func requestLine(out io.Writer, c Context) error {
	r := c.Request()
	_, err := io.WriteString(
		out,
		fmt.Sprintf("%s %s %s",
			r.Method,
			r.URL,
			r.Proto,
		),
	)
	return err
}
func httpStatus(out io.Writer, c Context) error {
	_, err := io.WriteString(
		out,
		strconv.Itoa(c.Status()),
	)
	return err
}
func requestTime(out io.Writer, c Context) error {
	_, err := io.WriteString(
		out,
		time.Now().Format("02/Jan/2006:15:04:05 -0700"),
	)
	return err
}
func urlPath(out io.Writer, c Context) error {
	_, err := out.Write(valueOf(c.Request().URL.Path))
	return err
}

func username(out io.Writer, c Context) error {
	u := c.Request().URL.User
	var name string
	if u != nil {
		name = u.Username()
	}

	_, err := out.Write(valueOf(name))
	return err
}
func requestHost(out io.Writer, c Context) error {
	host := strings.Split(c.Request().Host, ":")[0]
	_, err := out.Write(valueOf(host))
	return err
}

type callback func(io.Writer, Context) error
type callbacks []callback

func (cs callbacks) Logit(out io.Writer, c Context) error {
	for _, cb := range cs {
		if err := cb(out, c); err != nil {
			return err
		}
	}
	return nil
}

// Compile checks the given format string, and creates a
// function that can be invoked to create the formatted line. Once
// compiled, the result can be used to format repeatedly
func Compile(f string) (callback, error) {
	cbs := callbacks{}
	start := 0
	max := len(f)

	for i := 0; i < max; {
		r, n := utf8.DecodeRuneInString(f[i:])
		if r == utf8.RuneError {
			return nil, ErrInvalidRuneSequence
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
				cbs = append(cbs, fixedByteArraySequence(f[start:i]).Logit)
				start = i
				break
			}
			cbs = append(cbs, fixedByteArraySequence(f[start:i-1]).Logit)
		}

		// Find what we have next.

		r, n = utf8.DecodeRuneInString(f[i:])
		if r == utf8.RuneError {
			return nil, ErrInvalidRuneSequence
		}
		i += n

		switch r {
		case '%':
			cbs = append(cbs, fixedByteArraySequence([]byte{'%'}).Logit)
			start = i + n - 1
		case 'b':
			cbs = append(cbs, header("Content-Length").Logit)
			start = i + n - 1
		case 'D': // custom
			cbs = append(cbs, elapsedTimeMicroSeconds)
			start = i + n - 1
		case 'h':
			cbs = append(cbs, remoteAddr)
			start = i + n - 1
		case 'H':
			cbs = append(cbs, httpProto)
			start = i + n - 1
		case 'l':
			cbs = append(cbs, fixedByteArraySequence(emptyValue).Logit)
			start = i + n - 1
		case 'm':
			cbs = append(cbs, httpMethod)
			start = i + n - 1
		case 'p':
			cbs = append(cbs, pid)
			start = i + n - 1
		case 'P':
			// Unimplemented
			return nil, ErrUnimplemented
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
			if len(f) >= i && f[i] == 's' {
				// "Last" status doesn't exist in our case, so it's the same as %s
				cbs = append(cbs, httpStatus)
				start = i + 1
				i = i + 1
			} else {
				// Otherwise we don't know what this is. just do a verbatim copy
				cbs = append(cbs, fixedByteArraySequence([]byte{'%', '>'}).Logit)
				start = i + n - 1
			}
		case '{':
			// Search the next }
			end := -1
			for j := i; j < max; j++ {
				if f[j] == '}' {
					end = j
					break
				}
			}

			if end != -1 && end < max-1 { // Found it!
				// check for suffix
				blockType := f[end+1]
				key := f[i:end]
				switch blockType {
				case 'i':
					cbs = append(cbs, header(key).Logit)
				case 'o':
					cbs = append(cbs, responseHeader(key).Logit)
				default: // case 't':
					return nil, ErrUnimplemented
				}

				start = end + 2
				i = end + 1
			} else {
				cbs = append(cbs, fixedByteArraySequence([]byte{'%', '{'}).Logit)
				start = i + n - 1
			}
		}
	}

	if start < max {
		cbs = append(cbs, fixedByteArraySequence(f[start:max]).Logit)
	}
	return cbs.Logit, nil
}

func (al *ApacheLog) Compile() error {
	if al.compiled != nil {
		return ErrAlreadyCompiled
	}

	c, err := Compile(al.format)
	if err != nil {
		return err
	}
	al.compiled = c
	return nil
}

// FormatCtxt creates the log line using the given Context
func (al *ApacheLog) FormatCtx(out io.Writer, ctx Context) error {
	if al.compiled == nil {
		if err := al.Compile(); err != nil {
			return err
		}
	}

	if err := al.compiled(out, ctx); err != nil {
		return err
	}
	return nil
}

/*
 * Format() creates the log line to be used in LogLine()
 */
func (al *ApacheLog) Format(
	out io.Writer,
	r *http.Request,
	status int,
	respHeader http.Header,
	reqtime time.Duration,
) error {
	ctx := ctxPool.Get().(*replaceContext)
	defer ctxPool.Put(ctx)

	ctx.respStatus = status
	ctx.respHdrs = respHeader
	ctx.request = r
	ctx.reqtime = reqtime
	return al.FormatCtx(out, ctx)
}
