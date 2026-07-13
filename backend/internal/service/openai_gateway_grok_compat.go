package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// grokCompatUpstreamRequest contains the normalized Responses request used by
// protocol-router entrypoints (/v1/messages and /v1/chat/completions). Keeping
// this state separate from the downstream protocol conversion allows both
// entrypoints to share Grok Build's stateless reasoning/tool replay semantics.
type grokCompatUpstreamRequest struct {
	body           []byte
	baseBody       []byte
	replayScope    grokReasoningReplayScope
	replayInjected bool
}

func (s *OpenAIGatewayService) prepareGrokCompatUpstreamRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	upstreamModel string,
	promptCacheKey string,
	forceStream bool,
) (*grokCompatUpstreamRequest, error) {
	if account == nil || !account.IsGrok() {
		return nil, fmt.Errorf("grok compatibility request requires a Grok account")
	}

	patched, err := patchGrokResponsesBody(body, upstreamModel, account.GrokUsingAPI())
	if err != nil {
		return nil, err
	}
	if forceStream {
		patched, err = sjson.SetBytes(patched, "stream", true)
		if err != nil {
			return nil, fmt.Errorf("force Grok compatibility stream mode: %w", err)
		}
	}
	if key := strings.TrimSpace(promptCacheKey); key != "" && strings.TrimSpace(gjson.GetBytes(patched, "prompt_cache_key").String()) == "" {
		patched, err = sjson.SetBytes(patched, "prompt_cache_key", key)
		if err != nil {
			return nil, fmt.Errorf("set Grok compatibility prompt cache key: %w", err)
		}
	}

	state := &grokCompatUpstreamRequest{
		body:     append([]byte(nil), patched...),
		baseBody: append([]byte(nil), patched...),
	}
	if !account.GrokUsingAPI() {
		state.body, state.replayScope, state.replayInjected = s.prepareGrokReasoningReplayRequest(ctx, c, state.body, upstreamModel)
	}
	return state, nil
}

// doGrokCompatUpstreamRequest sends one normalized Responses request and
// retries exactly once when a cached encrypted reasoning chain is rejected.
// The returned response body remains readable for endpoint-specific error or
// success conversion.
func (s *OpenAIGatewayService) doGrokCompatUpstreamRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	proxyURL string,
	upstreamModel string,
	state *grokCompatUpstreamRequest,
) (*http.Response, error) {
	if state == nil {
		return nil, fmt.Errorf("grok compatibility request state is nil")
	}

	for attempt := 0; attempt < 2; attempt++ {
		upstreamReq, err := buildGrokResponsesRequest(ctx, c, account, state.body, token)
		if err != nil {
			return nil, err
		}
		resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < http.StatusBadRequest {
			s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
			return resp, nil
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
		if attempt == 0 && state.replayInjected && isGrokInvalidReplayError(resp.StatusCode, respBody) {
			s.clearGrokReasoningReplay(ctx, state.replayScope)
			state.body = append([]byte(nil), state.baseBody...)
			state.body, state.replayScope, _ = s.prepareGrokReasoningReplayRequest(ctx, c, state.body, upstreamModel)
			state.replayInjected = false
			continue
		}
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		return resp, nil
	}
	return nil, fmt.Errorf("grok compatibility request exhausted retry attempts")
}

// normalizeGrokCompatUpstreamResponse converts xAI-specific Responses events
// into the standard event shape consumed by LightBridge's Anthropic and Chat
// conversion handlers while persisting the completed encrypted reasoning/tool
// chain for the next downstream tool-result turn.
func (s *OpenAIGatewayService) normalizeGrokCompatUpstreamResponse(
	ctx context.Context,
	resp *http.Response,
	state *grokCompatUpstreamRequest,
) error {
	if resp == nil || resp.Body == nil || state == nil {
		return nil
	}
	if isEventStreamResponse(resp.Header) {
		resp.Body = xai.NormalizeResponsesSSEStreamWithObserver(resp.Body, func(event []byte) {
			if strings.TrimSpace(gjson.GetBytes(event, "type").String()) == "response.completed" {
				s.cacheGrokReasoningReplay(ctx, state.replayScope, event)
			}
		})
		return nil
	}

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read Grok compatibility response: %w", err)
	}
	if normalized, changed := xai.NormalizeResponsesObject(rawBody); changed {
		rawBody = normalized
	}
	s.cacheGrokReasoningReplay(ctx, state.replayScope, rawBody)
	resp.Body = io.NopCloser(bytes.NewReader(rawBody))
	return nil
}
