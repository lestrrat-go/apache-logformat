package apachelog

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

/*
 * import("github.com/lestrrat/go-apache-logformat")
 * l := apachelog.CombinedLog
 * l.LogLine(req)
 */

type ApacheLog struct {
	logger  io.Writer
	format  string
	context *replaceContext
}

type replaceContext struct {
	request    *http.Request
	status     int
	respHeader http.Header
	reqtime    time.Duration
}

var CommonLog = NewApacheLog(
	os.Stderr,
	`%h %l %u %t "%r" %>s %b`,
)

// Combined is a pre-defined ApacheLog struct to log "combined" log format
var CombinedLog = NewApacheLog(
	os.Stderr,
	`%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-agent}i"`,
)

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
) {
	al.logger.Write(al.Format(r, status, respHeader, reqtime))
	al.logger.Write([]byte{'\n'})
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
) string {
	return string(al.Format(r, status, respHeader, reqtime))
}

/*
 * Format() creates the log line to be used in LogLine()
 */
func (al *ApacheLog) Format(
	r *http.Request,
	status int,
	respHeader http.Header,
	reqtime time.Duration,
) []byte {
	al.context = &replaceContext{
		r,
		status,
		respHeader,
		reqtime,
	}

	f := al.format
	b := &bytes.Buffer{}
	max := len(f)
	start := 0

	for i := 0; i < max; i++ {
		c := f[i]
		if c != '%' {
			continue
		}

		// Add to buffer everything we found so far
		if start != i {
			b.WriteString(f[start:i])
		}

		if i >= max-1 {
			start = i
			break
		}

		n := f[i+1]
		switch n {
		case '%':
			defaultAppend(&start, &i, b, "%")
		case 'b':
			defaultAppend(&start, &i, b, nilOrString(r.Header.Get("Content-Length")))
		case 'h':
			defaultAppend(&start, &i, b, nilOrString(r.RemoteAddr))
		case 'l':
			defaultAppend(&start, &i, b, nilField)
		case 'm':
			defaultAppend(&start, &i, b, r.Method)
		case 'p':
			defaultAppend(&start, &i, b, fmt.Sprintf("%d", os.Getpid()))
		case 'q':
			q := r.URL.RawQuery
			if q != "" {
				q = fmt.Sprintf("?%s", q)
			}
			defaultAppend(&start, &i, b, nilOrString(q))
		case 'r':
			defaultAppend(&start, &i, b, fmt.Sprintf("%s %s %s",
				r.Method,
				r.URL,
				r.Proto,
			))
		case 's':
			defaultAppend(&start, &i, b, fmt.Sprintf("%d", al.context.status))
		case 't':
			defaultAppend(&start, &i, b, time.Now().Format("02/Jan/2006:15:04:05 -0700"))
		case 'u':
			u := r.URL.User
			var name string
			if u != nil {
				name = u.Username()
			}

			defaultAppend(&start, &i, b, nilOrString(name))
		case 'v', 'V':
			host := r.URL.Host
			i := strings.Index(host, ":")
			if i > -1 {
				host = host[0:i]
			}
			defaultAppend(&start, &i, b, nilOrString(host))
		case '>':
			if f[i+2] == 's' {
				// "Last" status doesn't exist in our case, so it's the same as %s
				b.WriteString(fmt.Sprintf("%d", al.context.status))
				start = i + 3
				i = i + 2
			} else {
				// Otherwise we don't know what this is.
				start = i
			}
		case 'D': // custom
			var str string
			if al.context.reqtime > 0 {
				str = fmt.Sprintf("%d", al.context.reqtime/time.Microsecond)
			}
			defaultAppend(&start, &i, b, nilOrString(str))
		case 'H':
			defaultAppend(&start, &i, b, r.Proto)
		case 'P':
			// Unimplemented
		case 'T': // custom
			var str string
			if al.context.reqtime > 0 {
				str = fmt.Sprintf("%d", al.context.reqtime/time.Second)
			}
			defaultAppend(&start, &i, b, nilOrString(str))
		case 'U':
			defaultAppend(&start, &i, b, r.URL.Path)
		case '{':
			// Search the next }
			end := -1
			for j := i + 1; j < max; j++ {
				if f[j] == '}' {
					end = j
					break
				}
			}

			if end != -1 && end < max-1 { // Found it!
				// check for suffix
				blockType := f[end+1]
				key := f[i+2 : end]
				switch blockType {
				case 'i':
					b.WriteString(nilOrString(r.Header.Get(key)))
				case 'o':
					b.WriteString(nilOrString(al.context.respHeader.Get(key)))
				case 't':
					// XX Unimplmented
				}

				start = end + 2
				i = end + 1
			} else {
				start = i
				i = i + 1
			}
		}
	}

	if start < max {
		b.WriteString(f[start:max])
	}

	return b.Bytes()
}

var nilField = "-"

func nilOrString(v string) string {
	if v == "" {
		return nilField
	}
	return v
}
