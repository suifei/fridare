package main
import (
  "os"
  "fridare-gui/internal/rebuild"
)
func main() {
  df := rebuild.DockerfileSkeletonForMirror("docker.1ms.run")
  os.WriteFile(os.Args[1], []byte(df), 0644)
}
