VERSION = 0.1.$(shell date +%Y%m%d.%H%M)
FLAGS := "-s -w -X main.version=${VERSION}"

all: build readme

build:
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o http-post http-post.go lib-*.go

readme:
	cp HEAD.md README.md
	echo -e '\n```\n# http-post -h' >> README.md
	./http-post -h 2>> README.md
	echo -e '```' >> README.md
