package apicompat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesInputToChatMessages_DeveloperRoleMapsToSystem(t *testing.T) {
	messages, err := responsesInputToChatMessages("", json.RawMessage(`[{"role":"developer","content":"follow project instructions"}]`))
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, "system", messages[0].Role)
	assert.JSONEq(t, `"follow project instructions"`, string(messages[0].Content))
}

func TestResponsesInputToChatMessages_KeepsChatCompletionRoles(t *testing.T) {
	input := json.RawMessage(`[
		{"role":"system","content":"system message"},
		{"role":"user","content":"user message"},
		{"role":"assistant","content":"assistant message"},
		{"role":"tool","content":"tool message"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 4)

	assert.Equal(t, []string{"system", "user", "assistant", "tool"}, chatMessageRoles(messages))
}

func TestResponsesInputToChatMessages_EmptyRoleFallsBackToUser(t *testing.T) {
	messages, err := responsesInputToChatMessages("", json.RawMessage(`[{"role":"","content":"hello"}]`))
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, "user", messages[0].Role)
}

func TestResponsesInputToChatMessages_DeveloperRoleTrimAndCaseInsensitive(t *testing.T) {
	input := json.RawMessage(`[
		{"role":" Developer ","content":"one"},
		{"role":"\tDEVELOPER\n","content":"two"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 2)

	assert.Equal(t, []string{"system", "system"}, chatMessageRoles(messages))
}

func TestChatCompletionsStreamToResponses_ToolOnlyStrictEventChain(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("grok-test")
	toolIndex := 0
	finishReason := "tool_calls"
	emptyContent := ""

	var events []ResponsesStreamEvent
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		ID:      "chatcmpl_upstream",
		Created: 123,
		Model:   "grok-upstream",
		Choices: []ChatChunkChoice{{
			Index: 0,
			Delta: ChatDelta{Content: &emptyContent, ToolCalls: []ChatToolCall{{
				Index: &toolIndex,
				ID:    "call_1",
				Type:  "function",
				Function: ChatFunctionCall{
					Name:      "lookup",
					Arguments: `{"q":`,
				},
			}}},
		}},
	}, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{
			Index: 0,
			Delta: ChatDelta{ToolCalls: []ChatToolCall{{
				Index:    &toolIndex,
				Function: ChatFunctionCall{Arguments: `"x"}`},
			}}},
		}},
	}, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, FinishReason: &finishReason}},
	}, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Usage: &ChatUsage{PromptTokens: 2, CompletionTokens: 4, TotalTokens: 6},
	}, state)...)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	assert.Equal(t, []string{
		"response.created",
		"response.output_item.added",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	}, responsesStreamEventTypes(events))
	for index, event := range events {
		assert.Equal(t, index, event.SequenceNumber)
	}

	added := events[1]
	require.NotNil(t, added.Item)
	assert.Equal(t, 0, added.OutputIndex)
	assert.Equal(t, "call_1", added.Item.CallID)
	assert.Equal(t, "lookup", added.Item.Name)
	assert.Equal(t, "", added.Item.Arguments)
	itemID := added.Item.ID
	require.NotEmpty(t, itemID)
	assert.Equal(t, itemID, events[2].ItemID)
	assert.Equal(t, itemID, events[3].ItemID)

	argumentsDone := events[4]
	assert.Equal(t, 0, argumentsDone.OutputIndex)
	assert.Equal(t, itemID, argumentsDone.ItemID)
	assert.Equal(t, "lookup", argumentsDone.Name)
	assert.Equal(t, `{"q":"x"}`, argumentsDone.Arguments)

	itemDone := events[5]
	require.NotNil(t, itemDone.Item)
	assert.Equal(t, 0, itemDone.OutputIndex)
	assert.Equal(t, itemID, itemDone.Item.ID)
	assert.Equal(t, "call_1", itemDone.Item.CallID)
	assert.Equal(t, `{"q":"x"}`, itemDone.Item.Arguments)
	assert.Equal(t, "completed", itemDone.Item.Status)

	completed := events[6]
	require.NotNil(t, completed.Response)
	assert.True(t, strings.HasPrefix(completed.Response.ID, "resp_"))
	assert.NotEqual(t, "chatcmpl_upstream", completed.Response.ID)
	assert.Equal(t, int64(123), completed.Response.CreatedAt)
	require.NotNil(t, completed.Response.Usage)
	assert.Equal(t, 6, completed.Response.Usage.TotalTokens)
	require.Len(t, completed.Response.Output, 1)
	assert.Equal(t, itemID, completed.Response.Output[0].ID)
	assert.Equal(t, "call_1", completed.Response.Output[0].CallID)
	assert.Equal(t, `{"q":"x"}`, completed.Response.Output[0].Arguments)

	createdJSON, err := json.Marshal(events[0])
	require.NoError(t, err)
	assert.Contains(t, string(createdJSON), `"sequence_number":0`)
	addedJSON, err := json.Marshal(added)
	require.NoError(t, err)
	assert.Contains(t, string(addedJSON), `"output_index":0`)
	assert.Contains(t, string(addedJSON), `"arguments":""`)
}

