.PHONY: clean check test build start-local-db stop-local-db

TAG_NAME := $(shell git tag -l --contains HEAD)
SHA := $(shell git rev-parse --short HEAD)
VERSION := $(if $(TAG_NAME),$(TAG_NAME),$(SHA))
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%I:%M:%S%p')
LOCAL_DB := $(shell docker ps -f "name=faunadb" --format '{{.Names}}')

default: clean check test build

start-local-db:
ifneq ($(LOCAL_DB),faunadb)
	docker run -d --name faunadb -p 8443:8443 -p 8084:8084 fauna/faunadb:2.12.1
endif

stop-local-db:
ifeq ($(LOCAL_DB),faunadb)
	docker stop faunadb
	docker rm faunadb
endif

clean:
	rm -rf cover.out

test: clean start-local-db
	go test -v -cover ./...

build: clean
	@echo Version: $(VERSION) $(BUILD_DATE)
	CGO_ENABLED=0 go build -v -ldflags '-X "main.version=${VERSION}" -X "main.commit=${SHA}" -X "main.date=${BUILD_DATE}"'

image:
	docker build -t gcr.io/traefiklabs/plugin-service:$(VERSION) .

publish:
	docker push gcr.io/traefiklabs/plugin-service:$(VERSION)

publish-latest:
	docker tag gcr.io/traefiklabs/plugin-service:$(VERSION) gcr.io/traefiklabs/plugin-service:latest
	docker push gcr.io/traefiklabs/plugin-service:latest

check:
	golangci-lint run
