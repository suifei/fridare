package rebuild

import (
	"fmt"
	"os"
	"strings"
)

// DefaultDockerMirror is a China-friendly Docker Hub proxy prefix.
// Example: library/ubuntu:22.04 → docker.1ms.run/library/ubuntu:22.04
// When using this mirror, pulls should be direct (no HTTP proxy).
const DefaultDockerMirror = "docker.1ms.run"

// DefaultDockerfileBaseImage is the official base used in builder Dockerfile (short name).
const DefaultDockerfileBaseImage = "ubuntu:22.04"

// MirrorPullDirectDefault: docker.1ms.run and similar proxies require direct connect.
const MirrorPullDirectDefault = true

// ResolveDockerImage applies an optional registry mirror prefix to an image reference.
//
// Rules:
//   - empty mirror → image unchanged (or DefaultBuildImage if image empty)
//   - already starts with mirror → unchanged
//   - "ubuntu:22.04" / "library/ubuntu:22.04" → mirror/library/ubuntu:22.04
//   - "docker.io/library/ubuntu:22.04" → mirror/library/ubuntu:22.04
//   - "user/repo:tag" → mirror/user/repo:tag
//   - "ghcr.io/..." left as-is (third-party registry; do not prefix Hub mirrors)
func ResolveDockerImage(image, mirror string) string {
	image = strings.TrimSpace(image)
	mirror = strings.Trim(strings.TrimSpace(mirror), "/")
	if image == "" {
		image = DefaultBuildImage
	}
	if mirror == "" {
		return image
	}
	// Already mirrored
	if strings.HasPrefix(image, mirror+"/") {
		return image
	}
	// Do not re-prefix other registries
	if hasForeignRegistry(image) {
		return image
	}

	ref := image
	// Strip docker.io/ prefix
	ref = strings.TrimPrefix(ref, "docker.io/")
	// Short official names: ubuntu:22.04 → library/ubuntu:22.04
	if !strings.Contains(ref, "/") {
		ref = "library/" + ref
	}
	return mirror + "/" + ref
}

func hasForeignRegistry(image string) bool {
	// "ubuntu:22.04" has no slash — name:tag only, not a registry host
	slash := strings.Index(image, "/")
	if slash < 0 {
		return false
	}
	// first path component before any slash
	first := image[:slash]
	low := strings.ToLower(first)
	if low == "docker.io" {
		return false // Hub; will strip/prefix
	}
	// registry hosts look like ghcr.io, gcr.io, localhost:5000, reg.example.com
	if strings.Contains(first, ".") || strings.Contains(first, ":") {
		return true
	}
	return false
}

// Builder image feature marker — bump when Dockerfile toolchain set changes.
// Bootstrap uses this to decide whether an existing local image is stale.
// v2: Node.js 20 + Go 1.24 + NDK r29 (Frida 17.x requires node>=18; Go powers Compiler backend).
const (
	// v4: + aarch64-linux-gnu cross GCC for linux-arm64 (v3 had mingw only)
	BuilderImageFeatureTag = "toolchain-v4-ndk29-node20-go124-mingw-aarch64"
	BuilderImageNDKPath    = "/opt/android-ndk-r29"
	// Official tarball URLs installed at docker-build time (not per job).
	BuilderImageNodeVersion = "20.18.1"
	BuilderImageGoVersion   = "1.24.2"
	BuilderImageNodeURL     = "https://nodejs.org/dist/v20.18.1/node-v20.18.1-linux-x64.tar.xz"
	BuilderImageGoURL       = "https://go.dev/dl/go1.24.2.linux-amd64.tar.gz"
)

