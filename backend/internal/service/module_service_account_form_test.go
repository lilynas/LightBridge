package service

import (
	"encoding/json"
	"testing"
)

func TestModuleUIAccountFormSpecJSONIncludesModuleVersion(t *testing.T) {
	payload, err := json.Marshal(ModuleUIAccountFormSpec{
		ProviderID:    "lightbridge.provider.mock",
		ProviderName:  "Mock Provider",
		ModuleID:      "lightbridge.provider.mock",
		ModuleName:    "Mock Provider",
		ModuleVersion: "1.2.3",
		RemoteEntry:   "/api/v1/modules/lightbridge.provider.mock/1.2.3/assets/remoteEntry.js",
		ExposedModule: "./AccountForm",
	})
	if err != nil {
		t.Fatalf("marshal account form spec: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal account form spec: %v", err)
	}

	if got["moduleVersion"] != "1.2.3" {
		t.Fatalf("moduleVersion = %q, want %q", got["moduleVersion"], "1.2.3")
	}
}
