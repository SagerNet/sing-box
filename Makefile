NAME = sing-box
COMMIT = $(shell git rev-parse --short HEAD)
TAGS ?= with_quic,with_clash_api
PARAMS = -v -trimpath -tags '$(TAGS)' -ldflags \
		'-X "github.com/sagernet/sing-box/constant.Commit=$(COMMIT)" \
		-w -s -buildid='
MAIN = ./cmd/sing-box

.PHONY: test release

build:
	go build $(PARAMS) $(MAIN)

install:
	go install $(PARAMS) $(MAIN)

fmt:
	@gofumpt -l -w .
	@gofmt -s -w .
	@gci write -s "standard,prefix(github.com/sagernet/),default" .

fmt_install:
	go install -v mvdan.cc/gofumpt@latest
	go install -v github.com/daixiang0/gci@v0.4.0

lint:
	GOOS=linux golangci-lint run ./...
	GOOS=windows golangci-lint run ./...
	GOOS=darwin golangci-lint run ./...
	GOOS=freebsd golangci-lint run ./...

lint_install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

test:
	@go test -v . && \
	@pushd test && \
	@go test -v . && \
	@popd

clean:
	rm -rf bin dist
	rm -f $(shell go env GOPATH)/sing-box

update:
	git fetch
	git reset FETCH_HEAD --hard
	git clean -fdx