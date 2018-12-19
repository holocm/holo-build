# libpackagebuild

[![GoDoc](https://godoc.org/github.com/holocm/libpackagebuild?status.svg)](https://godoc.org/github.com/holocm/libpackagebuild)

This [Go](https://golang.org) library generates packages that can be installed by a system package manager. Supported formats include:

- dpkg (used by Debian and Ubuntu)
- pacman (used by Arch Linux)
- RPM (used by Suse, Redhat, Fedora, Mageia; _experimental support only_)

To add support for a new format, implement the `Generator` interface and submit a pull request.

## Example

```go
import (
  build "github.com/holocm/libpackagebuild"
)
```

TODO finish
TODO Makefile (gofmt, golint, govet, go test)
TODO Travis CI
