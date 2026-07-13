package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func (s *OpenAIGatewayService) forwardGrokResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	originalModel string,
	reqStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	if account.Type != AccountTypeOAuth {
		return nil, fmt.Errorf("grok account type %s is not supported by subscription forwarding", account.Type)
	}

	upstreamModel := account.GetMappedModel(originalModel)
	if strings.TrimSpace(upstreamModel) == "" {
		upstreamModel = "grok-4.3"
	}
	patchedBody, err := patchGrokResponsesBody(body, upstreamModel, account.GrokUsingAPI())
	if err != nil {
		return nil, err
	}
	conversationID := resolveGrokConversationID(c, patchedBody, upstreamModel)
	if conversationID != "" && !gjson.GetBytes(patchedBody, "prompt_cache_key").Exists() {
		patchedBody, _ = sjson.SetBytes(patchedBody, "prompt_cache_key", conversationID)
	}

	basePatchedBody := append([]byte(nil), patchedBody...)
	replayScope := grokReasoningReplayScope{}
	replayInjected := false
	if !account.GrokUsingAPI() {
		patchedBody, replayScope, replayInjected = s.prepareGrokReasoningReplayRequest(ctx, c, patchedBody, upstreamModel)
	}

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamStart := time.Now()
	var resp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		upstreamReq, buildErr := buildGrokResponsesRequest(upstreamCtx, c, account, patchedBody, token)
		if buildErr != nil {
			return nil, buildErr
		}
		resp, err = s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
		if err != nil {
			SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
			return nil, handleGrokUpstreamTransportError(c, account, err)
		}
		if resp.StatusCode < 400 {
			break
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		if attempt == 0 && replayInjected && isGrokInvalidReplayError(resp.StatusCode, respBody) {
			s.clearGrokReasoningReplay(ctx, replayScope)
			patchedBody = append([]byte(nil), basePatchedBody...)
			patchedBody, replayScope, _ = s.prepareGrokReasoningReplayRequest(ctx, c, patchedBody, upstreamModel)
			replayInjected = false
			continue
		}

		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
		s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
		upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
		if upstreamMsg == "" {
			upstreamMsg = fmt.Sprintf("xAI upstream returned status %d", resp.StatusCode)
		}
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
			Kind:               "failover",
			Message:            upstreamMsg,
		})
		s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		return s.handleErrorResponse(ctx, resp, c, account, patchedBody, upstreamModel)
	}
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if resp == nil {
		return nil, errors.New("grok upstream returned no response")
	}
	defer func() { _ = resp.Body.Close() }()

	s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
	actualStream := reqStream || isEventStreamResponse(resp.Header)
	if actualStream {
		resp.Body = xai.NormalizeResponsesSSEStreamWithObserver(resp.Body, func(event []byte) {
			if strings.TrimSpace(gjson.GetBytes(event, "type").String()) == "response.completed" {
				s.cacheGrokReasoningReplay(ctx, replayScope, event)
			}
		})
	} else {
		rawBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read Grok response: %w", readErr)
		}
		if normalized, changed := xai.NormalizeResponsesObject(rawBody); changed {
			rawBody = normalized
		}
		s.cacheGrokReasoningReplay(ctx, replayScope, rawBody)
		resp.Body = io.NopCloser(bytes.NewReader(rawBody))
	}

	var usage *OpenAIUsage
	var firstTokenMs *int
	responseID := ""
	if actualStream {
		streamResult, err := s.handleStreamingResponse(ctx, resp, c, account, startTime, originalModel, upstreamModel)
		if err != nil {
			return nil, err
		}
		usage = streamResult.usage
		firstTokenMs = streamResult.firstTokenMs
		responseID = strings.TrimSpace(streamResult.responseID)
	} else {
		nonStreamResult, err := s.handleNonStreamingResponse(ctx, resp, c, account, originalModel, upstreamModel)
		if err != nil {
			return nil, err
		}
		usage = nonStreamResult.usage
		responseID = strings.TrimSpace(nonStreamResult.responseID)
	}

	if usage == nil {
		usage = &OpenAIUsage{}
	}
	return &OpenAIForwardResult{
		RequestID:       firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
		ResponseID:      responseID,
		Usage:           *usage,
		Model:           originalModel,
		UpstreamModel:   upstreamModel,
		ReasoningEffort: ptrStringOrNil(normalizeOpenAIReasoningEffort(gjson.GetBytes(patchedBody, "reasoning.effort").String())),
		Stream:          actualStream,
		OpenAIWSMode:    false,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
	}, nil
}

func handleGrokUpstreamTransportError(c *gin.Context, account *Account, err error) error {
	safeErr := "upstream request failed"
	if err != nil {
		safeErr = sanitizeUpstreamErrorMessage(err.Error())
	}
	setOpsUpstreamError(c, 0, safeErr, "")
	if account != nil {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.EffectivePlatform(),
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			Kind:               "request_error",
			Message:            safeErr,
		})
	}
	c.JSON(http.StatusBadGateway, gin.H{
		"error": gin.H{
			"type":    "upstream_error",
			"message": "Upstream request failed",
		},
	})
	return fmt.Errorf("grok upstream request failed: %s", safeErr)
}

