package main
import (
  "fmt"
  "fridare-gui/internal/rebuild"
  "os"
  "path/filepath"
)
func main() {
  root, magic := os.Args[1], os.Args[2]
  cfg := rebuild.JobConfig{MagicName: magic, DirectionProfile: "deep", ListenPort: 27142}
  if err := rebuild.ApplyDeepSourceExtras(root, cfg); err != nil { panic(err) }
  for _, f := range []string{"agent.c","ns.vala","t.py"} {
    b, _ := os.ReadFile(filepath.Join(root, f))
    fmt.Printf("=== %s ===\n%s\n", f, b)
  }
}
