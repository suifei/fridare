package rebuild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OpenAIProbeOptions drives a real HTTP connectivity check against an
// OpenAI-compatible base URL (not a mere "fields non-empty" check).
type OpenAIProbeOptions struct {
	BaseURL     string
	APIKey      string
	Model       string
	UseGUIProxy bool
	Proxy       string
	Timeout     time.Duration
}

// OpenAIProbeResult is the outcome of ProbeOpenAIEndpoint.
// Detail must never include the raw API key.
type OpenAIProbeResult struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code"`
	Detail     string `json:"detail"`
	Endpoint   string `json:"endpoint"`
	Method     string `json:"method"`
	UsedProxy  bool   `json:"used_proxy"`
	ProxyURL   string `json:"proxy_url,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// RedactSecret replaces secret substrings in text for logs/scratch evidence.
func RedactSecret(text, secret string) string {
	if secret == "" || text == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "***REDACTED***")
}

// NormalizeOpenAIBaseURL trims trailing slashes from the configured base.
func NormalizeOpenAIBaseURL(base string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/")
}

// ProbeOpenAIEndpoint performs a real HTTP call to the OpenAI-compatible API.
// Primary path: GET {base}/models with Bearer auth.
// Fallback: POST {base}/chat/completions with a minimal body when models is 404/405.
// Proxy is applied only when UseGUIProxy is true and Proxy is non-empty.
func ProbeOpenAIEndpoint(ctx context.Context, opts OpenAIProbeOptions) OpenAIProbeResult {
	base := NormalizeOpenAIBaseURL(opts.BaseURL)
	key := strings.TrimSpace(opts.APIKey)
	if base == "" {
		return OpenAIProbeResult{OK: false, Detail: "OpenAI base URL 为空"}
	}
	if key == "" {
		return OpenAIProbeResult{OK: false, Endpoint: base, Detail: "API Key 为空"}
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 25 * time.Second
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}

	transport := &http.Transport{}
	usedProxy := false
	proxyURL := ""
	if opts.UseGUIProxy {
		p := strings.TrimSpace(opts.Proxy)
		if p != "" {
			u, err := url.Parse(p)
			if err != nil {
				return OpenAIProbeResult{
					OK: false, Endpoint: base, UsedProxy: true, ProxyURL: p,
					Detail: "代理 URL 无效: " + err.Error(),
				}
			}
			transport.Proxy = http.ProxyURL(u)
			usedProxy = true
			proxyURL = p
		} else {
			return OpenAIProbeResult{
				OK: false, Endpoint: base, UsedProxy: true,
				Detail: "已启用「端点走 GUI 代理」但代理地址为空",
			}
		}
	} else {
		// Explicit direct: ignore environment proxy for this probe.
		transport.Proxy = nil
	}

	client := &http.Client{Transport: transport, Timeout: timeout}
	modelsURL := base + "/models"
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return OpenAIProbeResult{OK: false, Endpoint: modelsURL, UsedProxy: usedProxy, ProxyURL: proxyURL,
			Detail: "构造请求失败: " + err.Error(), DurationMs: time.Since(start).Milliseconds()}
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "fridare-openai-probe/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return OpenAIProbeResult{
			OK: false, Endpoint: modelsURL, Method: "GET", UsedProxy: usedProxy, ProxyURL: proxyURL,
			Detail: RedactSecret("HTTP 失败: "+err.Error(), key), DurationMs: time.Since(start).Milliseconds(),
		}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	elapsed := time.Since(start).Milliseconds()
	snippet := RedactSecret(compactBody(body), key)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return OpenAIProbeResult{
			OK: true, StatusCode: resp.StatusCode, Endpoint: modelsURL, Method: "GET",
			UsedProxy: usedProxy, ProxyURL: proxyURL, DurationMs: elapsed,
			Detail: fmt.Sprintf("GET /models 成功 HTTP %d body≈%s", resp.StatusCode, snippet),
		}
	}

	// Some gateways reject GET /models; try a minimal chat completion.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed ||
		resp.StatusCode == http.StatusUnauthorized || resp.StatusCode >= 500 {
		chat := tryChatProbe(ctx, client, base, key, opts.Model, usedProxy, proxyURL, start)
		if chat.OK || chat.StatusCode > 0 {
			// Prefer chat result when models failed with not-found; still report auth errors honestly.
			if chat.OK || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
				return chat
			}
		}
	}

	return OpenAIProbeResult{
		OK: false, StatusCode: resp.StatusCode, Endpoint: modelsURL, Method: "GET",
		UsedProxy: usedProxy, ProxyURL: proxyURL, DurationMs: elapsed,
		Detail: fmt.Sprintf("GET /models HTTP %d body≈%s", resp.StatusCode, snippet),
	}
}

func tryChatProbe(ctx context.Context, client *http.Client, base, key, model string, usedProxy bool, proxyURL string, start time.Time) OpenAIProbeResult {
	chatURL := base + "/chat/completions"
	if strings.TrimSpace(model) == "" {
		model = "gpt-4o-mini"
	}
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"max_tokens": 8,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(raw))
	if err != nil {
		return OpenAIProbeResult{OK: false, Endpoint: chatURL, Method: "POST", UsedProxy: usedProxy, ProxyURL: proxyURL,
			Detail: "构造 chat 请求失败: " + err.Error(), DurationMs: time.Since(start).Milliseconds()}
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "fridare-openai-probe/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return OpenAIProbeResult{
			OK: false, Endpoint: chatURL, Method: "POST", UsedProxy: usedProxy, ProxyURL: proxyURL,
			Detail: RedactSecret("chat HTTP 失败: "+err.Error(), key), DurationMs: time.Since(start).Milliseconds(),
		}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	snippet := RedactSecret(compactBody(body), key)
	elapsed := time.Since(start).Milliseconds()
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	return OpenAIProbeResult{
		OK: ok, StatusCode: resp.StatusCode, Endpoint: chatURL, Method: "POST",
		UsedProxy: usedProxy, ProxyURL: proxyURL, DurationMs: elapsed,
		Detail: fmt.Sprintf("POST /chat/completions HTTP %d body≈%s", resp.StatusCode, snippet),
	}
}

func compactBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > 240 {
		s = s[:240] + "…"
	}
	if s == "" {
		return "(empty)"
	}
	return s
}

// FormatOpenAIProbeReport returns a multi-line report safe for logs (key redacted via Detail).
func FormatOpenAIProbeReport(r OpenAIProbeResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("ok=%v status=%d method=%s endpoint=%s\n", r.OK, r.StatusCode, r.Method, r.Endpoint))
	b.WriteString(fmt.Sprintf("used_proxy=%v proxy=%s duration_ms=%d\n", r.UsedProxy, r.ProxyURL, r.DurationMs))
	b.WriteString("detail=" + r.Detail + "\n")
	return b.String()
}
