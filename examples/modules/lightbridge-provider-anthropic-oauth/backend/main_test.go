package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseAuthorizationCode(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantCode  string
		wantState string
	}{
		{name: "raw code", raw: "abc123", wantCode: "abc123"},
		{name: "code hash state", raw: "abc123#state456", wantCode: "abc123", wantState: "state456"},
		{name: "callback url", raw: "https://platform.claude.com/oauth/code/callback?code=abc123&state=state456", wantCode: "abc123", wantState: "state456"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotState := parseAuthorizationCode(tt.raw)
			if gotCode != tt.wantCode || gotState != tt.wantState {
				t.Fatalf("parseAuthorizationCode() = (%q, %q), want (%q, %q)", gotCode, gotState, tt.wantCode, tt.wantState)
			}
		})
	}
}

func TestBuildAuthorizationURL(t *testing.T) {
	authURL := buildAuthorizationURL("state-1", "challenge-1", claudeOAuthScopeFull)
	for _, want := range []string{
		"https://claude.ai/oauth/authorize?",
		"code=true",
		"client_id=9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		"redirect_uri=https%3A%2F%2Fplatform.claude.com%2Foauth%2Fcode%2Fcallback",
		"scope=org%3Acreate_api_key+user%3Aprofile+user%3Ainference",
		"code_challenge=challenge-1",
		"code_challenge_method=S256",
		"state=state-1",
	} {
		if !strings.Contains(authURL, want) {
			t.Fatalf("auth URL %q does not contain %q", authURL, want)
		}
	}
}

func TestNormalizeModelInBody(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[]}`)
	got := normalizeModelInBody(body)
	var root map[string]any
	if err := json.Unmarshal(got, &root); err != nil {
		t.Fatal(err)
	}
	if root["model"] != "claude-sonnet-4-5-20250929" {
		t.Fatalf("model = %v", root["model"])
	}
}

func TestSanitizeCountTokensRequestBody(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[],"stream":true,"temperature":0.5,"max_tokens":100}`)
	got := sanitizeCountTokensRequestBody(body)
	var root map[string]any
	if err := json.Unmarshal(got, &root); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"stream", "temperature", "max_tokens"} {
		if _, ok := root[key]; ok {
			t.Fatalf("%s should have been removed: %s", key, got)
		}
	}
}
