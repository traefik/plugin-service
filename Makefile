.PHONY: clean check test build start-local-db stop-local-db

TAG_NAME := $(shell git tag -l --contains HEAD)
SHA := $(shell git rev-parse --short HEAD)
VERSION := $(if $(TAG_NAME),$(TAG_NAME),$(SHA))
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%I:%M:%S%p')
LOCAL_DB_MONGO := $(shell docker ps -f "name=mongodb-pilot" --format '{{.Names}}')
BIN_NAME := "instance-info-service"

# Default build target
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
DOCKER_BUILD_PLATFORMS ?= linux/amd64,linux/arm64

default: clean check test build

start-local-db:
ifneq ($(LOCAL_DB_MONGO),mongodb-pilot)
	docker run -d -p 27017:27017 --name mongodb-pilot \
        -e MONGO_INITDB_ROOT_USERNAME=mongoadmin \
        -e MONGO_INITDB_ROOT_PASSWORD=secret \
        mongo:4.2.11
endif

stop-local-db:
ifeq ($(LOCAL_DB_MONGO),mongodb-pilot)
	docker stop mongodb-pilot
	docker rm mongodb-pilot
endif

clean:
	rm -rf cover.out

test: clean start-local-db
	go test -v -cover ./...

build: clean
	@echo Version: $(VERSION) $(BUILD_DATE)
	CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build -v -ldflags '-X "main.version=${VERSION}" -X "main.commit=${SHA}" -X "main.date=${BUILD_DATE}"' -o "./dist/${GOOS}/${GOARCH}/${BIN_NAME}"

build-linux-arm64: export GOOS := linux
build-linux-arm64: export GOARCH := arm64
build-linux-arm64:
	make build

build-linux-amd64: export GOOS := linux
build-linux-amd64: export GOARCH := amd64
build-linux-amd64:
	make build

## Build Multi archs Docker image
multi-arch-image-%: build-linux-amd64 build-linux-arm64
	docker buildx build $(DOCKER_BUILDX_ARGS) --progress=chain -t gcr.io/traefiklabs/$(BIN_NAME):$* --platform=$(DOCKER_BUILD_PLATFORMS) -f buildx.Dockerfile .

image:
	docker build -t gcr.io/traefiklabs/plugin-service:$(VERSION) .

publish:
	docker push gcr.io/traefiklabs/plugin-service:$(VERSION)

publish-latest:
	docker tag gcr.io/traefiklabs/plugin-service:$(VERSION) gcr.io/traefiklabs/plugin-service:latest
	docker push gcr.io/traefiklabs/plugin-service:latest

check:
	golangci-lint run
