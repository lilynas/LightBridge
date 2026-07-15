package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// shouldRetryOpenAIResponsesWithoutInputNamespaces recognizes the compatibility
// failure returned by Responses implementations that can emit namespaced tool
// calls but reject the namespace field when the same function_call is replayed
// in input history. Official OpenAI/Azure endpoints may require namespace, so
// the gateway only enables this fallback after the selected upstream explicitly
// rejects it.
func shouldRetryOpenAIResponsesWithoutInputNamespaces(statusCode int, responseBody []byte) bool {
	if statusCode != http.StatusBadRequest || len(responseBody) == 0 {
		return false
	}

	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	message := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	param := strings.TrimSpace(firstNonEmptyString(
		gjson.GetBytes(responseBody, "error.param").String(),
		gjson.GetBytes(responseBody, "param").String(),
	))

	if isOpenAIResponsesInputNamespaceParam(param) &&
		(code == "unknown_parameter" || strings.Contains(message, "unknown parameter")) {
		return true
	}

	// Some OpenAI-compatible routers wrap the provider error as an encoded JSON
	// string. Keep the fallback narrow: all three markers must be present.
	raw := strings.ToLower(string(responseBody))
	return strings.Contains(raw, "unknown parameter") &&
		strings.Contains(raw, "input[") &&
		strings.Contains(raw, "].namespace")
}

func isOpenAIResponsesInputNamespaceParam(raw string) bool {
	value := strings.Trim(strings.TrimSpace(raw), "'\"")
	if !strings.HasPrefix(value, "input[") || !strings.HasSuffix(value, "].namespace") {
		return false
	}
	end := strings.IndexByte(value, ']')
	if end <= len("input[") || value[end:] != "].namespace" {
		return false
	}
	_, err := strconv.Atoi(value[len("input["):end])
	return err == nil
}

// stripOpenAIResponsesInputNamespaces removes only the top-level namespace
// metadata from replayed input items. Tool definitions and current-turn output
// remain untouched, preserving native namespace support whenever the upstream
// accepts it.
func stripOpenAIResponsesInputNamespaces(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}
	input, ok := reqBody["input"].([]any)
	if !ok {
		return false
	}

	changed := false
	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := item["namespace"]; exists {
			delete(item, "namespace")
			changed = true
		}
	}
	return changed
}

func stripOpenAIResponsesInputNamespacesFromBody(body []byte) ([]byte, bool, error) {
	if len(body) == 0 || !strings.Contains(string(body), `"namespace"`) {
		return body, false, nil
	}

	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return body, false, fmt.Errorf("parse responses namespace compatibility body: %w", err)
	}
	if !stripOpenAIResponsesInputNamespaces(reqBody) {
		return body, false, nil
	}

	normalized, err := json.Marshal(reqBody)
	if err != nil {
		return body, false, fmt.Errorf("serialize responses namespace compatibility body: %w", err)
	}
	return normalized, true, nil
}
