package apachelog

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/lestrrat-go/apache-logformat/internal/httputil"
	"github.com/lestrrat-go/apache-logformat/internal/logctx"
	"github.com/stretchr/testify/assert"
	"net/http/httptest"
)

func isDash(t *testing.T, s string) bool {
	return assert.Equal(t, "-", s, "expected dash")
}

func isEmpty(t *testing.T, s string) bool {
	return assert.Equal(t, "", s, "expected dash")
}

func TestInternalDashEmpty(t *testing.T) {
	f := func(t *testing.T, name string, dash bool, f FormatWriter) {
		var buf bytes.Buffer
		r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		r.Host = ""
		r.Method = ""
		r.Proto = ""
		ctx := logctx.Get(r)

		t.Run(fmt.Sprintf("%s (dash=%t)", name, dash), func(t *testing.T) {
			if !assert.NoError(t, f.WriteTo(&buf, ctx), "callback should succeed") {
				return
			}
			if dash {
				isDash(t, buf.String())
			} else {
				isEmpty(t, buf.String())
			}
		})
	}

	type dashEmptyCase struct {
		Name   string
		Dash   bool
		Format FormatWriter
	}
	cases := []dashEmptyCase{
		{Name: "Request Header", Dash: true, Format: requestHeader("foo")},
		{Name: "Response Header", Dash: true, Format: responseHeader("foo")},
		{Name: "Request Method", Dash: false, Format: requestHttpMethod},
		{Name: "Request Proto", Dash: false, Format: requestHttpProto},
		{Name: "Request RemoteAddr", Dash: true, Format: requestRemoteAddr},
		{Name: "Request Raw Query", Dash: false, Format: rawQuery},
		{Name: "Response Status", Dash: false, Format: httpStatus},
		{Name: "Request Username", Dash: true, Format: username},
		{Name: "Request Host", Dash: true, Format: requestHost},
		{Name: "Response ContentLength", Dash: true, Format: responseContentLength},
	}

	for _, c := range cases {
		f(t, c.Name, c.Dash, c.Format)
	}
}

func TestResponseWriterDefaultStatusCode(t *testing.T) {
	writer := httptest.NewRecorder()
	uut := httputil.GetResponseWriter(writer)
	if uut.StatusCode() != http.StatusOK {
		t.Fail()
	}
}

func TestFlusherInterface(t *testing.T) {
	var rw httputil.ResponseWriter
	var f http.Flusher = &rw
	_ = f
}

func TestFlusher(t *testing.T) {
	lines := []string{
		"Hello, World!",
		"Hello again, World!",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, line := range lines {
			if _, err := w.Write([]byte(line)); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(time.Microsecond)
		}
	})

	s := httptest.NewServer(CommonLog.Wrap(handler, ioutil.Discard))
	defer s.Close()

	req, err := http.NewRequest("GET", s.URL, nil)
	if !assert.NoError(t, err, "request creation should succeed") {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if !assert.NoError(t, err, "GET should succeed") {
		return
	}
	defer resp.Body.Close()

	buf := make([]byte, 64)
	var i int
	for {
		n, err := resp.Body.Read(buf)
		if n == 0 {
			if !assert.Equal(t, len(lines)-1, i-1, "wrong number of chunks") {
				return
			}
			break
		}
		t.Logf("Response body %d: %d %s", i, n, buf)
		if !assert.Equal(t, []byte(lines[i]), buf[:n], "wrong response body") {
			return
		}
		if err == io.EOF {
			if !assert.Equal(t, len(lines)-1, i, "wrong number of chunks") {
				return
			}
			break
		}
		if !assert.NoError(t, err, "Body read should succeed") {
			return
		}
		i++
	}
}