func TestChatCompletionsStreamToResponses_ReasoningThenToolUsesContiguousStableIndexes(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("grok-test")
	reasoning := "I should inspect the repository."
	toolIndex := 0
	finishReason := "tool_calls"

	var events []ResponsesStreamEvent
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{ReasoningContent: &reasoning}}},
	}, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{ToolCalls: []ChatToolCall{{
			Index: &toolIndex,
			ID:    "call_1",
			Function: ChatFunctionCall{
				Name:      "read_file",
				Arguments: `{"path":"README.md"}`,
			},
		}}}}},
	}, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, FinishReason: &finishReason}},
	}, state)...)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	assert.Equal(t, []string{
		"response.created",
		"response.output_item.added",
		"response.reasoning_summary_text.delta",
		"response.output_item.added",
		"response.function_call_arguments.delta",
		"response.reasoning_summary_text.done",
		"response.output_item.done",
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	}, responsesStreamEventTypes(events))
	assert.Equal(t, 0, events[1].OutputIndex)
	assert.Equal(t, events[1].Item.ID, events[2].ItemID)
	assert.Equal(t, 1, events[3].OutputIndex)
	assert.Equal(t, events[3].Item.ID, events[4].ItemID)
	assert.Equal(t, events[1].Item.ID, events[5].ItemID)
	assert.Equal(t, events[3].Item.ID, events[7].ItemID)

	completed := events[len(events)-1].Response
	require.NotNil(t, completed)
	require.Len(t, completed.Output, 2)
	assert.Equal(t, "reasoning", completed.Output[0].Type)
	assert.Equal(t, events[1].Item.ID, completed.Output[0].ID)
	assert.Equal(t, "function_call", completed.Output[1].Type)
	assert.Equal(t, events[3].Item.ID, completed.Output[1].ID)
}

func TestChatCompletionsStreamToResponses_ParallelToolIndexesAreContiguous(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("grok-test")
	firstChatIndex := 3
	secondChatIndex := 7
	finishReason := "tool_calls"

	var events []ResponsesStreamEvent
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{ToolCalls: []ChatToolCall{
			{Index: &firstChatIndex, ID: "call_3", Function: ChatFunctionCall{Name: "first", Arguments: `{}`}},
			{Index: &secondChatIndex, ID: "call_7", Function: ChatFunctionCall{Name: "second", Arguments: `{}`}},
		}}}},
	}, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, FinishReason: &finishReason}},
	}, state)...)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	assert.Equal(t, 0, events[1].OutputIndex)
	assert.Equal(t, 0, events[2].OutputIndex)
	assert.Equal(t, 1, events[3].OutputIndex)
	assert.Equal(t, 1, events[4].OutputIndex)
	completed := events[len(events)-1].Response
	require.NotNil(t, completed)
	require.Len(t, completed.Output, 2)
	assert.Equal(t, "call_3", completed.Output[0].CallID)
	assert.Equal(t, "call_7", completed.Output[1].CallID)
}

func TestResponsesToChatCompletionsRequest_InstructionsAndInputDeveloperRole(t *testing.T) {
	req := &ResponsesRequest{
		Model:        "gpt-4o",
		Instructions: "Use concise answers.",
		Input: json.RawMessage(`[
			{"role":"developer","content":[{"type":"input_text","text":"Prefer JSON."}]},
			{"role":"user","content":"Hello"}
		]`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 3)

	assert.Equal(t, []string{"system", "system", "user"}, chatMessageRoles(out.Messages))
	assert.JSONEq(t, `"Use concise answers."`, string(out.Messages[0].Content))
	assert.JSONEq(t, `"Prefer JSON."`, string(out.Messages[1].Content))
	assert.JSONEq(t, `"Hello"`, string(out.Messages[2].Content))
}

func chatMessageRoles(messages []ChatMessage) []string {
	roles := make([]string, 0, len(messages))
	for _, message := range messages {
		roles = append(roles, message.Role)
	}
	return roles
}

func responsesStreamEventTypes(events []ResponsesStreamEvent) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}
