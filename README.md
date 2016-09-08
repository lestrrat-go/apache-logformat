go-apache-logformat
===================

[![Build Status](https://travis-ci.org/lestrrat/go-apache-logformat.png?branch=master)](https://travis-ci.org/lestrrat/go-apache-logformat)

[![Coverage Status](https://coveralls.io/repos/lestrrat/go-apache-logformat/badge.png?branch=topic%2Fgoveralls)](https://coveralls.io/r/lestrrat/go-apache-logformat?branch=topic%2Fgoveralls)

# SYNOPSYS

```go
import (
  "net/http"
  "os"

  "github.com/lestrrat/go-apache-logformat"
)

func main() {
  var s http.ServeMux
  s.HandleFunc("/", handleIndex)
  s.HandleFunc("/foo", handleFoo)

  http.ListenAndServe(":8080", apachelog.CombinedLog.Wrap(s, os.Stderr))
}
```

# DESCRIPTION

This is a port of Perl5's [Apache::LogFormat::Compiler](https://metacpan.org/release/Apache-LogFormat-Compiler) to golang