// DockerfileSkeletonForMirror returns a full Frida builder image definition.
// ALL heavy toolchain deps are installed HERE (docker build):
//   apt tools, Node.js ≥18, Go ≥1.24, Android NDK r29, MinGW, aarch64 cross GCC.
// Later AI-mod / compile jobs only VERIFY the environment and let Frida
// fetch version-specific subprojects — never re-download NDK/Node/Go each job.
func DockerfileSkeletonForMirror(mirror string) string {
	base := ResolveDockerImage(DefaultDockerfileBaseImage, mirror)
	ndkURL := AndroidNDKURL
	// China-friendly apt mirror when using docker.1ms.run or empty (still helps in CN)
	useCNApt := mirror != "" || true
	aptSetup := ""
	if useCNApt {
		aptSetup = `
# Prefer Aliyun Ubuntu mirrors (archive.ubuntu.com often blocked without proxy)
RUN set -eux; \
    if [ -f /etc/apt/sources.list ]; then \
      sed -i 's|http://archive.ubuntu.com/ubuntu|http://mirrors.aliyun.com/ubuntu|g' /etc/apt/sources.list; \
      sed -i 's|http://security.ubuntu.com/ubuntu|http://mirrors.aliyun.com/ubuntu|g' /etc/apt/sources.list; \
      sed -i 's|https://archive.ubuntu.com/ubuntu|http://mirrors.aliyun.com/ubuntu|g' /etc/apt/sources.list; \
      sed -i 's|https://security.ubuntu.com/ubuntu|http://mirrors.aliyun.com/ubuntu|g' /etc/apt/sources.list; \
    fi; \
    if [ -d /etc/apt/sources.list.d ]; then \
      find /etc/apt/sources.list.d -type f -exec sed -i 's|archive.ubuntu.com|mirrors.aliyun.com|g' {} \; ; \
      find /etc/apt/sources.list.d -type f -exec sed -i 's|security.ubuntu.com|mirrors.aliyun.com|g' {} \; ; \
    fi
`
	}
	return fmt.Sprintf(`# Fridare Frida builder — ALL heavy deps prepared at image build time
# Base (Hub mirror aware): %s
# Features: %s
# Policy: docker build installs toolchain; AI agent / compile only verifies + version subprojects.
FROM %s
ENV DEBIAN_FRONTEND=noninteractive \
    ANDROID_NDK_ROOT=%s \
    FRIDARE_BUILDER_FEATURES=%s \
    LANG=C.UTF-8 \
    PATH=/usr/local/go/bin:/usr/local/node/bin:$PATH
# Optional proxy for Google/Node/Go downloads during docker build (pass --build-arg)
ARG HTTP_PROXY=
ARG HTTPS_PROXY=
ARG http_proxy=
ARG https_proxy=
%s
# --- apt host tools (Ubuntu packages; NOT nodejs from apt — too old on 22.04) ---
# mingw-w64: windows-x86_64 / windows-x86 cross
# gcc-aarch64-linux-gnu: linux-arm64 cross (Frida host=linux-arm64 needs aarch64-linux-gnu-gcc)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential git curl ca-certificates xz-utils unzip \
    python3 python3-pip python3-setuptools python3-venv \
    ninja-build pkg-config \
    libglib2.0-dev flex bison \
    cmake \
    mingw-w64 g++-mingw-w64 \
    gcc-aarch64-linux-gnu g++-aarch64-linux-gnu \
 && rm -rf /var/lib/apt/lists/* \
 && command -v aarch64-linux-gnu-gcc >/dev/null \
 && aarch64-linux-gnu-gcc --version | head -1
# Helper: apply optional build-time proxy without set -u explosions
# --- Node.js %s (Frida needs >=18; Ubuntu 22.04 apt is v12) ---
RUN set -ex; \
    export http_proxy="${http_proxy:-${HTTP_PROXY:-}}"; \
    export https_proxy="${https_proxy:-${HTTPS_PROXY:-}}"; \
    curl -fL --retry 5 --retry-delay 3 -o /tmp/node.tar.xz %s; \
    mkdir -p /usr/local/node; \
    tar -xJf /tmp/node.tar.xz -C /usr/local/node --strip-components=1; \
    rm -f /tmp/node.tar.xz; \
    node -v; npm -v; \
    node -e "const M=process.versions.node.split('.')[0]|0; if(M<18) process.exit(1)"
# --- Go %s (Frida Compiler backend / ESBuild needs >=1.24) ---
RUN set -ex; \
    export http_proxy="${http_proxy:-${HTTP_PROXY:-}}"; \
    export https_proxy="${https_proxy:-${HTTPS_PROXY:-}}"; \
    curl -fL --retry 5 --retry-delay 3 -o /tmp/go.tgz %s; \
    tar -C /usr/local -xzf /tmp/go.tgz; \
    rm -f /tmp/go.tgz; \
    go version
# --- Android NDK r29 (baked into image; not re-downloaded each job) ---
RUN set -ex; \
    export http_proxy="${http_proxy:-${HTTP_PROXY:-}}"; \
    export https_proxy="${https_proxy:-${HTTPS_PROXY:-}}"; \
    curl -fL --retry 5 --retry-delay 3 -o /tmp/android-ndk.zip %s; \
    mkdir -p /tmp/ndk-extract; \
    unzip -q /tmp/android-ndk.zip -d /tmp/ndk-extract; \
    EXTRACTED="$(find /tmp/ndk-extract -maxdepth 1 -type d -name 'android-ndk-*' | head -1)"; \
    test -n "$EXTRACTED"; \
    mv "$EXTRACTED" "$ANDROID_NDK_ROOT"; \
    rm -rf /tmp/android-ndk.zip /tmp/ndk-extract; \
    test -d "$ANDROID_NDK_ROOT/toolchains" -o -f "$ANDROID_NDK_ROOT/ndk-build"; \
    echo "ANDROID_NDK_ROOT=$ANDROID_NDK_ROOT"
# Clear build-time proxy for runtime containers
ENV HTTP_PROXY= HTTPS_PROXY= http_proxy= https_proxy= ALL_PROXY= all_proxy=
WORKDIR /work
RUN echo "$FRIDARE_BUILDER_FEATURES" > /etc/fridare-builder-features && \
    echo "node=$(node -v) go=$(go version) ndk=$ANDROID_NDK_ROOT" >> /etc/fridare-builder-features
CMD ["bash"]
`, base, BuilderImageFeatureTag, base, BuilderImageNDKPath, BuilderImageFeatureTag, aptSetup,
		BuilderImageNodeVersion, BuilderImageNodeURL,
		BuilderImageGoVersion, BuilderImageGoURL,
		ndkURL)
}

