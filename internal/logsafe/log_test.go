package logsafe

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestRedaction(t *testing.T) {
	secret := "PRIVATE_PLACEHOLDER_VALUE"
	for _, input := range []string{
		"https://user:" + secret + "@example.com/s/" + secret + "/play?signature=" + secret + "#" + secret,
		"HTTPS://user:" + secret + "@example.com/live?signature=" + secret,
		"/direct/s/" + secret + "/play/example/one",
		"/internal/" + secret + "/play/example/one",
		"failed: Authorization: Bearer " + secret,
		"Cookie: session=" + secret + "; other=" + secret,
		"provider_session=" + secret,
		"bearer " + secret,
		"http://%xx?token=" + secret,
		"atv_" + strings.Repeat("x", 43),
	} {
		got := Text(input)
		if strings.Contains(got, secret) || strings.Contains(got, strings.Repeat("x", 43)) {
			t.Fatalf("redaction failed: %s", got)
		}
	}
	if Text("discovery failed") != "discovery failed" {
		t.Fatal("lost safe message")
	}
}

func TestStructuredRedaction(t *testing.T) {
	secret := "PRIVATE_PLACEHOLDER_VALUE"
	var output bytes.Buffer
	logger := New(&output).With("provider", "example").WithGroup("request").With("Cookie", secret)
	logger.Info("provider call failed", slog.Group("nested", slog.String("Authorization", "Bearer "+secret), slog.String("url", "https://example.com/live?token="+secret)),
		"headers", http.Header{"Cookie": []string{secret}}, "error", errors.New(secret), "arbitrary", map[string]string{"value": secret}, "count", 3)
	New(&output).WithGroup("session").With("value", secret).Info("created")
	New(&output).WithGroup("Authorization").Info("request", "value", secret)
	if strings.Contains(output.String(), secret) {
		t.Fatalf("structured secret emitted: %s", output.String())
	}
	if !strings.Contains(output.String(), "example") || !strings.Contains(output.String(), `"count":3`) {
		t.Fatal("safe operational attributes lost")
	}
}
