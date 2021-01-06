package apachelog_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	apachelog "github.com/lestrrat-go/apache-logformat/v2"
	"github.com/lestrrat-go/apache-logformat/v2/internal/logctx"
	strftime "github.com/lestrrat-go/strftime"
	"github.com/stretchr/testify/assert"
)

const message = "Hello, World!"

var hello = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, message)
})

func TestBasic(t *testing.T) {
	r, err := http.NewRequest("GET", "http://golang.org", nil)
	if err != nil {
		t.Errorf("Failed to create request: %s", err)
	}
	r.RemoteAddr = "127.0.0.1"
	r.Header.Set("User-Agent", "Apache-LogFormat Port In Golang")
	r.Header.Set("Referer", "http://dummy.com")

	var out bytes.Buffer
	h := apachelog.CombinedLog.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello, World!"))
	}), &out)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	t.Logf("output = %s", strconv.Quote(out.String()))
}

func newServer(l *apachelog.ApacheLog, h http.Handler, out io.Writer) *httptest.Server {
	return httptest.NewServer(l.Wrap(h, out))
}

func testLog(t *testing.T, pattern, expected string, h http.Handler, modifyURL func(string) string, modifyRequest func(*http.Request)) {
	l, err := apachelog.New(pattern)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	var buf bytes.Buffer
	s := newServer(l, h, &buf)
	defer s.Close()

	u := s.URL
	if modifyURL != nil {
		u = modifyURL(u)
	}

	r, err := http.NewRequest("GET", u, nil)
	if !assert.NoError(t, err, "request creation should succeed") {
		return
	}

	if modifyRequest != nil {
		modifyRequest(r)
	}

	_, err = http.DefaultClient.Do(r)
	if !assert.NoError(t, err, "GET should succeed") {
		return
	}

	if !assert.Equal(t, expected, buf.String()) {
		return
	}
}

func TestVerbatim(t *testing.T) {
	testLog(t,
		"This should be a verbatim percent sign -> %%",
		"This should be a verbatim percent sign -> %\n",
		hello,
		nil,
		nil,
	)
}

func TestResponseHeader(t *testing.T) {
	testLog(t,
		"%{X-Req-Header}i %{X-Resp-Header}o",
		"Gimme a response! Here's your response\n",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Resp-Header", "Here's your response")
		}),
		nil,
		func(r *http.Request) {
			r.Header.Set("X-Req-Header", "Gimme a response!")
		},
	)
}

func TestQuery(t *testing.T) {
	testLog(t,
		`%m %U %q %H`,
		"GET /foo ?bar=baz HTTP/1.1\n",
		hello,
		func(u string) string {
			return u + "/foo?bar=baz"
		},
		nil,
	)
}

func TestTime(t *testing.T) {
	o := logctx.Clock
	defer func() { logctx.Clock = o }()

	const longTimeAgo = 233431200 * time.Second
	const pattern = `%Y-%m-%d`

	f, _ := strftime.New(pattern)
	cl := clock.NewMock()
	cl.Add(longTimeAgo)
	logctx.Clock = cl

	// Mental note: %{[mu]?sec}t should (milli|micro)?seconds since the epoch.
	testLog(t,
		fmt.Sprintf(
			`%%T %%D %%{sec}t %%{msec}t %%{usec}t %%{begin:%s}t %%{end:%s}t %%{%s}t`,
			pattern,
			pattern,
			pattern,
		),
		fmt.Sprintf(
			"1 1000000 %d %d %d %s %s %s\n",
			longTimeAgo/time.Second,
			longTimeAgo/time.Millisecond,
			longTimeAgo/time.Microsecond,
			f.FormatString(cl.Now()),
			f.FormatString(cl.Now().Add(time.Second)),
			f.FormatString(cl.Now()),
		),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cl.Add(time.Second)
		}),
		nil,
		nil,
	)
}

func TestElapsed(t *testing.T) {
	o := logctx.Clock
	defer func() { logctx.Clock = o }()
	cl := clock.NewMock()
	logctx.Clock = cl
	testLog(t, `%T %D %{s}T %{us}T %{ms}T`,
		"3 3141592 3 3141592 3141\n",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cl.Add(3141592 * time.Microsecond)
		}),
		nil,
		nil,
	)

	_, err := apachelog.New(`%{h}T`)
	assert.EqualError(t, err, "failed to compile log format: unrecognised elapsed time unit: h")
}

func TestElapsedTimeFraction(t *testing.T) {
	o := logctx.Clock
	defer func() { logctx.Clock = o }()

	cl := clock.NewMock()
	cl.Add(time.Second + time.Millisecond*200 + time.Microsecond*90)
	logctx.Clock = cl
	testLog(t,
		`%{msec_frac}t %{usec_frac}t`,
		"200.09 90\n",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		nil,
		nil,
	)
}

func TestStrayPercent(t *testing.T) {
	testLog(t,
		`stray percent at the end: %`,
		"stray percent at the end: %\n",
		hello,
		nil,
		nil,
	)
}

