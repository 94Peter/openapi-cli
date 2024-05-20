VERSION=`git describe --tags`
BUILD_TIME=`date +%FT%T%z`
LDFLAGS=-ldflags "-X main.Version=${V} -X main.BuildTime=${BUILD_TIME}"
NAME=openapi-cli

run: build
	./bin/$(NAME) ${PARAMS}

build: clear
	go build ${LDFLAGS} -o ./bin/$(NAME) ./container/main.go
	./bin/$(NAME) -v

clear:
	rm -rf ./bin/$(NAME)

build-docker:
	docker build --build-arg SERVICE=$(NAME) -t 94peter/$(NAME):dev .
	docker rmi $$(docker images --filter "dangling=true" -q --no-trunc)

push-docker:
	docker tag 94peter/${NAME}:dev  94peter/${NAME}:$(V)
	docker push 94peter/${NAME}:$(V)