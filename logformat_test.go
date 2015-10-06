package apachelog

import (
	"bytes"
	"fmt"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"testing"
	"time"
)

func TestBasic(t *testing.T) {
	l := CombinedLog
	r, err := http.NewRequest("GET", "http://golang.org", nil)
	if err != nil {
		t.Errorf("Failed to create request: %s", err)
	}
	r.RemoteAddr = "127.0.0.1"
	r.Header.Set("User-Agent", "Apache-LogFormat Port In Golang")
	r.Header.Set("Referer", "http://dummy.com")
	output, err := l.FormatString(
		r,
		200,
		http.Header{"Content-Type": []string{"text/plain"}},
		1500000,
	)
	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}
	if output == "" {
		t.Errorf("Failed to Format")
		return
	}
	t.Logf(`output = "%s"`, output)
}

func TestAllTypes(t *testing.T) {
	l := NewApacheLog(os.Stderr, "This should be a verbatim percent sign -> %%")
	output, err := l.FormatString(
		&http.Request{},
		200,
		http.Header{},
		0,
	)

	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}

	if output != "This should be a verbatim percent sign -> %" {
		t.Errorf("Failed to format. Got '%s'", output)
	}
}

func TestResponseHeader(t *testing.T) {
	l := NewApacheLog(os.Stderr, "%{X-Req-Header}i %{X-Resp-Header}o")
	r, err := http.NewRequest("GET", "http://golang.org", nil)
	if err != nil {
		t.Errorf("Failed to create request: %s", err)
	}

	r.Header.Set("X-Req-Header", "Gimme a response!")

	output, err := l.FormatString(r, 200, http.Header{"X-Resp-Header": []string{"Here's your response"}}, 1000000)

	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}

	if output != "Gimme a response! Here's your response" {
		t.Errorf("output '%s' did not match", output)
	}
	t.Logf("%s", output)
}

func TestQuery(t *testing.T) {
	l := NewApacheLog(os.Stderr, "%m %U %q %H")
	r, err := http.NewRequest("GET", "http://golang.org/foo?bar=baz", nil)
	if err != nil {
		t.Errorf("Failed to create request: %s", err)
	}

	output, err := l.FormatString(r, 200, http.Header{}, 1000000)

	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}

	if output != "GET /foo ?bar=baz HTTP/1.1" {
		t.Errorf("output '%s' did not match", output)
	}
	t.Logf("%s", output)
}

func TestElpasedTime(t *testing.T) {
	l := NewApacheLog(os.Stderr, "%T %D")
	output, err := l.FormatString(&http.Request{}, 200, http.Header{}, 1*time.Second)

	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}

	if output != "1 1000000" {
		t.Errorf("output '%s' did not match", output)
	}
	t.Logf("%s", output)
}

func TestClone(t *testing.T) {
	l := CombinedLog.Clone()
	l.SetOutput(os.Stdout)

	if CombinedLog.logger == l.logger {
		t.Errorf("logger struct must not be the same")
	}
}

