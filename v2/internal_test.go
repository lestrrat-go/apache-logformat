package apachelog

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"net/http/httptest"

	"github.com/lestrrat-go/apache-logformat/v2/internal/logctx"
	"github.com/stretchr/testify/assert"
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

func TestFlusher(t *testing.T) {
	lines := []string{
		"Hello, World!",
		"Hello again, World!",
	}

	// We need to synchronize the reads with the writes of each chunk
	sync := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, line := range lines {
			if _, err := w.Write([]byte(line)); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			sync <- struct{}{}
		}
		close(sync)
	})

	s := httptest.NewServer(CommonLog.Wrap(handler, ioutil.Discard))
	defer s.Close()

	req, err := http.NewRequest("GET", s.URL, nil)
	if !assert.NoError(t, err, "request creation should succeed") {
		return
	}

	// If it isn't flushing properly and sending chunks, then the call to
	// Do() will hang waiting for the entire body, while the handler will be
	// stuck after having written the first line.  So have an explicit
	// timeout for the read.
	timer := time.NewTimer(time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := http.DefaultClient.Do(req)
		if !assert.NoError(t, err, "GET should succeed") {
			return
		}
		defer resp.Body.Close()

		buf := make([]byte, 64)
		var i int
		for {
			_, ok := <-sync
			if !ok {
				break
			}
			n, err := resp.Body.Read(buf)
			if n == 0 {
				if !assert.Equal(t, len(lines)-1, i-1, "wrong number of chunks") {
					return
				}
				break
			}
			t.Logf("Response body %d: %d %s", i, n, buf[:n])
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
	}()

	select {
	case <-timer.C:
		close(sync)
		t.Fatal("timed out: not flushing properly?")
	case <-done:
	}
}