func patchGrokResponsesBody(body []byte, upstreamModel string, usingAPI bool) ([]byte, error) {
	if usingAPI {
		return xai.PatchOfficialXAIResponsesRequest(body, upstreamModel)
	}
	return xai.PatchGrokBuildResponsesRequest(body, upstreamModel)
}

func buildGrokResponsesRequest(ctx context.Context, c *gin.Context, account *Account, body []byte, token string) (*http.Request, error) {
	if account == nil {
		return nil, fmt.Errorf("grok account is nil")
	}
	usingAPI := account.GrokUsingAPI()
	resolvedBaseURL, err := account.GetGrokChatBaseURL()
	if err != nil {
		return nil, err
	}
	targetURL, err := xai.BuildChatResponsesURL(account.GetCredential("base_url"), usingAPI)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileGrok))
	stream := gjson.GetBytes(body, "stream").Bool()
	xai.ApplyChatHeaders(req, token, stream, usingAPI, resolvedBaseURL, resolveGrokConversationID(c, body, gjson.GetBytes(body, "model").String()))
	if c != nil {
		if v := c.GetHeader("OpenAI-Beta"); strings.TrimSpace(v) != "" {
			req.Header.Set("OpenAI-Beta", v)
		}
	}
	return req, nil
}

func resolveGrokConversationID(c *gin.Context, body []byte, model string) string {
	if value := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()); value != "" {
		return value
	}
	if c != nil {
		for _, header := range []string{xai.GrokConversationHeader, "x-session-id", "x-client-session-id"} {
			if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
				return value
			}
		}
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "grok-composer-") {
		return uuid.NewString()
	}
	return ""
}

func (s *OpenAIGatewayService) updateGrokUsageSnapshot(ctx context.Context, accountID int64, snapshot *xai.QuotaSnapshot) {
	if s == nil || s.accountRepo == nil || accountID <= 0 || snapshot == nil {
		return
	}
	if strings.TrimSpace(snapshot.ObservationSource) == "" {
		snapshot.ObservationSource = "gateway_response"
	}
	if s.codexSnapshotThrottle != nil && !s.codexSnapshotThrottle.Allow(accountID, time.Now()) {
		return
	}
	_ = s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{
		grokQuotaSnapshotExtraKey: snapshot,
	})
}

func (s *OpenAIGatewayService) handleGrokAccountUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte) {
	if s == nil || account == nil {
		return
	}
	switch statusCode {
	case http.StatusUnauthorized:
		s.tempUnscheduleGrok(ctx, account, 10*time.Minute, "grok oauth token unauthorized")
	case http.StatusForbidden:
		s.tempUnscheduleGrok(ctx, account, 30*time.Minute, "grok entitlement or subscription tier denied")
	case http.StatusTooManyRequests:
		cooldown := 2 * time.Minute
		if xai.FreeUsageExhausted(responseBody) {
			cooldown = 24 * time.Hour
		} else if snapshot := xai.ParseQuotaHeaders(headers, statusCode); snapshot != nil && snapshot.RetryAfterSeconds != nil && *snapshot.RetryAfterSeconds > 0 {
			cooldown = time.Duration(*snapshot.RetryAfterSeconds) * time.Second
		}
		s.tempUnscheduleGrok(ctx, account, cooldown, "grok rate limited")
	default:
		if statusCode >= 500 {
			s.tempUnscheduleGrok(ctx, account, 2*time.Minute, "grok upstream temporary error")
		}
	}
	_ = responseBody
}

func (s *OpenAIGatewayService) tempUnscheduleGrok(ctx context.Context, account *Account, cooldown time.Duration, reason string) {
	if s == nil || account == nil {
		return
	}
	until := time.Now().Add(cooldown)
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(until) {
		until = *account.TempUnschedulableUntil
	}
	s.BlockAccountScheduling(account, until, reason)
	if s.accountRepo != nil {
		stateCtx, cancel := openAIAccountStateContext(ctx)
		defer cancel()
		_ = s.accountRepo.SetTempUnschedulable(stateCtx, account.ID, until, reason)
	}
}

func isGrokInvalidReplayError(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest || len(body) == 0 {
		return false
	}
	message := strings.ToLower(string(body))
	if !strings.Contains(message, "reasoning") && !strings.Contains(message, "encrypted_content") && !strings.Contains(message, "function_call") && !strings.Contains(message, "call_id") {
		return false
	}
	for _, signal := range []string{
		"invalid signature",
		"invalid encrypted",
		"encrypted_content",
		"reasoning item",
		"reasoning content",
		"missing function call",
		"function call not found",
		"unknown call_id",
		"no tool call found",
	} {
		if strings.Contains(message, signal) {
			return true
		}
	}
	return false
}

func ptrStringOrNil(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
