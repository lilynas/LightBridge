package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeGrokReplayItemsCapsItemCount(t *testing.T) {
	items := make([]json.RawMessage, 0, grokReasoningReplayMaxItems+5)
	for i := 0; i < grokReasoningReplayMaxItems+5; i++ {
		items = append(items, json.RawMessage(fmt.Sprintf(`{"type":"function_call","call_id":"call-%d","name":"tool","arguments":"{}"}`, i)))
	}

	normalized, ok := normalizeGrokReplayItems(items)
	require.True(t, ok)
	require.Len(t, normalized, grokReasoningReplayMaxItems)
}

func TestNormalizeGrokReplayItemsCapsSerializedBytes(t *testing.T) {
	reasoning := json.RawMessage(fmt.Sprintf(`{"type":"reasoning","encrypted_content":%q}`, validGrokReplayEncryptedContentForTest()))
	oversizedCall := json.RawMessage(fmt.Sprintf(
		`{"type":"function_call","call_id":"large","name":"tool","arguments":%q}`,
		strings.Repeat("x", grokReasoningReplayMaxEntryBytes),
	))

	normalized, ok := normalizeGrokReplayItems([]json.RawMessage{reasoning, oversizedCall})
	require.True(t, ok)
	require.Len(t, normalized, 1)
	require.Equal(t, reasoning, normalized[0])
}
