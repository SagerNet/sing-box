NAME = sing-box
COMMIT = $(shell git rev-parse --short HEAD)
TAGS ?= with_gvisor,with_quic,with_wireguard,with_utls,with_reality_server,with_clash_api
TAGS_TEST ?= with_gvisor,with_quic,with_wireguard,with_grpc,with_ech,with_utls,with_reality_server,with_shadowsocksr
PARAMS = -v -trimpath -tags "$(TAGS)" -ldflags "-s -w -buildid="
MAIN = ./cmd/sing-box

.PHONY: test release

build:
	go build $(PARAMS) $(MAIN)

install:
	go install $(PARAMS) $(MAIN)

fmt:
	@gofumpt -l -w .
	@gofmt -s -w .
	@gci write --custom-order -s "standard,prefix(github.com/sagernet/),default" .

fmt_install:
	go install -v mvdan.cc/gofumpt@latest
	go install -v github.com/daixiang0/gci@latest

lint:
	GOOS=linux golangci-lint run ./...
	GOOS=android golangci-lint run ./...
	GOOS=windows golangci-lint run ./...
	GOOS=darwin golangci-lint run ./...
	GOOS=freebsd golangci-lint run ./...

lint_install:
	go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@latest

proto:
	@go run ./cmd/internal/protogen
	@gofumpt -l -w .
	@gofumpt -l -w .

proto_install:
	go install -v google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install -v google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

snapshot:
	go run ./cmd/internal/build goreleaser release --rm-dist --snapshot || exit 1
	mkdir dist/release
	mv dist/*.tar.gz dist/*.zip dist/*.deb dist/*.rpm dist/release
	ghr --delete --draft --prerelease -p 1 nightly dist/release
	rm -r dist

release:
	go run ./cmd/internal/build goreleaser release --rm-dist --skip-publish || exit 1
	mkdir dist/release
	mv dist/*.tar.gz dist/*.zip dist/*.deb dist/*.rpm dist/release
	ghr --delete --draft --prerelease -p 3 $(shell git describe --tags) dist/release
	rm -r dist

release_install:
	go install -v github.com/goreleaser/goreleaser@latest
	go install -v github.com/tcnksm/ghr@latest

test:
	@go test -v ./... && \
	cd test && \
	go mod tidy && \
	go test -v -tags "$(TAGS_TEST)" .

test_stdio:
	@go test -v ./... && \
	cd test && \
	go mod tidy && \
	go test -v -tags "$(TAGS_TEST),force_stdio" .

lib:
	go run ./cmd/internal/build_libbox

lib_install:
	go get -v -d
	go install -v github.com/sagernet/gomobile/cmd/gomobile@v0.0.0-20221130124640-349ebaa752ca
	go install -v github.com/sagernet/gomobile/cmd/gobind@v0.0.0-20221130124640-349ebaa752ca

clean:
	rm -rf bin dist sing-box
	rm -f $(shell go env GOPATH)/sing-box

update:
	git fetch
	git reset FETCH_HEAD --hard
	git clean -fdx