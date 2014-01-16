package apachelog

import (
  "bytes"
  "fmt"
  "io"
  "os"
  "net/http"
  "strings"
  "time"
  "regexp"
)

/*
 * import("github.com/lestrrat/go-apache-logformat")
 * l := apachelog.CombinedLog
 * l.LogLine(req)
 */

type ApacheLog struct {
  logger io.Writer
  format string
  context *replaceContext
}

type replaceContext struct {
  request     *http.Request
  status      int
  respHeader  http.Header
  reqtime     time.Duration
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
  return &ApacheLog {
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
func (self *ApacheLog) Clone() *ApacheLog {
  return NewApacheLog(self.logger, self.format)
}

/*
 * SetOutput() can be used to send the output of LogLine to somewhere other 
 * than os.Stderr
 */
func (self *ApacheLog) SetOutput(w io.Writer) {
  self.logger = w
}

/*
 * r is http.Request from client. status is the response status code.
 * respHeader is an http.Header of the response.
 *
 * reqtime is optional, and denotes the time taken to serve the request
 *
 */
func (self *ApacheLog) LogLine(
  r           *http.Request,
  status      int,
  respHeader  http.Header,
  reqtime     time.Duration,
) {
  self.logger.Write([]byte(self.Format(r, status, respHeader, reqtime) + "\n"))
}

var percentReplacer = regexp.MustCompile(
  `(?:\%\{(.+?)\}([a-zA-Z])|\%(?:[<>])?([a-zA-Z\%]))`,
)

/*
 * Format() creates the log line to be used in LogLine()
 */
func (self *ApacheLog) Format(
  r           *http.Request,
  status      int,
  respHeader  http.Header,
  reqtime     time.Duration,
) (string) {
  return self.FormatLoop(r, status, respHeader, reqtime)
}

func (self *ApacheLog) FormatLoop(
  r           *http.Request,
  status      int,
  respHeader  http.Header,
  reqtime     time.Duration,
) (string) {
  self.context = &replaceContext {
    r,
    status,
    respHeader,
    reqtime,
  }
  return self.replaceLoop()
}

func (self *ApacheLog) FormatRegexp(
  r           *http.Request,
  status      int,
  respHeader  http.Header,
  reqtime     time.Duration,
) (string) {
  self.context = &replaceContext {
    r,
    status,
    respHeader,
    reqtime,
  }
  return percentReplacer.ReplaceAllStringFunc(
    self.format,
    self.replaceFunc,
  )
}

func (self *ApacheLog) replaceLoop() string {
  f := self.format
  r := self.context.request
  b := &bytes.Buffer {}
  max   := len(f)
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

    if i >= max - 1 {
      start = i
      break
    }

    n := f[i + 1]
    switch (n) {
    case '%':
      b.WriteByte('%')
      start = i + 2
      i = i + 1
    case 'b':
      b.WriteString(nilOrString(r.Header.Get("Content-Length")))
      start = i + 2
      i = i + 1
    case 'q':
      q := r.URL.RawQuery
      if q != "" {
        b.WriteString(fmt.Sprintf("?%s", q))
      }
      start = i + 2
      i = i + 1
    case 'm':
      b.WriteString(r.Method)
      start = i + 2
      i = i + 1
    case 'D': // custom
      if self.context.reqtime > 0 {
        b.WriteString(fmt.Sprintf("%d", self.context.reqtime / time.Microsecond))
      }
      start = i + 2
      i = i + 1
    case 'H':
      b.WriteString(r.Proto)
      start = i + 2
      i = i + 1
    case 'T': // custom
      if self.context.reqtime > 0 {
        b.WriteString(fmt.Sprintf("%d", self.context.reqtime / time.Second))
      }
      start = i + 2
      i = i + 1
    case 'U':
      b.WriteString(r.URL.Path)
      start = i + 2
      i = i + 1
    case '{':
      // Search the next }
      end := -1
      for j := i + 1; j < max; j++ {
        if f[j] == '}' {
          end = j
          break
        }
      }

      if end != -1 && end < max - 1 { // Found it!
        // check for suffix
        blockType := f[end+1]
        key       := f[i + 2:end]
        switch (blockType) {
        case 'i':
          b.WriteString(nilOrString(r.Header.Get(key)))
        case 'o':
          b.WriteString(nilOrString(self.context.respHeader.Get(key)))
        case 't':
          // XX Unimplmented
        }

        start = end + 2
        i     = end + 1
      } else {
        start = i 
        i     = i + 1
      }
    }
  }

  if start < max {
    b.WriteString(f[start:max])
  }

  return string(b.Bytes())
}

var nilField string = "-"
func nilOrString(v string) string {
  if v == "" {
    return nilField
  } else {
    return v
  }
}

func (self *ApacheLog) replaceFunc (match string) string {
  r := self.context.request
  switch string(match) {
  case "%%":
    return "%"
  case "%b":
    return nilOrString(r.Header.Get("Content-Length"))
  case "%m":
    return r.Method
  case "%h":
    clientIP := r.RemoteAddr
    if clientIP == "" {
      return nilField
    }
    if colon := strings.LastIndex(clientIP, ":"); colon != -1 {
      clientIP = clientIP[:colon]
    }
    return clientIP
  case "%l":
    return nilField
  case "%q":
    q := r.URL.RawQuery
    if q != "" {
      return fmt.Sprintf("?%s", q)
    }
    return q
  case "%r":
    return fmt.Sprintf("%s %s %s",
      r.Method,
      r.URL,
      r.Proto,
    )
  case "%s", "%>s": // > doesn't mean anything here
    return fmt.Sprintf("%d", self.context.status)
  case "%t":
    return time.Now().Format("02/Jan/2006:15:04:05 -0700")
  case "%u":
    u := r.URL.User
    if u != nil {
      if name := u.Username(); name != "" {
        return name
      }
    }
    return nilField
  case "%D": // custom
    if self.context.reqtime > 0 {
      return fmt.Sprintf("%d", self.context.reqtime / time.Microsecond)
    } else {
      return ""
    }
  case "%H":
    return r.Proto
  case "%T": // custom
    if self.context.reqtime > 0 {
      return fmt.Sprintf("%d", self.context.reqtime / time.Second)
    } else {
      return ""
    }
  case "%U":
    return r.URL.Path
  default:
    // if the second character isn't "{" at this point, we don't
    // know what the f this is. just return it
    if match[1] != '{' {
      return match
    }

    match = strings.TrimPrefix(match, "%{")

    var blockType byte
    // check the last character of this pattern "}i"
    for _, t := range []byte { 'i', 'o', 't' } {
      if match[len(match) - 1] == t {
        match = strings.TrimSuffix(match, fmt.Sprintf("}%c", t))
        blockType = t
        break
      }
    }

    switch blockType {
    case 'i':
      return nilOrString(r.Header.Get(match))
    case 'o':
      return nilOrString(self.context.respHeader.Get(match))
    case 't':
      // XX Unimplmented
    }
  }
  return ""
}