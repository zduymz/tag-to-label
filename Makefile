.PHONY: linux macos docker run test clean
.DEFAULT_TARGET := macos

NAME ?= tag-to-label
LDFLAGS ?= -X=main.version=$(VERSION) -w -s
VERSION ?= $(shell git describe --tags --always --dirty)
BUILD_FLAGS ?= -v
CGO_ENABLED ?= 0


macos:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=${CGO_ENABLED} go build -o build/macos/${NAME} ${BUILD_FLAGS} -ldflags "$(LDFLAGS)" $^

linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=${CGO_ENABLED} go build -o build/linux/${NAME} ${BUILD_FLAGS} -ldflags "$(LDFLAGS)" $^

docker: linux
	docker build --no-cache --squash --rm -t ${NAME}:latest .
	docker tag ${NAME}:latest duym/${NAME}:latest
	docker push duym/${NAME}:latest

run:
	./build/macos/${NAME} -kubeconfig=./staging.config -aws.creds=/Users/dmai/.aws/credentials

test:
	go test -v -race $(shell go list ./... )

clean:
	- rm -fr ./build/*
	- docker rmi `docker images -f "dangling=true" -q --no-trunc`
