package rebuild

import "fmt"

// BuildTarget describes a Frida configure --host= value and UI metadata.
type BuildTarget struct {
	ID          string // stable id, e.g. "android-arm64"
	Label       string // UI label
	Host        string // passed to ./configure --host=
	Platform    string // android | ios | windows | macos | linux
	Arch        string
	// Notes: honest toolchain limitations (shown in UI)
	Notes string
	// DockerFriendly: true if typically buildable inside Linux Docker with NDK/cross toolchains
	DockerFriendly bool
}

// SupportedBuildTargets lists mainstream platform/CPU goals.
// iOS/macOS targets are listed but marked with Apple-host limitations when building in Linux Docker.
func SupportedBuildTargets() []BuildTarget {
	return []BuildTarget{
		// Android
		{ID: "android-arm64", Label: "Android arm64", Host: "android-arm64", Platform: "android", Arch: "arm64", DockerFriendly: true, Notes: "Linux Docker + Android NDK 可交叉编译"},
		{ID: "android-arm", Label: "Android arm", Host: "android-arm", Platform: "android", Arch: "arm", DockerFriendly: true, Notes: "Linux Docker + Android NDK 可交叉编译"},
		{ID: "android-x86_64", Label: "Android x86_64", Host: "android-x86_64", Platform: "android", Arch: "x86_64", DockerFriendly: true, Notes: "模拟器常用"},
		{ID: "android-x86", Label: "Android x86", Host: "android-x86", Platform: "android", Arch: "x86", DockerFriendly: true, Notes: "模拟器常用"},

		// iOS (Apple host preferred)
		{ID: "ios-arm64", Label: "iOS arm64", Host: "ios-arm64", Platform: "ios", Arch: "arm64", DockerFriendly: false, Notes: "官方工具链偏 macOS/Xcode；Linux Docker 通常无法完整编译"},
		{ID: "ios-arm64e", Label: "iOS arm64e", Host: "ios-arm64e", Platform: "ios", Arch: "arm64e", DockerFriendly: false, Notes: "需要 Apple 宿主与对应 SDK"},
		{ID: "ios-arm64-simulator", Label: "iOS arm64 simulator", Host: "ios-arm64-simulator", Platform: "ios", Arch: "arm64-sim", DockerFriendly: false, Notes: "需要 macOS 宿主"},

		// Windows — Linux Docker 交叉须用 MinGW triplet（x86_64-w64-mingw32），
		// 不能用默认 windows-x86_64（MSVC/ProgramFiles 路径，容器内无 VS）。
		{ID: "windows-x86_64", Label: "Windows x86_64", Host: "x86_64-w64-mingw32", Platform: "windows", Arch: "x86_64", DockerFriendly: true, Notes: "Docker+MinGW 交叉；无预置 mingw SDK 时 configure 会 --without-prebuilds=sdk:host"},
		{ID: "windows-x86", Label: "Windows x86", Host: "i686-w64-mingw32", Platform: "windows", Arch: "x86", DockerFriendly: true, Notes: "Docker+MinGW 交叉（i686-w64-mingw32）"},
		{ID: "windows-arm64", Label: "Windows arm64", Host: "windows-arm64", Platform: "windows", Arch: "arm64", DockerFriendly: false, Notes: "官方偏 MSVC；Linux Docker 交叉支持有限"},

		// macOS
		{ID: "macos-arm64", Label: "macOS arm64", Host: "macos-arm64", Platform: "macos", Arch: "arm64", DockerFriendly: false, Notes: "通常需在 macOS 宿主编译"},
		{ID: "macos-x86_64", Label: "macOS x86_64", Host: "macos-x86_64", Platform: "macos", Arch: "x86_64", DockerFriendly: false, Notes: "通常需在 macOS 宿主编译"},

		// Linux (bonus, often used as host)
		{ID: "linux-x86_64", Label: "Linux x86_64", Host: "linux-x86_64", Platform: "linux", Arch: "x86_64", DockerFriendly: true, Notes: "Docker 原生友好"},
		// Host must be the GNU triplet so Frida releng resolve_gcc_binaries finds aarch64-linux-gnu-gcc.
		// Bare "linux-arm64" leaves machine.triplet=nil → "no C compiler found" without CC= override.
		{ID: "linux-arm64", Label: "Linux arm64", Host: "aarch64-linux-gnu", Platform: "linux", Arch: "arm64", DockerFriendly: true, Notes: "Docker amd64 交叉：Host=aarch64-linux-gnu；镜像 v4+ 预装 gcc-aarch64-linux-gnu"},
	}
}

// TargetByID returns a build target or error.
func TargetByID(id string) (BuildTarget, error) {
	for _, t := range SupportedBuildTargets() {
		if t.ID == id {
			return t, nil
		}
	}
	return BuildTarget{}, fmt.Errorf("未知编译目标: %s", id)
}

// TargetIDs returns all target IDs in order.
func TargetIDs() []string {
	ts := SupportedBuildTargets()
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

// TargetLabels returns UI labels in the same order as TargetIDs.
func TargetLabels() []string {
	ts := SupportedBuildTargets()
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Label
	}
	return out
}

// ConfigureHostArgs returns the configure invocation for out-of-tree builds.
// Pattern matches official Frida docs: ../frida/configure --host=<host>
func ConfigureHostArgs(sourceRel string, host string) []string {
	if sourceRel == "" {
		sourceRel = ".."
	}
	return []string{sourceRel + "/configure", "--host=" + host}
}

// MakeArgs is the default make invocation after configure.
func MakeArgs() []string {
	return []string{"make"}
}
