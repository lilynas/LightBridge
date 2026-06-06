package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/LightBridge/internal/modules"
	"github.com/gin-gonic/gin"
)

func (s *GatewayService) SetProviderRegistry(registry *modules.ProviderRegistry) {
	if s == nil {
		return
	}
	s.providerRegistry = registry
}

func (s *AccountTestService) SetProviderRegistry(registry *modules.ProviderRegistry) {
	if s == nil {
		return
	}
	s.providerRegistry = registry
}

func (s *GatewayService) forwardModuleProvider(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest, startTime time.Time) (*ForwardResult, bool, error) {
	adapter, providerID, handled, err := s.resolveModuleProviderAdapter(account)
	if err != nil {
		return nil, handled, err
	}
	if !handled {
		return nil, false, nil
	}
	if parsed == nil {
		return nil, true, fmt.Errorf("parse request: empty request")
	}
	downstreamProtocol := strings.TrimSpace(parsed.Protocol)
	if downstreamProtocol == "" {
		downstreamProtocol = strings.TrimSpace(account.Platform)
	}

	events, err := adapter.Forward(ctx, modules.GatewayRequest{
		DownstreamProtocol: downstreamProtocol,
		Endpoint:           requestPath(c),
		Method:             requestMethod(c),
		Headers:            moduleForwardHeaders(c),
		Body:               json.RawMessage(parsed.Body),
		Stream:             parsed.Stream,
		Account:            providerAccountFromService(account, providerID),
		Metadata: map[string]any{
			"model": parsed.Model,
		},
	})
	if err != nil {
		return nil, true, err
	}

	usage := ClaudeUsage{}
	statusCode := http.StatusOK
	requestID := ""
	wroteHeaders := false
	upstreamAccepted := false
	notifyUpstreamAccepted := func() {
		if upstreamAccepted || parsed.OnUpstreamAccepted == nil {
			return
		}
		upstreamAccepted = true
		parsed.OnUpstreamAccepted()
	}

	for event := range events {
		if event.Usage != nil {
			usage.InputTokens = int(event.Usage.InputTokens)
			usage.OutputTokens = int(event.Usage.OutputTokens)
		}
		switch strings.ToLower(strings.TrimSpace(event.Type)) {
		case "headers":
			notifyUpstreamAccepted()
			if event.StatusCode > 0 {
				statusCode = event.StatusCode
			}
			applyModuleGatewayHeaders(c, event.Headers)
			if c != nil && !wroteHeaders {
				c.Status(statusCode)
				wroteHeaders = true
			}
		case "data":
			notifyUpstreamAccepted()
			if c != nil {
				if !wroteHeaders {
					c.Status(statusCode)
					wroteHeaders = true
				}
				if err := writeModuleGatewayData(c, parsed.Stream, event.Data); err != nil {
					return nil, true, err
				}
			}
		case "usage":
		case "error":
			if event.Error != nil {
				if c != nil && !wroteHeaders {
					status := event.Error.StatusCode
					if status == 0 {
						status = http.StatusBadGateway
					}
					c.JSON(status, gin.H{
						"type": "error",
						"error": gin.H{
							"type":    event.Error.Code,
							"message": event.Error.Message,
						},
					})
				}
				return nil, true, fmt.Errorf("provider %s error: %s", providerID, event.Error.Message)
			}
			return nil, true, fmt.Errorf("provider %s returned an error event", providerID)
		case "done":
			if c != nil && parsed.Stream {
				if _, err := c.Writer.Write([]byte("data: [DONE]\n\n")); err != nil {
					return nil, true, err
				}
				c.Writer.Flush()
			}
		}
		if event.Metadata != nil {
			if raw, ok := event.Metadata["request_id"].(string); ok && raw != "" {
				requestID = raw
			}
		}
	}

	return &ForwardResult{
		RequestID: requestID,
		Usage:     usage,
		Model:     parsed.Model,
		Stream:    parsed.Stream,
		Duration:  time.Since(startTime),
	}, true, nil
}

func (s *GatewayService) resolveModuleProviderAdapter(account *Account) (modules.ProviderAdapter, string, bool, error) {
	var registry *modules.ProviderRegistry
	if s != nil {
		registry = s.providerRegistry
	}
	return resolveModuleProviderAdapter(registry, account)
}

func resolveModuleProviderAdapter(registry *modules.ProviderRegistry, account *Account) (modules.ProviderAdapter, string, bool, error) {
	if account == nil {
		return nil, "", false, nil
	}
	if !accountUsesModuleProvider(account) {
		return nil, "", false, nil
	}
	providerID := effectiveServiceProviderID(account)
	if providerID == "" {
		return nil, "", true, fmt.Errorf("module provider account %d has no provider_id", account.ID)
	}
	if registry == nil {
		return nil, providerID, true, fmt.Errorf("provider module registry is not configured")
	}
	adapter, err := registry.Resolve(providerID)
	if err != nil {
		return nil, providerID, true, fmt.Errorf("provider module %q is not registered", providerID)
	}
	return adapter, providerID, true, nil
}

