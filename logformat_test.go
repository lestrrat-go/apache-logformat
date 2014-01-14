package apachelog

import (
  "net/http"
  "os"
  "testing"
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
  output := l.Format(
    r,
    200,
    http.Header{ "Content-Type": []string{"text/plain"} },
    1500000,
  )
  if output == "" {
    t.Errorf("Failed to Format")
  }
  t.Logf(`output = "%s"`, output)
}

func TestResponseHeader(t *testing.T) {
  l := NewApacheLog(os.Stderr, "%{X-Req-Header}i %{X-Resp-Header}o")
  r, err := http.NewRequest("GET", "http://golang.org", nil)
  if err != nil {
    t.Errorf("Failed to create request: %s", err)
  }

  r.Header.Set("X-Req-Header", "Gimme a response!")

  output := l.Format(r, 200, http.Header{"X-Resp-Header": []string{"Here's your response"}}, 1000000)
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

  output := l.Format(r, 200, http.Header{}, 1000000)
  if output != "GET /foo ?bar=baz HTTP/1.1" {
    t.Errorf("output '%s' did not match", output)
  }
  t.Logf("%s", output)
}