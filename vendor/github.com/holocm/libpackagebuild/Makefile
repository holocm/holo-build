help:
	@printf "Available targets:\n\n\tmake test\n\n"

PKG = github.com/holocm/libpackagebuild
GO_ALLPKGS := $(shell go list $(PKG)/...)

test:
	@if ! hash golint 2>/dev/null; then printf "\e[1;36m>> Installing golint...\e[0m\n"; go get -u golang.org/x/lint/golint; fi
	@printf "\e[1;36m>> gofmt\e[0m\n"
	@if s="$$(gofmt -s -l *.go */*.go 2>/dev/null)"                            && test -n "$$s"; then printf ' => %s\n%s\n' gofmt  "$$s"; false; fi
	@printf "\e[1;36m>> golint\e[0m\n"
	@if s="$$(golint $(GO_ALLPKGS) 2>/dev/null)" && test -n "$$s"; then printf ' => %s\n%s\n' golint "$$s"; false; fi
	@printf "\e[1;36m>> go vet\e[0m\n"
	@go vet $(GO_ALLPKGS)
	@printf "\e[1;36m>> go test\e[0m\n"
	@go test $(GO_ALLPKGS)
	@printf "\e[1;32m>> All tests successful.\e[0m\n"
