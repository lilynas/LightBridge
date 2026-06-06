package main

import (
	"encoding/json"
	"testing"
)

func TestPrepareOpenAIUpstreamRequestConvertsOAuthChatCompletions(t *testing.T) {
	body := []byte(`{
		"model":"codex",
		"stream":false,
		"messages":[
			{"role":"system","content":"be concise"},
			{"role":"user","content":[{"type":"text","text":"hello"}]}
		],
		"max_completion_tokens":4,
		"temperature":0.7
	}`)
	url, out, oauth, err := prepareOpenAIUpstreamRequest(GatewayRequest{
		Endpoint: "/v1/chat/completions",
		Body:     body,
		Account: ProviderAccount{
			Config:  map[string]any{"type": "oauth"},
			Secrets: map[string]any{"access_token": "oauth-token"},
		},
	})
	if err != nil {
		t.Fatalf("prepareOpenAIUpstreamRequest returned error: %v", err)
	}
	if url != chatGPTCodexResponsesURL {
		t.Fatalf("url = %q, want %q", url, chatGPTCodexResponsesURL)
	}
	if !oauth {
		t.Fatalf("oauth = false, want true")
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decode converted body: %v", err)
	}
	if decoded["model"] != "gpt-5.1-codex" {
		t.Fatalf("model = %v, want gpt-5.1-codex", decoded["model"])
	}
	if decoded["stream"] != true {
		t.Fatalf("stream = %v, want true", decoded["stream"])
	}
	if decoded["store"] != false {
		t.Fatalf("store = %v, want false", decoded["store"])
	}
	if got := int(decoded["max_output_tokens"].(float64)); got != 16 {
		t.Fatalf("max_output_tokens = %d, want minimum 16", got)
	}
	input, ok := decoded["input"].([]any)
	if !ok || len(input) != 2 {
		t.Fatalf("input = %#v, want two items", decoded["input"])
	}
}

func TestPrepareOpenAIUpstreamRequestKeepsAPIKeyChatCompletions(t *testing.T) {
	body := []byte(`{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"}]}`)
	url, out, oauth, err := prepareOpenAIUpstreamRequest(GatewayRequest{
		Endpoint: "/v1/chat/completions",
		Body:     body,
		Account: ProviderAccount{
			Secrets: map[string]any{"api_key": "sk-test"},
		},
	})
	if err != nil {
		t.Fatalf("prepareOpenAIUpstreamRequest returned error: %v", err)
	}
	if url != openAIAPIBaseURL+"/v1/chat/completions" {
		t.Fatalf("url = %q, want API chat completions URL", url)
	}
	if oauth {
		t.Fatalf("oauth = true, want false")
	}
	if string(out) != string(body) {
		t.Fatalf("body changed for API key account: %s", string(out))
	}
}

func TestApplyTokenResponseStoresOAuthSecrets(t *testing.T) {
	account := ProviderAccount{}
	applyTokenResponse(&account, &tokenResponse{
		AccessToken:  "access",
		RefreshToken: "refresh",
		IDToken:      "id",
		ExpiresIn:    3600,
	})
	if account.Secrets["access_token"] != "access" {
		t.Fatalf("access_token not stored: %#v", account.Secrets)
	}
	if account.Secrets["refresh_token"] != "refresh" {
		t.Fatalf("refresh_token not stored: %#v", account.Secrets)
	}
	if account.Secrets["id_token"] != "id" {
		t.Fatalf("id_token not stored: %#v", account.Secrets)
	}
	if account.Metadata["expires_in"] != int64(3600) {
		t.Fatalf("expires_in = %#v, want 3600", account.Metadata["expires_in"])
	}
}
