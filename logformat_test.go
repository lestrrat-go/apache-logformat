package apachelog

import (
  "net/http"
  "os"
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

func TestAllTypes(t *testing.T) {
  l := NewApacheLog(os.Stderr, "This should be a verbatim percent sign -> %%")
  output := l.Format(
    &http.Request{},
    200,
    http.Header{},
    0,
  )

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

func TestElpasedTime (t *testing.T) {
  l := NewApacheLog(os.Stderr, "%T %D")
  output := l.Format(&http.Request{}, 200, http.Header{}, 1 * time.Second)
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

func BenchmarkReplaceLoop(t *testing.B) {
  l := CombinedLog
  r, err := http.NewRequest("GET", "http://golang.org", nil)
  if err != nil {
    t.Errorf("Failed to create request: %s", err)
  }
  r.RemoteAddr = "127.0.0.1"
  r.Header.Set("User-Agent", "Apache-LogFormat Port In Golang")
  r.Header.Set("Referer", "http://dummy.com")

  for i := 0; i < 100000; i ++ {
    output := l.FormatLoop(
      r,
      200,
      http.Header{ "Content-Type": []string{"text/plain"} },
      1500000,
    )
    if output == "" {
      t.Errorf("Failed to Format")
    }
  }
}

func BenchmarkReplaceRegexp(t *testing.B) {
  l := CombinedLog
  r, err := http.NewRequest("GET", "http://golang.org", nil)
  if err != nil {
    t.Errorf("Failed to create request: %s", err)
  }
  r.RemoteAddr = "127.0.0.1"
  r.Header.Set("User-Agent", "Apache-LogFormat Port In Golang")
  r.Header.Set("Referer", "http://dummy.com")

  for i := 0; i < 100000; i++ {
    output := l.FormatRegexp(
      r,
      200,
      http.Header{ "Content-Type": []string{"text/plain"} },
      1500000,
    )
    if output == "" {
      t.Errorf("Failed to Format")
    }
  }
}
