go-apache-logformat
===================

[![Build Status](https://travis-ci.org/lestrrat/go-apache-logformat.png?branch=master)](https://travis-ci.org/lestrrat/go-apache-logformat)

Port of Perl5's [Apache::LogFormat::Compiler](https://metacpan.org/release/Apache-LogFormat-Compiler) to golang

To install, simply issue a `go get`:

```
go get github.com/lestrrat/go-apache-logformat
```


```go
import(
    "net/http"
    "github.com/lestrrat/go-apache-logformat"
)

var logger *apachelog.NewRequest = apachelog.CombinedLog

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // You need to save status from somewhere, as you can't
    // get the status code from ResponseWriter
    defer logger.LogLine(req, status, w.Header())
}
```
