package main
import (
  "context"
  "fmt"
  "os"
  "strings"
  "time"
  "fridare-gui/internal/config"
  "fridare-gui/internal/rebuild"
)
func main() {
  cfg, err := config.LoadConfig()
  if err != nil { fmt.Println("err", err); os.Exit(2) }
  key := strings.TrimSpace(cfg.OpenAIAPIKey)
  fmt.Printf("mode=direct base=%s key_len=%d\n", cfg.OpenAIBaseURL, len(key))
  var last bool
  for i := 1; i <= 2; i++ {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    res := rebuild.ProbeOpenAIEndpoint(ctx, rebuild.OpenAIProbeOptions{
      BaseURL: cfg.OpenAIBaseURL, APIKey: key, Model: cfg.OpenAIModel,
      UseGUIProxy: false, Proxy: "", Timeout: 25*time.Second,
    })
    cancel()
    last = res.OK
    fmt.Printf("--- direct probe %d ---\n%s", i, rebuild.RedactSecret(rebuild.FormatOpenAIProbeReport(res), key))
  }
  if !last { os.Exit(1) }
}