func (s *AccountTestService) testModuleProviderAccount(c *gin.Context, account *Account, modelID string, prompt string, mode string) (bool, error) {
	var registry *modules.ProviderRegistry
	if s != nil {
		registry = s.providerRegistry
	}
	adapter, _, handled, err := resolveModuleProviderAdapter(registry, account)
	if err != nil {
		if c != nil {
			return handled, s.sendErrorAndEnd(c, err.Error())
		}
		return handled, err
	}
	if !handled {
		return false, nil
	}
	if c == nil || c.Request == nil {
		return true, fmt.Errorf("gin context is not available")
	}
	providerID := effectiveServiceProviderID(account)
	s.sendEvent(c, TestEvent{Type: "test_start", Model: modelID})
	moduleAccount := providerAccountFromService(account, providerID)
	if moduleAccount.Metadata == nil {
		moduleAccount.Metadata = make(map[string]any)
	}
	moduleAccount.Metadata["test_model"] = modelID
	moduleAccount.Metadata["test_prompt"] = prompt
	moduleAccount.Metadata["test_mode"] = mode
	result, err := adapter.TestAccount(c.Request.Context(), modules.TestAccountRequest{
		Account: moduleAccount,
		Mode:    mode,
	})
	if err != nil {
		return true, s.sendErrorAndEnd(c, err.Error())
	}
	if result != nil && result.Message != "" {
		s.sendEvent(c, TestEvent{Type: "status", Text: result.Message})
	}
	if result == nil || !result.OK {
		message := "Provider module account test failed"
		if result != nil && result.Message != "" {
			message = result.Message
		}
		return true, s.sendErrorAndEnd(c, message)
	}
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return true, nil
}

func providerAccountFromService(account *Account, providerID string) modules.ProviderAccount {
	config := copyAnyMap(account.Extra)
	if config == nil {
		config = make(map[string]any)
	}
	config["platform"] = account.Platform
	config["type"] = account.Type
	if account.ProxyID != nil && account.Proxy != nil {
		config["proxy_url"] = account.Proxy.URL()
	}
	return modules.ProviderAccount{
		ID:          strconv.FormatInt(account.ID, 10),
		ProviderID:  providerID,
		DisplayName: account.Name,
		Config:      config,
		Secrets:     copyAnyMap(account.Credentials),
		Metadata: map[string]any{
			"platform": account.Platform,
			"type":     account.Type,
		},
	}
}

func effectiveServiceProviderID(account *Account) string {
	if account == nil {
		return ""
	}
	if providerID := strings.TrimSpace(account.ProviderID); providerID != "" {
		return providerID
	}
	if account.Extra != nil {
		if raw, ok := account.Extra["provider_id"].(string); ok {
			if providerID := strings.TrimSpace(raw); providerID != "" {
				return providerID
			}
		}
	}
	if accountUsesModuleProvider(account) {
		return ""
	}
	return strings.TrimSpace(account.Platform)
}

func accountUsesModuleProvider(account *Account) bool {
	if account == nil {
		return false
	}
	if account.Type == AccountTypeModule {
		return true
	}
	if account.Platform == PlatformModule {
		return true
	}
	if account.Extra != nil {
		if raw, ok := account.Extra["module_id"].(string); ok && strings.TrimSpace(raw) != "" {
			return true
		}
	}
	return false
}

func copyAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func requestPath(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return c.Request.URL.Path
}

func requestMethod(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return http.MethodPost
	}
	return c.Request.Method
}

func moduleForwardHeaders(c *gin.Context) map[string][]string {
	if c == nil || c.Request == nil {
		return nil
	}
	out := make(map[string][]string)
	for key, values := range c.Request.Header {
		lower := strings.ToLower(strings.TrimSpace(key))
		switch lower {
		case "authorization", "cookie", "set-cookie":
			continue
		}
		out[key] = append([]string(nil), values...)
	}
	return out
}

func applyModuleGatewayHeaders(c *gin.Context, headers map[string][]string) {
	if c == nil || len(headers) == 0 {
		return
	}
	for key, values := range headers {
		lower := strings.ToLower(strings.TrimSpace(key))
		switch lower {
		case "authorization", "content-length", "connection", "cookie", "proxy-authenticate", "set-cookie", "transfer-encoding", "www-authenticate":
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
}

func writeModuleGatewayData(c *gin.Context, stream bool, data json.RawMessage) error {
	if c == nil {
		return nil
	}
	if stream {
		if c.Writer.Header().Get("Content-Type") == "" {
			c.Writer.Header().Set("Content-Type", "text/event-stream")
		}
		if _, err := c.Writer.Write([]byte("data: ")); err != nil {
			return err
		}
		if _, err := c.Writer.Write(data); err != nil {
			return err
		}
		if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}
	if _, err := c.Writer.Write(data); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}
