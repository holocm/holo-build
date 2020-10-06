default: build/holo-build build/man/holo-build.8

VERSION := $(shell ./util/find_version.sh)
GO_BUILDFLAGS = -mod vendor

env:
	@env

build/%: FORCE
	go build $(GO_BUILDFLAGS) -ldflags "-s -w -X github.com/holocm/holo-build/src/holo-build/common.version=$(VERSION)" -o build/$* ./src/$*

# manpages are generated using pod2man (which comes with Perl and therefore
# should be readily available on almost every Unix system)
build/man/%: doc/%.pod
	@mkdir -p build/man
	pod2man --name="$(shell echo $* | cut -d. -f1)" --section=$(shell echo $* | cut -d. -f2) \
		--center="Configuration Management" --release="holo-build $(VERSION)" \
		$< $@

GO_ALLPKGS := $(shell go list ./...)

test: check # just a synonym
check: default build/dump-package
	@if ! hash golint 2>/dev/null; then printf "\e[1;36m>> Installing golint...\e[0m\n"; go get -u golang.org/x/lint/golint; fi
	@printf "\e[1;36m>> gofmt\e[0m\n"
	@if s="$$(gofmt -s -l *.go */*.go 2>/dev/null)" && test -n "$$s"; then printf ' => %s\n%s\n' gofmt  "$$s"; false; fi
	@printf "\e[1;36m>> golint\e[0m\n"
	@if s="$$(golint $(GO_ALLPKGS) 2>/dev/null)"    && test -n "$$s"; then printf ' => %s\n%s\n' golint "$$s"; false; fi
	@printf "\e[1;36m>> go vet\e[0m\n"
	@go vet $(GO_ALLPKGS)
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

.PHONY: test check install vendor FORCE
