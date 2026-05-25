PRODUCT := astralink
MODULE := github.com/astralink/astralink-go
ABIS := arm64-v8a armeabi-v7a x86_64 x86
NDK ?= $(ANDROID_NDK_HOME)

.PHONY: all build test verify server client android-libs lab-transport

all: build

build: server client

server:
	go build -o bin/astralink-server ./cmd/server

client:
	go build -o bin/astralink-client ./cmd/client

test:
	go test ./...

verify:
	go mod verify
	go test ./internal/transport/... ./internal/udpserver/...
	go test ./internal/client/... -skip ExchangeUDP

lab-transport:
	go test ./internal/transport/... -v -run 'RTT|Stickiness|Multipath|FEC|SentVsAcked|Promotion'
	go test ./internal/client/... -v -run 'Transport|StreamAck'

android-libs:
	@mkdir -p android/app/src/main/jniLibs
	@for abi in $(ABIS); do \
		GOOS=android GOARCH=$$(case $$abi in arm64-v8a) echo arm64;; armeabi-v7a) echo arm;; x86_64) echo amd64;; x86) echo 386;; esac) \
		CGO_ENABLED=1 \
		go build -buildmode=c-shared -o android/app/src/main/jniLibs/$$abi/libastralink_client.so ./cmd/client; \
	done
