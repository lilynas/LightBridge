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

	argumentsDone := events[3]
	assert.Equal(t, 0, argumentsDone.OutputIndex)
	assert.Equal(t, itemID, argumentsDone.ItemID)
	assert.Equal(t, "lookup", argumentsDone.Name)
	assert.Equal(t, `{"q":"x"}`, argumentsDone.Arguments)

	itemDone := events[4]
	require.NotNil(t, itemDone.Item)
	assert.Equal(t, 0, itemDone.OutputIndex)
	assert.Equal(t, itemID, itemDone.Item.ID)
	assert.Equal(t, "call_1", itemDone.Item.CallID)
	assert.Equal(t, `{"q":"x"}`, itemDone.Item.Arguments)
	assert.Equal(t, "completed", itemDone.Item.Status)

	completed := events[5]
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
		"response.reasoning_summary_text.done",
		"response.output_item.done",
		"response.output_item.added",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	}, responsesStreamEventTypes(events))
	assert.Equal(t, 0, events[1].OutputIndex)
	assert.Equal(t, events[1].Item.ID, events[2].ItemID)
	assert.Equal(t, events[1].Item.ID, events[3].ItemID)
	assert.Equal(t, 1, events[5].OutputIndex)
	assert.Equal(t, events[5].Item.ID, events[6].ItemID)
	assert.Equal(t, events[5].Item.ID, events[7].ItemID)

	completed := events[len(events)-1].Response
	require.NotNil(t, completed)
	require.Len(t, completed.Output, 2)
	assert.Equal(t, "reasoning", completed.Output[0].Type)
	assert.Equal(t, events[1].Item.ID, completed.Output[0].ID)
	assert.Equal(t, "function_call", completed.Output[1].Type)
	assert.Equal(t, events[5].Item.ID, completed.Output[1].ID)
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

	added := filterResponsesEvents(events, "response.output_item.added")
	require.Len(t, added, 2)
	assert.Equal(t, 0, added[0].OutputIndex)
	assert.Equal(t, 1, added[1].OutputIndex)
	completed := events[len(events)-1].Response
	require.NotNil(t, completed)
	require.Len(t, completed.Output, 2)
	assert.Equal(t, "call_3", completed.Output[0].CallID)
	assert.Equal(t, "call_7", completed.Output[1].CallID)
}

func TestChatCompletionsStreamToResponses_MergesFragmentedNameAndCumulativeArguments(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("grok-test")
	emptyFinishReason := ""

	var events []ResponsesStreamEvent
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{ToolCalls: []ChatToolCall{{
			ID: "call_list",
			Function: ChatFunctionCall{
				Name:      "list",
				Arguments: `{"path"`,
			},
		}}}}},
	}, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{
			Index: 0,
			Delta: ChatDelta{ToolCalls: []ChatToolCall{{
				ID: "call_list",
				Function: ChatFunctionCall{
					Name:      "_dir",
					Arguments: `{"path":"."}`,
				},
			}}},
			FinishReason: &emptyFinishReason,
		}},
	}, state)...)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	added := filterResponsesEvents(events, "response.output_item.added")
	require.Len(t, added, 1)
	require.NotNil(t, added[0].Item)
	assert.Equal(t, "list_dir", added[0].Item.Name)
	assert.Equal(t, "call_list", added[0].Item.CallID)

	argumentsDone := filterResponsesEvents(events, "response.function_call_arguments.done")
	require.Len(t, argumentsDone, 1)
	assert.Equal(t, `{"path":"."}`, argumentsDone[0].Arguments)
	assert.JSONEq(t, `{"path":"."}`, argumentsDone[0].Arguments)
}

func TestChatCompletionsStreamToResponses_ParallelCallsWithoutIndexesUseCallIDs(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("grok-test")
	finishReason := "tool_calls"

	var events []ResponsesStreamEvent
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{ToolCalls: []ChatToolCall{
			{ID: "call_a", Function: ChatFunctionCall{Name: "read_file", Arguments: `{"path":`}},
			{ID: "call_b", Function: ChatFunctionCall{Name: "list_dir", Arguments: `{"path":`}},
		}}}},
	}, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{
			Index: 0,
			Delta: ChatDelta{ToolCalls: []ChatToolCall{
				{ID: "call_a", Function: ChatFunctionCall{Arguments: `"README.md"}`}},
				{ID: "call_b", Function: ChatFunctionCall{Arguments: `"."}`}},
			}},
			FinishReason: &finishReason,
		}},
	}, state)...)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	completed := events[len(events)-1].Response
	require.NotNil(t, completed)
	require.Len(t, completed.Output, 2)
	assert.Equal(t, "call_a", completed.Output[0].CallID)
	assert.Equal(t, "read_file", completed.Output[0].Name)
	assert.JSONEq(t, `{"path":"README.md"}`, completed.Output[0].Arguments)
	assert.Equal(t, "call_b", completed.Output[1].CallID)
	assert.Equal(t, "list_dir", completed.Output[1].Name)
	assert.JSONEq(t, `{"path":"."}`, completed.Output[1].Arguments)
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

