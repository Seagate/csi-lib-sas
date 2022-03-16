.PHONY: all build clean install

all: clean build install

clean: 
	go clean -r -x
	-rm -rf _output

build:
	go build ./sas/
	go build -o _output/example ./example/main.go

install:
	go install ./sas/