// ImageHasBuilderFeaturesShell returns a one-liner to verify the image was built
// with the expected toolchain (NDK + Node + Go + aarch64 cross). Exit 0 if OK.
func ImageHasBuilderFeaturesShell() string {
	return fmt.Sprintf(
		`set -e; test -f /etc/fridare-builder-features; grep -q %s /etc/fridare-builder-features; test -d "$ANDROID_NDK_ROOT" -o -d %s; command -v node >/dev/null; node -e "const M=process.versions.node.split('.')[0]|0; if(M<18) process.exit(1)"; command -v go >/dev/null; command -v aarch64-linux-gnu-gcc >/dev/null || { echo '[fridare] aarch64-linux-gnu-gcc missing (linux-arm64)'; exit 1; }; echo OK`,
		shellQuote(BuilderImageFeatureTag), shellQuote(BuilderImageNDKPath),
	)
}

// DockerPullEnv builds process env for `docker pull` / `docker build` (Hub access).
// When pullDirect is true (recommended for docker.1ms.run), strips proxy vars so
// the daemon/client reaches the mirror without going through GUI HTTP proxy.
func DockerPullEnv(pullDirect bool) []string {
	env := os.Environ()
	if !pullDirect {
		return env
	}
	return stripProxyEnv(env)
}

// stripProxyEnv removes HTTP(S)_PROXY so Docker Hub mirror can be reached directly.
func stripProxyEnv(env []string) []string {
	drop := map[string]bool{
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "http_proxy": true, "https_proxy": true,
		"ALL_PROXY": true, "all_proxy": true,
		// Keep NO_PROXY — harmless for direct
	}
	out := make([]string, 0, len(env))
	for _, e := range env {
		eq := strings.IndexByte(e, '=')
		if eq <= 0 {
			out = append(out, e)
			continue
		}
		if drop[e[:eq]] {
			continue
		}
		out = append(out, e)
	}
	// Explicitly clear for tools that only check process env after merge
	out = append(out,
		"HTTP_PROXY=", "HTTPS_PROXY=", "http_proxy=", "https_proxy=",
		"ALL_PROXY=", "all_proxy=",
	)
	return out
}

// ShouldPullDockerDirect reports whether image pulls should bypass proxy.
// Default true when a non-empty mirror is configured.
func ShouldPullDockerDirect(mirror string, explicit *bool) bool {
	if explicit != nil {
		return *explicit
	}
	return strings.TrimSpace(mirror) != ""
}

// DockerMirrorHelp is short UI guidance.
const DockerMirrorHelp = `国内访问 Docker Hub 失败时，配置镜像前缀，例如：
  docker.1ms.run
则 ubuntu:22.04 → docker.1ms.run/library/ubuntu:22.04
使用 docker.1ms.run 时请勾选「镜像直连（不走代理）」。`
