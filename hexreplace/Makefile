.PHONY: test build

help:
	@echo "make test - Run the tests"
	@echo "make build - Build the project"

build:
	@go build 

test:
	@go run main.go /Library/Frameworks/Python.framework/Versions/3.12/lib/python3.12/site-packages/frida/_frida.abi3.so \
	abcde \
	_frida.abi3.so.so