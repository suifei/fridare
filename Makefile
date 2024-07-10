.PHONY: help clean build deploy

help:
	@echo "make help - Show this help"
	@echo "make clean - Clean the project"
	@echo "make deploy - Deploy the project"
	@echo "make build - Build the project"

clean:
	@rm -rf dist

build:
	@./build.sh 16.3.3 8899 http://127.0.0.1:1081

deploy:
	./autoinstall.sh
	@echo "Please run 'frida -H <iPhone IP>:8899 -F' to connect to the device"

all: clean build deploy