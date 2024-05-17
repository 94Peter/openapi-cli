VERSION=`git describe --tags`
BUILD_TIME=`date +%FT%T%z`
LDFLAGS=-ldflags "-X main.Version=${V} -X main.BuildTime=${BUILD_TIME}"
NAME=openapi-cli

run: build
	./bin/$(NAME)

build: clear
	go build ${LDFLAGS} -o ./bin/$(NAME) ./container/main.go
	./bin/$(NAME) -v

clear:
	rm -rf ./bin/$(NAME)