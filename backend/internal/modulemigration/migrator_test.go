package modulemigration

import "testing"

func TestNormalizeOpenAIAccountFromLightBridgeKeepsOAuthAndCodexMetadata(t *testing.T) {
	row := sourceRow{
		"__source_kind":  SourceLightBridge,
		"id":             int64(42),
		"name":           "Codex OAuth",
		"platform":       "openai",
		"type":           "oauth",
		"credentials":    `{"access_token":"old-at","refresh_token":"old-rt","client_id":"app_old"}`,
		"extra":          `{"openai_passthrough":true}`,
		"codex_cli_only": true,
		"openai_oauth_responses_websockets_v2_mode":    "ctx_pool",
		"openai_oauth_responses_websockets_v2_enabled": true,
	}

	record := accountFromRow(SourceLightBridge, row)
	if !isOpenAIAccount(record, row) {
		t.Fatal("expected row to be detected as OpenAI account")
	}
	normalizeOpenAIAccount(&record, row)

	if record.ProviderID != openAIProviderID {
		t.Fatalf("provider_id = %q, want %q", record.ProviderID, openAIProviderID)
	}
	if record.Platform != "module" {
		t.Fatalf("platform = %q, want module", record.Platform)
	}
	if got := record.Credentials["refresh_token"]; got != "old-rt" {
		t.Fatalf("refresh_token = %v, want old-rt", got)
	}
	if got := record.Credentials["client_id"]; got != "app_old" {
		t.Fatalf("client_id = %v, want app_old", got)
	}
	if got := record.Extra["openai_passthrough"]; got != true {
		t.Fatalf("openai_passthrough = %v, want true", got)
	}
	if got := record.Extra["codex_cli_only"]; got != true {
		t.Fatalf("codex_cli_only = %v, want true", got)
	}
	migration, ok := record.Extra["module_migration"].(map[string]any)
	if !ok {
		t.Fatalf("module_migration metadata missing: %#v", record.Extra["module_migration"])
	}
	if migration["source"] != SourceLightBridge {
		t.Fatalf("migration source = %v, want %s", migration["source"], SourceLightBridge)
	}
	if migration["legacy_account_id"] != "42" {
		t.Fatalf("legacy_account_id = %v, want 42", migration["legacy_account_id"])
	}
}

func TestNormalizeOpenAIAccountFromSub2APITokenBuildsAPIKeyAccount(t *testing.T) {
	row := sourceRow{
		"__source_kind": SourceSub2API,
		"id":            "sub2-token-1",
		"provider":      "openai",
		"token":         "sk-legacy",
		"base_url":      "https://api.openai.com/v1",
		"name":          "Sub2API OpenAI",
	}

	record := accountFromRow(SourceSub2API, row)
	if !isOpenAIAccount(record, row) {
		t.Fatal("expected Sub2API row to be detected as OpenAI account")
	}
	normalizeOpenAIAccount(&record, row)

	if record.Type != "apikey" {
		t.Fatalf("type = %q, want apikey", record.Type)
	}
	if got := record.Credentials["api_key"]; got != "sk-legacy" {
		t.Fatalf("api_key = %v, want sk-legacy", got)
	}
	if got := record.Credentials["base_url"]; got != "https://api.openai.com/v1" {
		t.Fatalf("base_url = %v, want OpenAI base URL", got)
	}
	migration := record.Extra["module_migration"].(map[string]any)
	if migration["source"] != SourceSub2API {
		t.Fatalf("migration source = %v, want %s", migration["source"], SourceSub2API)
	}
}