func TestResponsesToChatCompletionsRequest_GroupsParallelCallsAndPreservesOutputs(t *testing.T) {
	parallel := true
	req := &ResponsesRequest{
		Model:             "grok-test",
		ParallelToolCalls: &parallel,
		Input: json.RawMessage(`[
			{"role":"user","content":"inspect"},
			{"type":"function_call","id":"item_a","call_id":"call_a","name":"read_file","arguments":"{\"path\":\"README.md\"}"},
			{"type":"function_call","id":"item_b","call_id":"call_b","name":"list_dir","arguments":"{\"path\":\".\"}"},
			{"type":"function_call_output","call_id":"call_a","output":"file contents"},
			{"type":"function_call_output","call_id":"call_b","output":{"entries":["README.md"]}}
		]`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out.ParallelToolCalls)
	assert.True(t, *out.ParallelToolCalls)
	require.Len(t, out.Messages, 4)
	assert.Equal(t, "assistant", out.Messages[1].Role)
	require.Len(t, out.Messages[1].ToolCalls, 2)
	assert.Equal(t, "call_a", out.Messages[1].ToolCalls[0].ID)
	assert.Equal(t, "call_b", out.Messages[1].ToolCalls[1].ID)
	assert.Equal(t, "tool", out.Messages[2].Role)
	assert.JSONEq(t, `"file contents"`, string(out.Messages[2].Content))
	assert.Equal(t, "tool", out.Messages[3].Role)
	assert.JSONEq(t, `"{\"entries\":[\"README.md\"]}"`, string(out.Messages[3].Content))
}

func TestResponsesChatBridgePreservesNamespaceCustomAndAdditionalTools(t *testing.T) {
	parallel := true
	req := &ResponsesRequest{
		Model:             "grok-test",
		ParallelToolCalls: &parallel,
		Tools: []ResponsesTool{
			{
				Type: "namespace",
				Name: "mcp__calendar",
				Tools: []ResponsesTool{{
					Type:       "function",
					Name:       "lookup",
					Parameters: json.RawMessage(`{"type":"object","properties":{"date":{"type":"string"}}}`),
				}},
			},
			{Type: "custom", Name: "code", Description: "Run code"},
		},
		Input: json.RawMessage(`[
			{"type":"additional_tools","tools":[{"type":"namespace","name":"collaboration","tools":[{"type":"function","name":"send_message","parameters":{"type":"object"}}]}]},
			{"role":"user","content":"run and notify"},
			{"type":"custom_tool_call","call_id":"call_code","name":"code","input":"print(1)"},
			{"type":"custom_tool_call_output","call_id":"call_code","output":"1"},
			{"type":"function_call","call_id":"call_lookup","namespace":"mcp__calendar","name":"lookup","arguments":"{\"date\":\"today\"}"},
			{"type":"function_call_output","call_id":"call_lookup","output":"free"}
		]`),
	}

	chat, mapping, err := ResponsesToChatCompletionsRequestWithToolMapping(req)
	require.NoError(t, err)
	require.NotNil(t, mapping)
	require.Len(t, chat.Tools, 3)
	assert.Equal(t, "mcp__calendar__lookup", chat.Tools[0].Function.Name)
	assert.Equal(t, "code", chat.Tools[1].Function.Name)
	assert.JSONEq(t, `{"type":"object","properties":{"input":{"type":"string"}},"required":["input"],"additionalProperties":false}`, string(chat.Tools[1].Function.Parameters))
	assert.Equal(t, "collaboration__send_message", chat.Tools[2].Function.Name)
	require.Len(t, chat.Messages, 5)
	assert.Equal(t, "user", chat.Messages[0].Role)
	assert.Equal(t, "assistant", chat.Messages[1].Role)
	assert.Equal(t, "code", chat.Messages[1].ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"input":"print(1)"}`, chat.Messages[1].ToolCalls[0].Function.Arguments)
	assert.Equal(t, "mcp__calendar__lookup", chat.Messages[3].ToolCalls[0].Function.Name)

	response := ChatCompletionsResponseToResponsesWithToolMapping(&ChatCompletionsResponse{
		ID:    "chatcmpl_tools",
		Model: "grok-test",
		Choices: []ChatChoice{{
			Message: ChatMessage{Role: "assistant", ToolCalls: []ChatToolCall{
				{ID: "call_code_2", Type: "function", Function: ChatFunctionCall{Name: "code", Arguments: `{"input":"print(2)"}`}},
				{ID: "call_lookup_2", Type: "function", Function: ChatFunctionCall{Name: "mcp__calendar__lookup", Arguments: `{"date":"tomorrow"}`}},
			}},
			FinishReason: "tool_calls",
		}},
	}, "grok-test", mapping)

	require.Len(t, response.Output, 2)
	assert.Equal(t, "custom_tool_call", response.Output[0].Type)
	assert.Equal(t, "code", response.Output[0].Name)
	assert.Equal(t, "print(2)", response.Output[0].Input)
	assert.Empty(t, response.Output[0].Arguments)
	assert.Equal(t, "function_call", response.Output[1].Type)
	assert.Equal(t, "lookup", response.Output[1].Name)
	assert.Equal(t, "mcp__calendar", response.Output[1].Namespace)
	encoded, err := json.Marshal(response.Output[0])
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"custom_tool_call","id":"`+response.Output[0].ID+`","call_id":"call_code_2","name":"code","input":"print(2)","status":"completed"}`, string(encoded))
}

