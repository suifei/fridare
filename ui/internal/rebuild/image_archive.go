package rebuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultDockerHubMirror is the recommended China Hub mirror (user-provided).
// Used as default in GUI config and Dockerfile FROM rewriting.
const DefaultDockerHubMirror = "docker.1ms.run"

// BuilderStableImageTag returns a local-only tag alias for the feature set,
// e.g. fridare/frida-builder:toolchain-v2-ndk29-node20-go124
// so users can reuse the exact toolchain without rebuilding.
func BuilderStableImageTag(baseImage string) string {
	repo := strings.TrimSpace(baseImage)
	if repo == "" {
		repo = DefaultBuildImage
	}
	// strip existing tag
	if i := strings.LastIndex(repo, ":"); i > 0 && !strings.Contains(repo[i+1:], "/") {
		repo = repo[:i]
	}
	return repo + ":" + BuilderImageFeatureTag
}

// ImageArchiveDir is where docker save tars are kept for offline reuse.
func ImageArchiveDir(workRoot string) string {
	if workRoot == "" {
		workRoot = DefaultSourceWorkDir("")
	}
	return filepath.Join(workRoot, "images")
}

// ArchiveBuilderImage runs `docker save -o <work>/images/<name>-<feature>.tar <image>`.
// Large (~several GB); only when user requests or bootstrap step completes with ArchiveImage.
// Returns the tar path on success.
func ArchiveBuilderImage(ctx context.Context, runner Runner, env []string, image, workRoot string) (string, error) {
	if runner == nil {
		return "", fmt.Errorf("no runner")
	}
	image = strings.TrimSpace(image)
	if image == "" {
		image = DefaultBuildImage
	}
	dir := ImageArchiveDir(workRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	// Stable filename so re-bootstrap overwrites same archive (one copy per feature set)
	name := strings.ReplaceAll(BuilderImageFeatureTag, ":", "-")
	tarPath := filepath.Join(dir, "frida-builder-"+name+".tar")
	// docker save -o path image
	out, err := runner.Run(ctx, env, "docker", "save", "-o", tarPath, image)
	if err != nil {
		return "", fmt.Errorf("docker save: %w\n%s", err, truncate(out, 400))
	}
	// Write a small sidecar with metadata
	meta := fmt.Sprintf("image=%s\nfeature=%s\nsaved_at=%s\nstable_tag=%s\n",
		image, BuilderImageFeatureTag, time.Now().Format(time.RFC3339), BuilderStableImageTag(image))
	_ = os.WriteFile(tarPath+".txt", []byte(meta), 0644)
	return tarPath, nil
}

// LoadArchivedBuilderImage loads a previously saved tar if local image missing.
func LoadArchivedBuilderImage(ctx context.Context, runner Runner, env []string, workRoot, preferImage string) (string, error) {
	if runner == nil {
		return "", fmt.Errorf("no runner")
	}
	dir := ImageArchiveDir(workRoot)
	name := strings.ReplaceAll(BuilderImageFeatureTag, ":", "-")
	tarPath := filepath.Join(dir, "frida-builder-"+name+".tar")
	if _, err := os.Stat(tarPath); err != nil {
		return "", fmt.Errorf("无本地镜像档案: %s", tarPath)
	}
	out, err := runner.Run(ctx, env, "docker", "load", "-i", tarPath)
	if err != nil {
		return "", fmt.Errorf("docker load: %w\n%s", err, truncate(out, 400))
	}
	// Retag to preferred local name if needed
	if preferImage != "" {
		stable := BuilderStableImageTag(preferImage)
		// Best-effort: tag latest name from load output is hard; re-tag stable → prefer
		_, _ = runner.Run(ctx, env, "docker", "tag", stable, preferImage)
	}
	return tarPath, nil
}

// ProbeLocalBuilderImage checks whether a usable builder image exists locally.
// Returns detail string for GUI (image id / feature / missing).
func ProbeLocalBuilderImage(ctx context.Context, runner Runner, image string) (ready bool, detail string) {
	if runner == nil {
		return false, "no runner"
	}
	if image == "" {
		image = DefaultBuildImage
	}
	out, err := runner.Run(ctx, nil, "docker", "images", "-q", image)
	if err != nil || strings.TrimSpace(out) == "" {
		// try stable feature tag
		stable := BuilderStableImageTag(image)
		out2, err2 := runner.Run(ctx, nil, "docker", "images", "-q", stable)
		if err2 != nil || strings.TrimSpace(out2) == "" {
			return false, "本地无 builder 镜像，请先执行步骤①"
		}
		image = stable
		out = out2
	}
	probe := RunContainerArgs(DockerWorkspace{Image: image}, []string{"bash", "-lc", ImageHasBuilderFeaturesShell()})
	if _, perr := runner.Run(ctx, nil, probe[0], probe[1:]...); perr != nil {
		return false, fmt.Sprintf("镜像 %s 存在但工具链过期/不完整（需重建步骤①） id=%s", image, strings.TrimSpace(out))
	}
	return true, fmt.Sprintf("就绪 %s feature=%s id=%s", image, BuilderImageFeatureTag, strings.TrimSpace(out))
}
