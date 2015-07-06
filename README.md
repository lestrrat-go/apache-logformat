go-apache-logformat
===================

[![Build Status](https://travis-ci.org/lestrrat/go-apache-logformat.png?branch=master)](https://travis-ci.org/lestrrat/go-apache-logformat)

[![Coverage Status](https://coveralls.io/repos/lestrrat/go-apache-logformat/badge.png?branch=topic%2Fgoveralls)](https://coveralls.io/r/lestrrat/go-apache-logformat?branch=topic%2Fgoveralls)

Port of Perl5's [Apache::LogFormat::Compiler](https://metacpan.org/release/Apache-LogFormat-Compiler) to golang

To install, simply issue a `go get`:

```
go get github.com/lestrrat/go-apache-logformat
```

To use the logger alone:

```go
import(
    "github.com/lestrrat/go-apache-logformat"
)

logger := apachelog.NewApacheLog(os.Stderr, "....")
logger.LogLine(...)
```

If you just want the Apache combined log format, you can use
the predefined struct:

```go
import(
    "github.com/lestrrat/go-apache-logformat"
)

logger := apachelog.CombinedLog.Clone()
```

If you want to change where the logger emits the log to,
you can either specify it in the `NewApacheLog` function or
use the `SetOutput` method

```go
file, err := os.OpenFile(...)
logger.SetOutput(file)
```

To wrap an existing handler, you can use the LoggingWriter:

```go
import(
    "net/http"
    "github.com/lestrrat/go-apache-logformat"
)

func Start() {
    http.ListenAndServe(
        ":8080",
        apachelog.WrapLoggingWriter(ServeHTTP, logger),
    )
}

var logger *apachelog.ApacheLog = apachelog.CombinedLog
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ...
}

```

To do more fine tuning, embed it in your app:

```go
import(
    "net/http"
    "github.com/lestrrat/go-apache-logformat"
)

var logger *apachelog.ApacheLog = apachelog.CombinedLog
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
    lw := apachelog.NewLoggingWriter(w, r, logger)
    defer lw.EmitLog()
}
```
