default: prepare-build
default: build/holo-build build/man/holo-build.8

VERSION := $(shell ./util/find_version.sh)
# force people to use golangvend
GOCC := env GOPATH=$(CURDIR)/.gopath GOBIN=$(CURDIR)/build go

env:
	@env

prepare-build:
	@mkdir -p build/man
build/holo-build: FORCE
	$(GOCC) install --ldflags "-s -w -X github.com/holocm/holo-build/src/holo-build/common.version=$(VERSION)" github.com/holocm/holo-build/src/holo-build
build/dump-package: FORCE
	$(GOCC) install --ldflags "-s -w" github.com/holocm/holo-build/src/dump-package

# manpages are generated using pod2man (which comes with Perl and therefore
# should be readily available on almost every Unix system)
build/man/%: doc/%.pod
	pod2man --name="$(shell echo $* | cut -d. -f1)" --section=$(shell echo $* | cut -d. -f2) \
		--center="Configuration Management" --release="holo-build $(VERSION)" \
		$< $@

test: check # just a synonym
check: default build/dump-package
	@bash test/compiler/run_tests.sh
	@bash test/interface/run_tests.sh

install: default src/holo-build.sh util/autocomplete.bash util/autocomplete.zsh
	install -D -m 0755 src/holo-build.sh      "$(DESTDIR)/usr/bin/holo-build"
	install -D -m 0755 build/holo-build       "$(DESTDIR)/usr/lib/holo/holo-build"
	install -D -m 0644 build/man/holo-build.8 "$(DESTDIR)/usr/share/man/man8/holo-build.8"
	install -D -m 0644 util/autocomplete.bash "$(DESTDIR)/usr/share/bash-completion/completions/holo-build"
	install -D -m 0644 util/autocomplete.zsh  "$(DESTDIR)/usr/share/zsh/site-functions/_holo-build"

vendor: FORCE
	$(GOCC) mod tidy
	$(GOCC) mod vendor

.PHONY: prepare-build test check install vendor FORCE