func TestMissingClosingBrace(t *testing.T) {
	testLog(t,
		`Missing closing brace: %{Test <- this should be verbatim`,
		"Missing closing brace: %{Test <- this should be verbatim\n",
		hello,
		nil,
		nil,
	)
}

func TestPercentS(t *testing.T) {
	// %s and %>s should be the same in our case
	testLog(t,
		`%s = %>s`,
		"404 = 404\n",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
		nil,
		nil,
	)
}

func TestPid(t *testing.T) {
	testLog(t,
		`%p`, // pid
		strconv.Itoa(os.Getpid())+"\n",
		hello,
		nil,
		nil,
	)
}

func TestUnknownAfterPecentGreaterThan(t *testing.T) {
	testLog(t,
		`%>X should be verbatim`, // %> followed by unknown char
		`%>X should be verbatim`+"\n",
		hello,
		nil,
		nil,
	)
}

func TestFixedSequence(t *testing.T) {
	testLog(t,
		`hello, world!`,
		"hello, world!\n",
		hello,
		nil,
		nil,
	)
}

func TestFull(t *testing.T) {
	l, err := apachelog.New(`hello, %% %b %D %h %H %l %m %p %q %r %s %t %T %{ms}T %u %U %v %V %>s %{X-LogFormat-Test}i %{X-LogFormat-Test}o world!`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	o := logctx.Clock
	defer func() { logctx.Clock = o }()

	cl := clock.NewMock()
	logctx.Clock = cl
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cl.Add(5 * time.Second)
		w.Header().Set("X-LogFormat-Test", "Hello, Response!")
		w.WriteHeader(http.StatusBadRequest)
	})
	var buf bytes.Buffer
	s := newServer(l, h, &buf)
	defer s.Close()

	r, err := http.NewRequest("GET", s.URL+"/hello_world?hello=world", nil)
	if !assert.NoError(t, err, "request creation should succeed") {
		return
	}

	r.Header.Add("X-LogFormat-Test", "Hello, Request!")

	_, err = http.DefaultClient.Do(r)
	if !assert.NoError(t, err, "GET should succeed") {
		return
	}

	if !assert.Regexp(t, `^hello, % - 5000000 127\.0\.0\.1 HTTP/1\.1 - GET \d+ \?hello=world GET /hello_world\?hello=world HTTP/1\.1 400 \[\d{2}/[a-zA-Z]+/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}\] 5 5000 - /hello_world 127\.0\.0\.1 127\.0\.0\.1 400 Hello, Request! Hello, Response! world!\n$`, buf.String(), "Log line must match") {
		return
	}
	t.Logf("%s", buf.String())
}

func TestPercentB(t *testing.T) {
	testLog(t,
		`%b`,
		fmt.Sprintf("%d\n", len(message)),
		hello,
		nil,
		nil,
	)
}

func TestIPv6RemoteAddr(t *testing.T) {
	format := `%h`
	expected := "[::1]\n"

	al, err := apachelog.New(format)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	ctx := &Context{
		request: &http.Request{
			RemoteAddr: "[::1]:51111",
		},
	}

	var buf bytes.Buffer
	_ = al.WriteLog(&buf, ctx)

	if !assert.Equal(t, expected, buf.String()) {
		return
	}
}

func TestEnvironmentVariable(t *testing.T) {
	// Well.... let's see. I don't want to change the user's env var,
	// so let's just scan for something already present in the environment variable list

	for _, v := range os.Environ() {
		vs := strings.SplitN(v, "=", 2)

		t.Logf("Testing environment variable %s", vs[0])
		al, err := apachelog.New(fmt.Sprintf(`%%{%s}e`, vs[0]))
		if !assert.NoError(t, err, "apachelog.New should succeed") {
			return
		}

		var ctx Context
		var buf bytes.Buffer
		_ = al.WriteLog(&buf, &ctx)

		var expected = "-"
		if vs[1] != "" {
			expected = vs[1]
		}
		// Be careful, the log line has a trailing new line
		expected = expected + "\n"

		if !assert.Equal(t, expected, buf.String()) {
			return
		}
	}
}

type Context struct {
	elapsedTime           time.Duration
	request               *http.Request
	requestTime           time.Time
	responseContentLength int64
	responseHeader        http.Header
	responseStatus        int
	responseTime          time.Time
}

func (ctx *Context) ElapsedTime() time.Duration {
	return ctx.elapsedTime
}

func (ctx *Context) Request() *http.Request {
	return ctx.request
}

func (ctx *Context) RequestTime() time.Time {
	return ctx.requestTime
}

func (ctx *Context) ResponseContentLength() int64 {
	return ctx.responseContentLength
}

func (ctx *Context) ResponseHeader() http.Header {
	return ctx.responseHeader
}

func (ctx *Context) ResponseStatus() int {
	return ctx.responseStatus
}

func (ctx *Context) ResponseTime() time.Time {
	return ctx.responseTime
}