func TestEdgeCase(t *testing.T) {
	// stray %
	l := NewApacheLog(os.Stderr, "stray percent at the end: %")
	output, err := l.FormatString(
		&http.Request{},
		200,
		http.Header{},
		0,
	)

	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}

	if output != "stray percent at the end: %" {
		t.Errorf("Failed to match output")
		t.Logf("Expected '%s', got '%s'", "stray percent at the end %", output)
	}

	// %{...} with missing }
	l = NewApacheLog(os.Stderr, "Missing closing brace: %{Test <- this should be verbatim")
	r, _ := http.NewRequest("GET", "http://golang.com", nil)
	r.Header.Set("Test", "Test Me Test Me")
	output, err = l.FormatString(
		r,
		200,
		http.Header{},
		0,
	)

	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}

	if output != "Missing closing brace: %{Test <- this should be verbatim" {
		t.Errorf("Failed to match output")
		t.Logf("Exepected '%s', got '%s'",
			"Missing closing brace: %{Test <- this should be verbatim",
			output,
		)
	}

	// %s and %>s should be the same in our case
	l = NewApacheLog(os.Stderr, "%s = %>s")
	output, err = l.FormatString(
		r,
		404,
		http.Header{},
		0,
	)

	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}

	if output != "404 = 404" {
		t.Errorf("%%s and %%>s should be the same. Expected '404 = 404', got '%s'", output)
	}

	// pid
	l = NewApacheLog(os.Stderr, "%p")
	output, err = l.FormatString(r, 200, http.Header{}, 0)
	if output != fmt.Sprintf("%d", os.Getpid()) {
		t.Errorf("%%p should get us our own pid. Expected '%d', got '%s'", os.Getpid(), output)
	}

	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}

	// %> followed by unknown char
	l = NewApacheLog(os.Stderr, "%>X should be verbatim")
	output, err = l.FormatString(
		r,
		200,
		http.Header{},
		0,
	)

	if err != nil {
		t.Errorf("Error while formatting: %s", err)
		return
	}

	if output != "%>X should be verbatim" {
		t.Errorf("%%>X should be verbatim: Expected '%%>X should be verbatim', got '%s'", output)
	}
}

func TestCompileAllFixedSequence(t *testing.T) {
	pat, err := Compile("hello, world!")
	if err != nil {
		t.Errorf("Failed to compile: %s", err)
		return
	}

	b := &bytes.Buffer{}
	pat(b, nil)
	if b.String() != "hello, world!" {
		t.Errorf("bad formatting")
	}
}

type dummyResponse struct {
	hdrs   http.Header
	status int
}

func (r dummyResponse) Header() http.Header {
	return r.hdrs
}
func (r dummyResponse) Status() int {
	return r.status
}

type dummyCtx struct {
	req     *http.Request
	res     Response
	elapsed time.Duration
}

func (d dummyCtx) ElapsedTime() time.Duration {
	return d.elapsed
}
func (d dummyCtx) Request() *http.Request {
	return d.req
}
func (d dummyCtx) Response() Response {
	return d.res
}

func TestCompile(t *testing.T) {
	pat, err := Compile("hello, %% %b %D %h %H %l %m %p %q %r %s %t %T %u %U %v %V %>s %{X-LogFormat-Test}i %{X-LogFormat-Test}o world!")
	if err != nil {
		t.Errorf("Failed to compile: %s", err)
		return
	}

	b := &bytes.Buffer{}
	pat(b, dummyCtx{
		elapsed: 5 * time.Second,
		req: &http.Request{
			Header: http.Header{
				textproto.CanonicalMIMEHeaderKey("Content-Length"):   []string{"8192"},
				textproto.CanonicalMIMEHeaderKey("X-LogFormat-Test"): []string{"Hello, Request!"},
			},
			Method:     "GET",
			Proto:      "HTTP/1.1",
			RemoteAddr: "192.168.11.1",
			Host:       "example.com",
			URL: &url.URL{
				Host:     "example.com",
				Path:     "/hello_world",
				RawQuery: "hello=world",
			},
		},
		res: &dummyResponse{
			hdrs: http.Header{
				textproto.CanonicalMIMEHeaderKey("X-LogFormat-Test"): []string{"Hello, Response!"},
			},
			status: 400,
		},
	})

	re := regexp.MustCompile(`^hello, % 8192 5000000 192\.168\.11\.1 HTTP/1\.1 - GET \d+ \?hello=world GET //example\.com/hello_world\?hello=world HTTP/1\.1 400 \d{2}/[a-zA-Z]+/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4} 5 - /hello_world example\.com example\.com 400 Hello, Request! Hello, Response! world!$`)

	if !re.Match(b.Bytes()) {
		t.Errorf("output did not match regexp")
		t.Logf("output: %s", b.String())
		t.Logf("regexp: %s", re)
		return
	}
}
