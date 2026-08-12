# Fridare build — formal artifacts go under dist/
# Windows: use make with MSYS/Git Bash, or run the PowerShell targets via make if available.
#
# Examples:
#   make help
#   make build-windows-amd64          # GUI + tools → dist/release-vVERSION/windows-amd64 + zip
#   make release VERSION=4.0.4        # all platforms (tools multi-arch + Windows GUI)
#   make clean

VERSION ?= 4.0.4
DIST    := dist
RELEASE := $(DIST)/release-v$(VERSION)

.PHONY: help clean build build-windows-amd64 release release-all tools-host deploy all

help:
	@echo "Fridare Makefile — formal output: $(DIST)/"
	@echo ""
	@echo "  make build-windows-amd64   Build Windows x64 GUI+tools → $(RELEASE)/windows-amd64 + zip"
	@echo "  make release               Same as build-windows-amd64 (default test/release on Windows host)"
	@echo "  make release-all           Full multi-platform tool zips + Windows GUI (scripts/build-release.ps1)"
	@echo "  make clean                 Remove $(DIST)/"
	@echo "  make build                 Legacy: fridare.sh build -latest (frida deb/server patch pipeline)"
	@echo "  make deploy                autoinstall.sh"
	@echo ""
	@echo "VERSION=$(VERSION)  DIST=$(DIST)"

clean:
	@echo "Cleaning $(DIST)/ ..."
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "if (Test-Path '$(DIST)') { Remove-Item -Recurse -Force '$(DIST)' }"
else
	@rm -rf "$(DIST)"
endif
	@echo "Done."

# Formal Windows x64 package (GUI + create/patch/hexreplace + helpers)
build-windows-amd64:
	@echo "=== Build Windows amd64 → $(RELEASE) ==="
	powershell -NoProfile -File scripts/build-release.ps1 -Version $(VERSION) -Only windows-amd64
	@echo ""
	@echo "Artifacts:"
	@echo "  $(RELEASE)/windows-amd64/"
	@echo "  $(RELEASE)/fridare-v$(VERSION)-windows-amd64.zip"

# Default "release" for local testing on a Windows host
release: build-windows-amd64

# Full multi-platform release (tools for all GOOS/GOARCH + Windows GUI)
release-all:
	powershell -NoProfile -File scripts/build-release.ps1 -Version $(VERSION)

# Legacy frida shell build (outputs still under dist/ via fridare.sh)
build:
	@./fridare.sh build -latest -y

deploy:
	./autoinstall.sh
	@echo "Please run 'frida -H <iPhone IP>:8899 -F' to connect to the device"

all: clean build-windows-amd64
