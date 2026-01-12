NAME = sing-box
COMMIT = $(shell git rev-parse --short HEAD)
TAGS ?= with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_ccm,with_ocm,badlinkname,tfogo_checklinkname0

GOHOSTOS = $(shell go env GOHOSTOS)
GOHOSTARCH = $(shell go env GOHOSTARCH)
VERSION=$(shell CGO_ENABLED=0 GOOS=$(GOHOSTOS) GOARCH=$(GOHOSTARCH) go run github.com/sagernet/sing-box/cmd/internal/read_tag@latest)

PARAMS = -v -trimpath -ldflags "-X 'github.com/sagernet/sing-box/constant.Version=$(VERSION)' -X 'internal/godebug.defaultGODEBUG=multipathtcp=0' -s -w -buildid= -checklinkname=0"
MAIN_PARAMS = $(PARAMS) -tags "$(TAGS)"
MAIN = ./cmd/sing-box
PREFIX ?= $(shell go env GOPATH)

.PHONY: test release docs build

build:
	export GOTOOLCHAIN=local && \
	go build $(MAIN_PARAMS) $(MAIN)

race:
	export GOTOOLCHAIN=local && \
	go build -race $(MAIN_PARAMS) $(MAIN)

ci_build:
	export GOTOOLCHAIN=local && \
	go build $(PARAMS) $(MAIN) && \
	go build $(MAIN_PARAMS) $(MAIN)

generate_completions:
	go run -v --tags "$(TAGS),generate,generate_completions" $(MAIN)

install:
	go build -o $(PREFIX)/bin/$(NAME) $(MAIN_PARAMS) $(MAIN)

fmt:
	@gofumpt -l -w .
	@gofmt -s -w .
	@gci write --custom-order -s standard -s "prefix(github.com/sagernet/)" -s "default" .

fmt_docs:
	go run ./cmd/internal/format_docs

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
	go install -v github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

proto:
	@go run ./cmd/internal/protogen
	@gofumpt -l -w .
	@gofumpt -l -w .

proto_install:
	go install -v google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install -v google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

update_certificates:
	go run ./cmd/internal/update_certificates

