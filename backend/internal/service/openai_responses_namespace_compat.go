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

	if isOpenAIResponsesInputNamespaceCompatibilityError(code, message, param) {
		return true
	}

	// Some OpenAI-compatible routers wrap the provider error as an encoded JSON
	// string. Keep the fallback narrow: all three markers must be present.
	raw := strings.ToLower(string(responseBody))
	if !strings.Contains(raw, "unknown parameter") {
		return false
	}
	return (strings.Contains(raw, "input[") && strings.Contains(raw, "].namespace")) ||
		(strings.Contains(raw, "input.") && strings.Contains(raw, ".namespace"))
}

func isOpenAIResponsesInputNamespaceCompatibilityError(code, message, param string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	message = strings.ToLower(strings.TrimSpace(message))
	return isOpenAIResponsesInputNamespaceParam(param) &&
		(code == "unknown_parameter" || strings.Contains(message, "unknown parameter"))
}

// shouldRetryOpenAIResponsesWSEventWithoutInputNamespaces applies the same
// narrow compatibility gate to a Responses WebSocket error event. Keeping the
// detector shared prevents HTTP, pooled WS, and passthrough WS from drifting
// into different namespace behavior again.
func shouldRetryOpenAIResponsesWSEventWithoutInputNamespaces(event []byte) bool {
	if len(event) == 0 {
		return false
	}
	return shouldRetryOpenAIResponsesWithoutInputNamespaces(openAIWSErrorHTTPStatus(event), event)
}

func isOpenAIResponsesInputNamespaceParam(raw string) bool {
	value := strings.Trim(strings.TrimSpace(raw), "'\"")
	if strings.HasPrefix(value, "input[") && strings.HasSuffix(value, "].namespace") {
		end := strings.IndexByte(value, ']')
		if end <= len("input[") || value[end:] != "].namespace" {
			return false
		}
		_, err := strconv.Atoi(value[len("input["):end])
		return err == nil
	}
	// A few compatible routers report JSON paths using dot notation instead of
	// the bracket notation used by OpenAI errors.
	if strings.HasPrefix(value, "input.") && strings.HasSuffix(value, ".namespace") {
		index := strings.TrimSuffix(strings.TrimPrefix(value, "input."), ".namespace")
		_, err := strconv.Atoi(index)
		return err == nil
	}
	return false
}

// stripOpenAIResponsesInputNamespaces removes only the top-level namespace
// metadata from replayed input items. Tool definitions and current-turn output
// remain untouched, preserving native namespace support whenever the upstream
// accepts it.
func stripOpenAIResponsesInputNamespaces(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}
	stripItem := func(item map[string]any) bool {
		if item == nil {
			return false
		}
		if _, exists := item["namespace"]; !exists {
			return false
		}
		delete(item, "namespace")
		return true
	}

	changed := false
	switch input := reqBody["input"].(type) {
	case []any:
		for _, rawItem := range input {
			item, ok := rawItem.(map[string]any)
			if ok && stripItem(item) {
				changed = true
			}
		}
	case map[string]any:
		if stripItem(input) {
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
