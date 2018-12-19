# libpackagebuild

[![GoDoc](https://godoc.org/github.com/holocm/libpackagebuild?status.svg)](https://godoc.org/github.com/holocm/libpackagebuild)

This [Go](https://golang.org) library generates packages that can be installed by a system package manager. Supported formats include:

- dpkg (used by Debian and Ubuntu)
- pacman (used by Arch Linux)
- RPM (used by Suse, Redhat, Fedora, Mageia; _experimental support only_)

To add support for a new format, implement the `Generator` interface and submit a pull request.

## Example

Error handling elided for brevity.

```go
import (
  build "github.com/holocm/libpackagebuild"
  "github.com/holocm/libpackagebuild/debian"
  "github.com/holocm/libpackagebuild/filesystem"
)

pkg := build.Package {
  Name: "my-console-configuration",
  Version: "1.0",
  Release: 1,
  Description: "just a quick example",
  Requires: []build.PackageRelation{
    {
      RelatedPackage: "linux",
      Constraints: []VersionConstraint{
        { Relation: ">=", Version: "4.14.5" },
      },
    },
  },
  FSRoot: filesystem.NewDirectory(),
}

err := pkg.InsertFSNode("/etc/vconsole.conf",
  filesystem.RegularFile { Content: "KEYMAP=us\n" },
)
err := pkg.InsertFSNode("/etc/systemd/system/mdmonitor.service",
  filesystem.Symlink { Target: "/dev/null" },
)

generator := debian.GeneratorFactory(pkg)
errs := generator.Validate()

fmt.Println(generator.RecommendedFileName())
  // output: "my-console-configuration_1.0-1_all.deb"

bytes, err := generator.Build()
  // `bytes` contains the resulting package as a bytestring
```

TODO Travis CI
