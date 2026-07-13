package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
)

// ForwardAsResponses serves OpenAI Responses clients through Gemini accounts.
// It reuses the stable Responses <-> Anthropic and Anthropic <-> Gemini bridges
// instead of introducing a parallel Gemini-specific canonical converter.
func (s *GeminiMessagesCompatService) ForwardAsResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*ForwardResult, error) {
	var responsesReq apicompat.ResponsesRequest
	if err := json.Unmarshal(body, &responsesReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "Failed to parse request body"}})
		return nil, err
	}
	if strings.TrimSpace(responsesReq.Model) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "model is required"}})
		return nil, fmt.Errorf("model is required")
	}

	originalModel := responsesReq.Model
	clientStream := responsesReq.Stream
	anthropicReq, err := apicompat.ResponsesToAnthropicRequest(&responsesReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": err.Error()}})
		return nil, err
	}
	anthropicReq.Stream = clientStream

	claudeBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal responses compat request: %w", err)
	}

	bridge := NewProtocolResponseBridge(c, AnthropicBridgeTargetResponses, clientStream, originalModel)
	capture, bridgeErr := NewProtocolBridgeContext(c, ctx, "/v1/messages", claudeBody, bridge)
	if bridgeErr != nil {
		return nil, bridgeErr
	}
	result, forwardErr := s.Forward(ctx, capture, account, claudeBody)
	finalizeErr := bridge.Finalize()
	if forwardErr != nil {
		return result, forwardErr
	}
	if finalizeErr != nil {
		return result, finalizeErr
	}
	return result, nil
}