func TestResponsesChatBridgeStreamingRestoresStrictToolEventShapes(t *testing.T) {
	req := &ResponsesRequest{
		Model: "grok-test",
		Input: json.RawMessage(`"use tools"`),
		Tools: []ResponsesTool{
			{Type: "namespace", Name: "dictionary", Tools: []ResponsesTool{{Type: "function", Name: "listen_dictionary", Parameters: json.RawMessage(`{"type":"object"}`)}}},
			{Type: "custom", Name: "shell"},
		},
	}
	_, mapping, err := ResponsesToChatCompletionsRequestWithToolMapping(req)
	require.NoError(t, err)
	state := NewChatCompletionsToResponsesStreamStateWithToolMapping("grok-test", mapping)
	index0, index1 := 0, 1
	finishReason := "tool_calls"

	var events []ResponsesStreamEvent
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{Choices: []ChatChunkChoice{{
		Delta: ChatDelta{ToolCalls: []ChatToolCall{
			{Index: &index0, ID: "call_dict", Function: ChatFunctionCall{Name: "dictionary__listen_", Arguments: `{"word":`}},
			{Index: &index1, ID: "call_shell", Function: ChatFunctionCall{Name: "shell", Arguments: `{"input":"pw`}},
		}},
	}}}, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{Choices: []ChatChunkChoice{{
		Delta: ChatDelta{ToolCalls: []ChatToolCall{
			{Index: &index0, Function: ChatFunctionCall{Name: "dictionary", Arguments: `"test"}`}},
			{Index: &index1, Function: ChatFunctionCall{Arguments: `d"}`}},
		}},
		FinishReason: &finishReason,
	}}}, state)...)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	added := filterResponsesEvents(events, "response.output_item.added")
	require.Len(t, added, 2)
	assert.Equal(t, "function_call", added[0].Item.Type)
	assert.Equal(t, "listen_dictionary", added[0].Item.Name)
	assert.Equal(t, "dictionary", added[0].Item.Namespace)
	assert.Equal(t, "custom_tool_call", added[1].Item.Type)
	assert.Equal(t, "shell", added[1].Item.Name)
	customDone := filterResponsesEvents(events, "response.custom_tool_call_input.done")
	require.Len(t, customDone, 1)
	assert.Equal(t, "pwd", customDone[0].Input)
	require.Len(t, filterResponsesEvents(events, "response.function_call_arguments.done"), 1)

	completed := events[len(events)-1].Response
	require.NotNil(t, completed)
	require.Len(t, completed.Output, 2)
	assert.Equal(t, "listen_dictionary", completed.Output[0].Name)
	assert.Equal(t, "dictionary", completed.Output[0].Namespace)
	assert.Equal(t, "custom_tool_call", completed.Output[1].Type)
	assert.Equal(t, "pwd", completed.Output[1].Input)
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

func filterResponsesEvents(events []ResponsesStreamEvent, eventType string) []ResponsesStreamEvent {
	filtered := make([]ResponsesStreamEvent, 0)
	for _, event := range events {
		if event.Type == eventType {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