release:
	go run ./cmd/internal/build goreleaser release --clean --skip publish
	mkdir dist/release
	mv dist/*.tar.gz \
		dist/*.zip \
		dist/*.deb \
		dist/*.rpm \
		dist/*_amd64.pkg.tar.zst \
		dist/*_arm64.pkg.tar.zst \
		dist/release
	ghr --replace --draft --prerelease -p 5 "v${VERSION}" dist/release
	rm -r dist/release

release_repo:
	go run ./cmd/internal/build goreleaser release -f .goreleaser.fury.yaml --clean

release_install:
	go install -v github.com/tcnksm/ghr@latest

update_android_version:
	go run ./cmd/internal/update_android_version

build_android:
	cd ../sing-box-for-android && ./gradlew :app:clean :app:assemblePlayRelease :app:assembleOtherRelease && ./gradlew --stop

upload_android:
	mkdir -p dist/release_android
	cp ../sing-box-for-android/app/build/outputs/apk/play/release/*.apk dist/release_android
	cp ../sing-box-for-android/app/build/outputs/apk/other/release/*-universal.apk dist/release_android
	ghr --replace --draft --prerelease -p 5 "v${VERSION}" dist/release_android
	rm -rf dist/release_android

release_android: lib_android update_android_version build_android upload_android

publish_android:
	cd ../sing-box-for-android && ./gradlew :app:publishPlayReleaseBundle && ./gradlew --stop

# TODO: find why and remove `-destination 'generic/platform=iOS'`
# TODO: remove xcode clean when fix control widget fixed
build_ios:
	cd ../sing-box-for-apple && \
	rm -rf build/SFI.xcarchive && \
	xcodebuild clean -scheme SFI && \
	xcodebuild archive -scheme SFI -configuration Release -destination 'generic/platform=iOS' -archivePath build/SFI.xcarchive -allowProvisioningUpdates | xcbeautify | grep -A 10 -e "Archive Succeeded" -e "ARCHIVE FAILED" -e "❌"

upload_ios_app_store:
	cd ../sing-box-for-apple && \
	xcodebuild -exportArchive -archivePath build/SFI.xcarchive -exportOptionsPlist SFI/Upload.plist -allowProvisioningUpdates

export_ios_ipa:
	cd ../sing-box-for-apple && \
	xcodebuild -exportArchive -archivePath build/SFI.xcarchive -exportOptionsPlist SFI/Export.plist -allowProvisioningUpdates -exportPath build/SFI && \
	cp build/SFI/sing-box.ipa dist/SFI.ipa

upload_ios_ipa:
	cd dist && \
	cp SFI.ipa "SFI-${VERSION}.ipa" && \
	ghr --replace --draft --prerelease "v${VERSION}" "SFI-${VERSION}.ipa"

release_ios: build_ios upload_ios_app_store

build_macos:
	cd ../sing-box-for-apple && \
	rm -rf build/SFM.xcarchive && \
	xcodebuild archive -scheme SFM -configuration Release -archivePath build/SFM.xcarchive -allowProvisioningUpdates | xcbeautify | grep -A 10 -e "Archive Succeeded" -e "ARCHIVE FAILED" -e "❌"

upload_macos_app_store:
	cd ../sing-box-for-apple && \
	xcodebuild -exportArchive -archivePath build/SFM.xcarchive -exportOptionsPlist SFI/Upload.plist -allowProvisioningUpdates

release_macos: build_macos upload_macos_app_store

build_macos_standalone:
	$(MAKE) -C ../sing-box-for-apple archive_macos_standalone

build_macos_dmg:
	$(MAKE) -C ../sing-box-for-apple build_macos_dmg

build_macos_pkg:
	$(MAKE) -C ../sing-box-for-apple build_macos_pkg

notarize_macos_dmg:
	$(MAKE) -C ../sing-box-for-apple notarize_macos_dmg

notarize_macos_pkg:
	$(MAKE) -C ../sing-box-for-apple notarize_macos_pkg

upload_macos_dmg:
	mkdir -p dist/SFM
	cp ../sing-box-for-apple/build/SFM-Apple.dmg "dist/SFM/SFM-${VERSION}-Apple.dmg"
	cp ../sing-box-for-apple/build/SFM-Intel.dmg "dist/SFM/SFM-${VERSION}-Intel.dmg"
	cp ../sing-box-for-apple/build/SFM-Universal.dmg "dist/SFM/SFM-${VERSION}-Universal.dmg"
	ghr --replace --draft --prerelease "v${VERSION}" "dist/SFM/SFM-${VERSION}-Apple.dmg"
	ghr --replace --draft --prerelease "v${VERSION}" "dist/SFM/SFM-${VERSION}-Intel.dmg"
	ghr --replace --draft --prerelease "v${VERSION}" "dist/SFM/SFM-${VERSION}-Universal.dmg"

upload_macos_pkg:
	mkdir -p dist/SFM
	cp ../sing-box-for-apple/build/SFM-Apple.pkg "dist/SFM/SFM-${VERSION}-Apple.pkg"
	cp ../sing-box-for-apple/build/SFM-Intel.pkg "dist/SFM/SFM-${VERSION}-Intel.pkg"
	cp ../sing-box-for-apple/build/SFM-Universal.pkg "dist/SFM/SFM-${VERSION}-Universal.pkg"
	ghr --replace --draft --prerelease "v${VERSION}" "dist/SFM/SFM-${VERSION}-Apple.pkg"
	ghr --replace --draft --prerelease "v${VERSION}" "dist/SFM/SFM-${VERSION}-Intel.pkg"
	ghr --replace --draft --prerelease "v${VERSION}" "dist/SFM/SFM-${VERSION}-Universal.pkg"

upload_macos_dsyms:
	mkdir -p dist/SFM
	cd ../sing-box-for-apple/build/SFM.System-universal.xcarchive && zip -r SFM.dSYMs.zip dSYMs
	cp ../sing-box-for-apple/build/SFM.System-universal.xcarchive/SFM.dSYMs.zip "dist/SFM/SFM-${VERSION}.dSYMs.zip"
	ghr --replace --draft --prerelease "v${VERSION}" "dist/SFM/SFM-${VERSION}.dSYMs.zip"

release_macos_standalone: build_macos_pkg notarize_macos_pkg upload_macos_pkg upload_macos_dsyms

build_tvos:
	cd ../sing-box-for-apple && \
	rm -rf build/SFT.xcarchive && \
	xcodebuild archive -scheme SFT -configuration Release -archivePath build/SFT.xcarchive -allowProvisioningUpdates | xcbeautify | grep -A 10 -e "Archive Succeeded" -e "ARCHIVE FAILED" -e "❌"

upload_tvos_app_store:
	cd ../sing-box-for-apple && \
	xcodebuild -exportArchive -archivePath "build/SFT.xcarchive" -exportOptionsPlist SFI/Upload.plist -allowProvisioningUpdates

export_tvos_ipa:
	cd ../sing-box-for-apple && \
	xcodebuild -exportArchive -archivePath "build/SFT.xcarchive" -exportOptionsPlist SFI/Export.plist -allowProvisioningUpdates -exportPath build/SFT && \
	cp build/SFT/sing-box.ipa dist/SFT.ipa

upload_tvos_ipa:
	cd dist && \
	cp SFT.ipa "SFT-${VERSION}.ipa" && \
	ghr --replace --draft --prerelease "v${VERSION}" "SFT-${VERSION}.ipa"

release_tvos: build_tvos upload_tvos_app_store

update_apple_version:
	go run ./cmd/internal/update_apple_version

update_macos_version:
	MACOS_PROJECT_VERSION=$(shell go run -v ./cmd/internal/app_store_connect next_macos_project_version) go run ./cmd/internal/update_apple_version

release_apple: lib_ios update_apple_version release_ios release_macos release_tvos release_macos_standalone

release_apple_beta: update_apple_version release_ios release_macos release_tvos

publish_testflight:
	go run -v ./cmd/internal/app_store_connect publish_testflight $(filter-out $@,$(MAKECMDGOALS))

prepare_app_store:
	go run -v ./cmd/internal/app_store_connect prepare_app_store

publish_app_store:
	go run -v ./cmd/internal/app_store_connect publish_app_store

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

lib_android_debug:
	go run ./cmd/internal/build_libbox -target android -debug

lib_apple:
	go run ./cmd/internal/build_libbox -target apple

lib_ios:
	go run ./cmd/internal/build_libbox -target apple -platform ios -debug

lib:
	go run ./cmd/internal/build_libbox -target android
	go run ./cmd/internal/build_libbox -target ios

lib_install:
	go install -v github.com/sagernet/gomobile/cmd/gomobile@v0.1.11
	go install -v github.com/sagernet/gomobile/cmd/gobind@v0.1.11

docs:
	venv/bin/mkdocs serve

publish_docs:
	venv/bin/mkdocs gh-deploy -m "Update" --force --ignore-version --no-history

docs_install:
	python -m venv venv
	source ./venv/bin/activate && pip install --force-reinstall mkdocs-material=="9.*" mkdocs-static-i18n=="1.2.*"

clean:
	rm -rf bin dist sing-box
	rm -f $(shell go env GOPATH)/sing-box

update:
	git fetch
	git reset FETCH_HEAD --hard
	git clean -fdx

%:
	@:
