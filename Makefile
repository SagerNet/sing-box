NAME=sing-box
COMMIT=$(shell git rev-parse --short HEAD)
PARAMS=-trimpath -tags '$(TAGS)' -ldflags \
		'-X "github.com/sagernet/sing-box/constant.Commit=$(COMMIT)" \
		-w -s -buildid='
MAIN=./cmd/sing-box

.PHONY: test release

build:
	go build $(PARAMS) $(MAIN)

action_version: build
	echo "::set-output name=VERSION::`./sing-box version -n`"

install:
	go install $(PARAMS) $(MAIN)

release:
	goreleaser release --snapshot --rm-dist

fmt_install:
	go install -v mvdan.cc/gofumpt@latest
	go install -v github.com/daixiang0/gci@v0.4.0

fmt:
	gofumpt -l -w .
	gofmt -s -w .
	gci write -s "standard,prefix(github.com/sagernet/),default" .

lint_install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

lint:
	GOOS=linux golangci-lint run ./...
	GOOS=windows golangci-lint run ./...
	GOOS=darwin golangci-lint run ./...
	GOOS=freebsd golangci-lint run ./...

test:
	go test -v . && \
	pushd test && \
	go test -v . && \
	popd

clean:
	rm -rf bin dist
	rm -f $(shell go env GOPATH)/sing-box

update:
	git fetch
	git reset FETCH_HEAD --hard
	git clean -fdx