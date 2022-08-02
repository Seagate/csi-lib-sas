.PHONY: all build clean install

help:
	@echo ""
	@echo "-----------------------------------------------------------------------------------"
	@echo "make all     - clean, build, install"
	@echo "make build   - build sas and example"
	@echo "make clean   - remove all generated files"
	@echo "make install - install sas"
	@echo "make run     - build, run example"
	@echo "make test    - run sas unit test"
	@echo ""

all: clean build install

clean: 
	go clean -r -x
	-rm -rf _output

build:
	go build ./sas/
	go build -o _output/example ./example/main.go

install:
	go install ./sas/

run: build
	_output/example -h

test:
	go test -v ./sas/
