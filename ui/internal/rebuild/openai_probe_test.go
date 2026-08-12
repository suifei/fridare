package rebuild

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRedactSecret(t *testing.T) {
	in := "Bearer sk-secret-123 failed sk-secret-123"
	out := RedactSecret(in, "sk-secret-123")
	if strings.Contains(out, "sk-secret-123") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "***REDACTED***") {
		t.Fatal(out)
	}
}

func TestProbeOpenAIEndpoint_ModelsOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer sk-test") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"bad key"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
	}))
	defer srv.Close()

	r := ProbeOpenAIEndpoint(context.Background(), OpenAIProbeOptions{
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-test",
		Timeout: 5 * time.Second,
	})
	if !r.OK {
		t.Fatalf("%+v", r)
	}
	if r.StatusCode != 200 {
		t.Fatal(r.StatusCode)
	}
	if r.UsedProxy {
		t.Fatal("proxy should be off")
	}
	if !strings.Contains(r.Detail, "models") && !strings.Contains(r.Detail, "200") {
		t.Fatal(r.Detail)
	}
	if strings.Contains(r.Detail, "sk-test") {
		t.Fatal("key leaked in detail")
	}
}

func TestProbeOpenAIEndpoint_ChatFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			http.NotFound(w, r)
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	r := ProbeOpenAIEndpoint(context.Background(), OpenAIProbeOptions{
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-x",
		Model:   "gpt-test",
		Timeout: 5 * time.Second,
	})
	if !r.OK {
		t.Fatalf("%+v", r)
	}
	if r.Method != "POST" {
		t.Fatal(r.Method)
	}
	if !strings.Contains(r.Endpoint, "chat/completions") {
		t.Fatal(r.Endpoint)
	}
}

func TestProbeOpenAIEndpoint_ProxyRequiredWhenFlagOn(t *testing.T) {
	r := ProbeOpenAIEndpoint(context.Background(), OpenAIProbeOptions{
		BaseURL:     "https://example.invalid/v1",
		APIKey:      "sk",
		UseGUIProxy: true,
		Proxy:       "",
	})
	if r.OK {
		t.Fatal("expected fail")
	}
	if !strings.Contains(r.Detail, "代理") {
		t.Fatal(r.Detail)
	}
}

func TestProbeOpenAIEndpoint_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()
	r := ProbeOpenAIEndpoint(context.Background(), OpenAIProbeOptions{
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-bad",
		Timeout: 5 * time.Second,
	})
	if r.OK {
		t.Fatal(r)
	}
	if r.StatusCode != 401 {
		// may fall through to chat which also 401
		if r.StatusCode == 0 {
			t.Fatal(r)
		}
	}
	if strings.Contains(r.Detail, "sk-bad") {
		t.Fatal(r.Detail)
	}
}

func TestEnvForAgent_ProxyOffDoesNotInjectGUIProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://system-only:1111")
	env := EnvForAgent(JobConfig{
		Proxy:            "http://gui-proxy:2222",
		AgentUseGUIProxy: false,
		OpenAIAPIKey:     "k",
		OpenAIBaseURL:    RecommendedAPIBase,
	})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "gui-proxy") {
		t.Fatalf("GUI proxy must not be forced when flag off:\n%s", joined)
	}
	// system proxy left alone (not cleared); Windows env keys may be lower-case
	if !strings.Contains(joined, "system-only:1111") {
		t.Fatalf("expected inherited system proxy preserved:\n%s", joined)
	}
	if !strings.Contains(joined, "OPENAI_API_KEY=k") {
		t.Fatal(joined)
	}
}
