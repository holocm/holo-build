# holo-build - cross-distribution system package compiler

[![Build Status](https://travis-ci.org/holocm/holo-build.svg?branch=master)](https://travis-ci.org/holocm/holo-build)

[Holo](https://github.com/holocm/holo) relies on system packages to deploy
configuration files and install applications. Distributions offer tooling to
build such packages, but most of the time, these tools impose an unnecessary
overhead when the goal is just to package up a few static files and list some
dependencies. holo-build provides a simple, distribution-independent package
description language and generates a system package from such a description.
Supported package formats include dpkg, pacman and RPM.

    [package]
    name     = "hologram-systemd-timesyncd"
    version  = "1.0"
    author   = "Jane Doe <jane.doe@example.org>"
    requires = ["systemd"]

    [[file]]
    path     = "/etc/systemd/timesyncd.conf.d/server.conf"
    content  = """
        [Time]
        NTP=ntp.someserver.local
    """

    [[symlink]]
    # as created by `systemctl enable systemd-timesyncd`
    path     = "/etc/systemd/system/sysinit.target.wants/systemd-timesyncd.service"
    target   = "/usr/lib/systemd/system/systemd-timesyncd.service"

    [[action]]
    on     = "setup"
    script = "systemctl daemon-reload && systemctl start systemd-timesyncd"

    [[action]]
    on     = "cleanup"
    script = "systemctl stop systemd-timesyncd"

## Installation

It is recommended to install `holo-build` as a package.
The [website](http://holocm.org) lists distributions that have a holo-build
package available.

holo-build requires [Go](https://golang.org) and [Perl](https://perl.org) as
build-time dependencies. There are no runtime dependencies other than a libc.
Once you're all set, the build is done with

```
make
make check
sudo make install
```

## Documentation

User documentation is available in [man page form](doc/holo-build.8.pod).

For further information, visit [holocm.org](http://holocm.org).
