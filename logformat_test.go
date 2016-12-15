package apachelog_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lestrrat/go-apache-logformat"
	"github.com/stretchr/testify/assert"
)

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
		w.Write([]byte("Hello, World!"))
	}), &out)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	t.Logf("output = %s", strconv.Quote(out.String()))
}

func TestVerbatim(t *testing.T) {
	l, err := apachelog.New("This should be a verbatim percent sign -> %%")
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, "This should be a verbatim percent sign -> %\n", b.String(), "output should match") {
		return
	}
	t.Logf("%s", b.String())
}

func TestResponseHeader(t *testing.T) {
	l, err := apachelog.New("%{X-Req-Header}i %{X-Resp-Header}o")
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	r, err := http.NewRequest("GET", "http://golang.org", nil)
	if !assert.NoError(t, err, "request creation should succeed") {
		return
	}

	r.Header.Set("X-Req-Header", "Gimme a response!")

	var b bytes.Buffer
	var c apachelog.LogCtx

	c.Request = r
	c.ResponseHeader = http.Header{}
	c.ResponseHeader.Add("X-Resp-Header", "Here's your response")

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, "Gimme a response! Here's your response\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestQuery(t *testing.T) {
	l, err := apachelog.New(`%m %U %q %H`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	r, err := http.NewRequest("GET", "http://golang.org/foo?bar=baz", nil)
	if !assert.NoError(t, err, "request creation should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx
	c.Request = r

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, "GET /foo ?bar=baz HTTP/1.1\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestElpasedTime(t *testing.T) {
	l, err := apachelog.New(`%T %D %{sec}t %{msec}t %{usec}t`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	r, err := http.NewRequest("GET", "http://golang.org", nil)
	if !assert.NoError(t, err, "request creation should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx
	c.Request = r
	c.ElapsedTime = time.Second

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, "1 1000000 1 1000 1000000\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestElpasedTimeFraction(t *testing.T) {
	l, err := apachelog.New(`%T.%{msec_frac}t%{usec_frac}t`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	r, err := http.NewRequest("GET", "http://golang.org", nil)
	if !assert.NoError(t, err, "request creation should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx
	c.Request = r
	c.ElapsedTime = time.Second + time.Millisecond*200 + time.Microsecond*90

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, "1.200090\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestStrayPercent(t *testing.T) {
	l, err := apachelog.New(`stray percent at the end: %`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, "stray percent at the end: %\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestMissingClosingBrace(t *testing.T) {
	l, err := apachelog.New(`Missing closing brace: %{Test <- this should be verbatim`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, "Missing closing brace: %{Test <- this should be verbatim\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestPercentS(t *testing.T) {
	// %s and %>s should be the same in our case
	l, err := apachelog.New(`%s = %>s`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx
	c.ResponseStatus = http.StatusNotFound

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, "404 = 404\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestPid(t *testing.T) {
	// pid
	l, err := apachelog.New(`%p`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, strconv.Itoa(os.Getpid())+"\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestUnknownAfterPecentGreaterThan(t *testing.T) {
	// %> followed by unknown char
	l, err := apachelog.New(`%>X should be verbatim`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, `%>X should be verbatim`+"\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestFixedSequence(t *testing.T) {
	l, err := apachelog.New(`hello, world!`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	var b bytes.Buffer
	var c apachelog.LogCtx

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Equal(t, `hello, world!`+"\n", b.String()) {
		return
	}
	t.Logf("%s", b.String())
}

func TestFull(t *testing.T) {
	l, err := apachelog.New(`hello, %% %b %D %h %H %l %m %p %q %r %s %t %T %u %U %v %V %>s %{X-LogFormat-Test}i %{X-LogFormat-Test}o world!`)
	if !assert.NoError(t, err, "apachelog.New should succeed") {
		return
	}

	r, err := http.NewRequest("GET", "http://golang.org", nil)
	if !assert.NoError(t, err, "request creation should succeed") {
		return
	}

	r.Header.Add("Content-Length", "8192")
	r.Header.Add("X-LogFormat-Test", "Hello, Request!")
	r.RemoteAddr = "192.168.11.1"
	r.Host = "example.com"
	r.URL = &url.URL{
		Host:     "example.com",
		Path:     "/hello_world",
		RawQuery: "hello=world",
	}

	var b bytes.Buffer
	var c apachelog.LogCtx
	c.Request = r
	c.ElapsedTime = 5 * time.Second
	c.ResponseStatus = http.StatusBadRequest
	c.ResponseHeader = http.Header{}
	c.ResponseHeader.Set("X-LogFormat-Test", "Hello, Response!")

	if !assert.NoError(t, l.WriteLog(&b, &c), "WriteLog should succeed") {
		return
	}

	if !assert.Regexp(t, `^hello, % 8192 5000000 192\.168\.11\.1 HTTP/1\.1 - GET \d+ \?hello=world GET //example\.com/hello_world\?hello=world HTTP/1\.1 400 \[\d{2}/[a-zA-Z]+/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}\] 5 - /hello_world example\.com example\.com 400 Hello, Request! Hello, Response! world!\n$`, b.String(), "Log line must match") {
		return
	}
	t.Logf("%s", b.String())
}
