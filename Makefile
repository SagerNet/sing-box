NAME = sing-box
COMMIT = $(shell git rev-parse --short HEAD)
TAGS_GO118 = with_gvisor,with_dhcp,with_wireguard,with_utls,with_reality_server,with_clash_api
TAGS_GO120 = with_quic,with_ech
TAGS ?= $(TAGS_GO118),$(TAGS_GO120)
TAGS_TEST ?= with_gvisor,with_quic,with_wireguard,with_grpc,with_ech,with_utls,with_reality_server,with_shadowsocksr

GOHOSTOS = $(shell go env GOHOSTOS)
GOHOSTARCH = $(shell go env GOHOSTARCH)
VERSION=$(shell CGO_ENABLED=0 GOOS=$(GOHOSTOS) GOARCH=$(GOHOSTARCH) go run ./cmd/internal/read_tag)

PARAMS = -v -trimpath -ldflags "-X 'github.com/sagernet/sing-box/constant.Version=$(VERSION)' -s -w -buildid="
MAIN_PARAMS = $(PARAMS) -tags $(TAGS)
MAIN = ./cmd/sing-box
PREFIX ?= $(shell go env GOPATH)

.PHONY: test release

build:
	go build $(MAIN_PARAMS) $(MAIN)

ci_build_go118:
	go build $(PARAMS) $(MAIN)
	go build $(PARAMS) -tags "$(TAGS_GO118)" $(MAIN)

ci_build:
	go build $(PARAMS) $(MAIN)
	go build $(MAIN_PARAMS) $(MAIN)

install:
	go build -o $(PREFIX)/bin/$(NAME) $(PARAMS) $(MAIN)

fmt:
	@gofumpt -l -w .
	@gofmt -s -w .
	@gci write --custom-order -s standard -s "prefix(github.com/sagernet/)" -s "default" .

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

release:
	go run ./cmd/internal/build goreleaser release --clean --skip-publish || exit 1
	mkdir dist/release
	mv dist/*.tar.gz dist/*.zip dist/*.deb dist/*.rpm dist/release
	ghr --replace --draft --prerelease -p 3 "v${VERSION}" dist/release
	rm -r dist

release_install:
	go install -v github.com/goreleaser/goreleaser@latest
	go install -v github.com/tcnksm/ghr@latest

update_android_version:
	go run ./cmd/internal/update_android_version

build_android:
	cd ../sing-box-for-android && ./gradlew :app:assembleRelease

upload_android:
	mkdir -p dist/release_android
	cp ../sing-box-for-android/app/build/outputs/apk/release/*.apk dist/release_android
	ghr --replace --draft --prerelease -p 3 "v${VERSION}" dist/release_android
	rm -rf dist/release_android

release_android: lib_android update_android_version build_android upload_android

publish_android:
	cd ../sing-box-for-android && ./gradlew :app:appCenterAssembleAndUploadRelease

build_ios:
	cd ../sing-box-for-apple && \
	rm -rf build/SFI.xcarchive && \
	xcodebuild archive -scheme SFI -configuration Release -archivePath build/SFI.xcarchive

upload_ios_app_store:
	cd ../sing-box-for-apple && \
	xcodebuild -exportArchive -archivePath build/SFI.xcarchive -exportOptionsPlist SFI/Upload.plist

release_ios: build_ios upload_ios_app_store

build_macos:
	cd ../sing-box-for-apple && \
	rm -rf build/SFM.xcarchive && \
	xcodebuild archive -scheme SFM -configuration Release -archivePath build/SFM.xcarchive

upload_macos_app_store:
	cd ../sing-box-for-apple && \
	xcodebuild -exportArchive -archivePath build/SFM.xcarchive -exportOptionsPlist SFI/Upload.plist

release_macos: build_macos upload_macos_app_store

build_macos_independent:
	cd ../sing-box-for-apple && \
	rm -rf build/SFT.System.xcarchive && \
	xcodebuild archive -scheme SFM.System -configuration Release -archivePath build/SFM.System.xcarchive

notarize_macos_independent:
	cd ../sing-box-for-apple && \
	xcodebuild -exportArchive -archivePath "build/SFM.System.xcarchive" -exportOptionsPlist SFM.System/Upload.plist

wait_notarize_macos_independent:
	sleep 60

export_macos_independent:
	rm -rf dist/SFM
	mkdir -p dist/SFM
	cd ../sing-box-for-apple && \
	xcodebuild -exportNotarizedApp -archivePath build/SFM.System.xcarchive -exportPath "../sing-box/dist/SFM"

upload_macos_independent:
	cd dist/SFM && \
	rm -f *.zip && \
	zip -ry "SFM-${VERSION}-universal.zip" SFM.app && \
	ghr --replace --draft --prerelease "v${VERSION}" *.zip

release_macos_independent: build_macos_independent notarize_macos_independent export_macos_independent wait_notarize_macos_independent upload_macos_independent

build_tvos:
	cd ../sing-box-for-apple && \
	rm -rf build/SFT.xcarchive && \
	export DEVELOPER_DIR=/Applications/Xcode-beta.app/Contents/Developer && \
	xcodebuild archive -scheme SFT -configuration Release -archivePath build/SFT.xcarchive

upload_tvos_app_store:
	cd ../sing-box-for-apple && \
	xcodebuild -exportArchive -archivePath "build/SFT.xcarchive" -exportOptionsPlist SFI/Upload.plist

release_tvos: build_tvos upload_tvos_app_store

update_apple_version:
	go run ./cmd/internal/update_apple_version

release_apple: update_apple_version release_ios release_macos release_tvos release_macos_independent
	rm -rf dist

release_apple_beta: update_apple_version release_ios release_macos release_tvos
	rm -rf dist

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

lib_android:
	go run ./cmd/internal/build_libbox -target android

lib_ios:
	go run ./cmd/internal/build_libbox -target ios

lib:
	go run ./cmd/internal/build_libbox -target android
	go run ./cmd/internal/build_libbox -target ios

lib_install:
	go get -v -d
	go install -v github.com/sagernet/gomobile/cmd/gomobile@v0.0.0-20230728014906-3de089147f59
	go install -v github.com/sagernet/gomobile/cmd/gobind@v0.0.0-20230728014906-3de089147f59

clean:
	rm -rf bin dist sing-box
	rm -f $(shell go env GOPATH)/sing-box

update:
	git fetch
	git reset FETCH_HEAD --hard
	git clean -fdx