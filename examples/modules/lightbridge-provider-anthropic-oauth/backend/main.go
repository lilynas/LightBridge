package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

const (
	providerAdapterService = "lightbridge.modules.ProviderAdapter"
	jsonCodecName          = "json"
	moduleID               = "anthropic-oauth"

	anthropicAPIBaseURL       = "https://api.anthropic.com"
	anthropicMessagesURL      = "https://api.anthropic.com/v1/messages?beta=true"
	anthropicCountTokensURL   = "https://api.anthropic.com/v1/messages/count_tokens?beta=true"
	claudeBaseURL             = "https://claude.ai"
	claudeOAuthAuthorizeURL   = "https://claude.ai/oauth/authorize"
	claudeOAuthTokenURL       = "https://platform.claude.com/v1/oauth/token"
	claudeOAuthClientID       = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	claudeOAuthRedirectURI    = "https://platform.claude.com/oauth/code/callback"
	claudeOAuthScopeFull      = "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"
	claudeOAuthScopeAPI       = "user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"
	claudeOAuthScopeInference = "user:inference"
	anthropicVersion          = "2023-06-01"
	betaOAuth                 = "oauth-2025-04-20"
	betaClaudeCode            = "claude-code-20250219"
	betaInterleavedThinking   = "interleaved-thinking-2025-05-14"
	betaFineGrainedToolStream = "fine-grained-tool-streaming-2025-05-14"
	betaTokenCounting         = "token-counting-2024-11-01"
	betaPromptCachingScope    = "prompt-caching-scope-2026-01-05"
	betaEffort                = "effort-2025-11-24"
	betaContextManagement     = "context-management-2025-06-27"
	betaExtendedCacheTTL      = "extended-cache-ttl-2025-04-11"
	defaultTestModel          = "claude-sonnet-4-5-20250929"
)

const (
	defaultBetaHeader     = betaClaudeCode + "," + betaOAuth + "," + betaInterleavedThinking + "," + betaFineGrainedToolStream
	messageBetaHeader     = betaClaudeCode + "," + betaOAuth + "," + betaInterleavedThinking + "," + betaPromptCachingScope + "," + betaEffort + "," + betaContextManagement + "," + betaExtendedCacheTTL
	countTokensBetaHeader = betaClaudeCode + "," + betaOAuth + "," + betaInterleavedThinking + "," + betaTokenCounting
)

var claudeDefaultHeaders = map[string]string{
	"User-Agent":                                "claude-cli/2.1.92 (external, cli)",
	"X-Stainless-Lang":                          "js",
	"X-Stainless-Package-Version":               "0.70.0",
	"X-Stainless-OS":                            "Linux",
	"X-Stainless-Arch":                          "arm64",
	"X-Stainless-Runtime":                       "node",
	"X-Stainless-Runtime-Version":               "v24.13.0",
	"X-Stainless-Retry-Count":                   "0",
	"X-Stainless-Timeout":                       "600",
	"X-App":                                     "cli",
	"Anthropic-Dangerous-Direct-Browser-Access": "true",
}

var modelIDOverrides = map[string]string{
	"claude-sonnet-4-5": "claude-sonnet-4-5-20250929",
	"claude-opus-4-5":   "claude-opus-4-5-20251101",
	"claude-haiku-4-5":  "claude-haiku-4-5-20251001",
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return jsonCodecName }

func (jsonCodec) Marshal(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	if len(data) == 0 || v == nil {
		return nil
	}
	return json.Unmarshal(data, v)
}

type emptyMessage struct{}

type providerService interface {
	mustEmbedUnimplementedProviderService()
}

type anthropicOAuthProvider struct{}

func (anthropicOAuthProvider) mustEmbedUnimplementedProviderService() {}

type ProviderMetadata struct {
	ID              string          `json:"id"`
	DisplayName     string          `json:"display_name"`
	Supports        map[string]bool `json:"supports"`
	CredentialTypes []string        `json:"credential_types,omitempty"`
	Extra           map[string]any  `json:"extra,omitempty"`
}

type ModelInfo struct {
	ID           string          `json:"id"`
	DisplayName  string          `json:"display_name,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	Metadata     map[string]any  `json:"metadata,omitempty"`
}

type ListModelsRequest struct {
	CredentialRef string         `json:"credential_ref,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
}

type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

type ProviderAccount struct {
	ID            string         `json:"id,omitempty"`
	ProviderID    string         `json:"provider_id,omitempty"`
	DisplayName   string         `json:"display_name,omitempty"`
	CredentialRef string         `json:"credential_ref,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	Secrets       map[string]any `json:"secrets,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type AccountValidationResult struct {
	Valid    bool           `json:"valid"`
	Warnings []string       `json:"warnings,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type TestAccountRequest struct {
	Account ProviderAccount `json:"account"`
	Mode    string          `json:"mode,omitempty"`
}

type TestAccountResult struct {
	OK       bool           `json:"ok"`
	Message  string         `json:"message,omitempty"`
	Latency  *DurationSpec  `json:"latency,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type DurationSpec struct {
	Seconds int64 `json:"seconds"`
	Nanos   int32 `json:"nanos"`
}

type UpstreamError struct {
	StatusCode int            `json:"status_code,omitempty"`
	Code       string         `json:"code,omitempty"`
	Message    string         `json:"message,omitempty"`
	Headers    map[string]any `json:"headers,omitempty"`
	Body       any            `json:"body,omitempty"`
}

type NormalizedError struct {
	Retryable   bool           `json:"retryable"`
	StatusCode  int            `json:"status_code,omitempty"`
	Code        string         `json:"code,omitempty"`
	Message     string         `json:"message,omitempty"`
	ProviderRaw map[string]any `json:"provider_raw,omitempty"`
}

type GatewayRequest struct {
	DownstreamProtocol string              `json:"downstream_protocol,omitempty"`
	Endpoint           string              `json:"endpoint"`
	Method             string              `json:"method,omitempty"`
	Headers            map[string][]string `json:"headers,omitempty"`
	Body               json.RawMessage     `json:"body,omitempty"`
	Stream             bool                `json:"stream"`
	Account            ProviderAccount     `json:"account,omitempty"`
	Metadata           map[string]any      `json:"metadata,omitempty"`
}

type GatewayEvent struct {
	Type       string              `json:"type"`
	StatusCode int                 `json:"status_code,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Data       json.RawMessage     `json:"data,omitempty"`
	Usage      *TokenUsage         `json:"usage,omitempty"`
	Error      *NormalizedError    `json:"error,omitempty"`
	Metadata   map[string]any      `json:"metadata,omitempty"`
}

type TokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens,omitempty"`
	TotalTokens  int64 `json:"total_tokens"`
}

type TokenCountRequest struct {
	Model    string          `json:"model"`
	Messages []any           `json:"messages,omitempty"`
	Input    any             `json:"input,omitempty"`
	Config   map[string]any  `json:"config,omitempty"`
	Account  ProviderAccount `json:"account,omitempty"`
}

type TokenCountResponse struct {
	Usage TokenUsage `json:"usage"`
}

type tokenResponse struct {
	AccessToken  string       `json:"access_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int64        `json:"expires_in"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	Scope        string       `json:"scope,omitempty"`
	Organization *orgInfo     `json:"organization,omitempty"`
	Account      *accountInfo `json:"account,omitempty"`
}

type orgInfo struct {
	UUID string `json:"uuid"`
}

type accountInfo struct {
	UUID         string `json:"uuid"`
	EmailAddress string `json:"email_address"`
}

func main() {
	socketPath := strings.TrimSpace(os.Getenv("LIGHTBRIDGE_MODULE_SOCKET"))
	if socketPath == "" {
		fatal("LIGHTBRIDGE_MODULE_SOCKET is required")
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		fatal("create socket dir: %v", err)
	}
	if err := removeStaleSocket(socketPath); err != nil {
		fatal("remove stale socket: %v", err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fatal("listen provider socket: %v", err)
	}
	defer func() { _ = os.Remove(socketPath) }()

	server := grpc.NewServer(grpc.ForceServerCodec(jsonCodec{}))
	registerProviderService(server)
	if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		fatal("serve provider socket: %v", err)
	}
}

func fatal(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func removeStaleSocket(socketPath string) error {
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s exists and is not a socket", socketPath)
	}
	return os.Remove(socketPath)
}

func registerProviderService(server *grpc.Server) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: providerAdapterService,
		HandlerType: (*providerService)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Metadata", Handler: metadataHandler},
			{MethodName: "HealthCheck", Handler: healthCheckHandler},
			{MethodName: "ListModels", Handler: listModelsHandler},
			{MethodName: "ValidateAccount", Handler: validateAccountHandler},
			{MethodName: "RefreshAccount", Handler: refreshAccountHandler},
			{MethodName: "TestAccount", Handler: testAccountHandler},
			{MethodName: "NormalizeError", Handler: normalizeErrorHandler},
			{MethodName: "CountTokens", Handler: countTokensHandler},
		},
		Streams: []grpc.StreamDesc{{
			StreamName:    "Forward",
			Handler:       forwardHandler,
			ServerStreams: true,
			ClientStreams: true,
		}},
	}, anthropicOAuthProvider{})
}

func metadataHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(emptyMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) {
		return ProviderMetadata{
			ID:              moduleID,
			DisplayName:     "Anthropic OAuth Provider",
			CredentialTypes: []string{"oauth", "setup_token", "session_key"},
			Supports:        map[string]bool{"chat": true, "stream": true, "messages": true, "tools": true, "vision": true, "count_tokens": true},
			Extra: map[string]any{
				"downstream_protocols": []string{"anthropic"},
				"oauth_authorize_url":  claudeOAuthAuthorizeURL,
				"oauth_client_id":      claudeOAuthClientID,
				"oauth_scopes": []string{
					claudeOAuthScopeFull,
					claudeOAuthScopeInference,
				},
				"oauth_redirect_uri": claudeOAuthRedirectURI,
				"endpoints": []string{
					"/v1/messages",
					"/v1/messages/count_tokens",
				},
			},
		}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("Metadata")}, handler)
}

func healthCheckHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(emptyMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) { return emptyMessage{}, nil }
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("HealthCheck")}, handler)
}

func listModelsHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(ListModelsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) {
		return ListModelsResponse{Models: []ModelInfo{
			{ID: "claude-opus-4-8", DisplayName: "Claude Opus 4.8", Capabilities: map[string]bool{"chat": true, "stream": true, "tools": true, "vision": true}},
			{ID: "claude-opus-4-7", DisplayName: "Claude Opus 4.7", Capabilities: map[string]bool{"chat": true, "stream": true, "tools": true, "vision": true}},
			{ID: "claude-opus-4-6", DisplayName: "Claude Opus 4.6", Capabilities: map[string]bool{"chat": true, "stream": true, "tools": true, "vision": true}},
			{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6", Capabilities: map[string]bool{"chat": true, "stream": true, "tools": true, "vision": true}},
			{ID: "claude-sonnet-4-5", DisplayName: "Claude Sonnet 4.5", Capabilities: map[string]bool{"chat": true, "stream": true, "tools": true, "vision": true}},
			{ID: "claude-haiku-4-5", DisplayName: "Claude Haiku 4.5", Capabilities: map[string]bool{"chat": true, "stream": true, "tools": true, "vision": true}},
		}}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("ListModels")}, handler)
}

func validateAccountHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(ProviderAccount)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req any) (any, error) {
		account := req.(*ProviderAccount)
		if accessToken(account) != "" {
			return AccountValidationResult{Valid: true, Metadata: map[string]any{"provider_id": moduleID, "auth_type": "oauth"}}, nil
		}
		if secretString(account.Secrets, "refresh_token") != "" {
			token, err := refreshClaudeToken(ctx, secretString(account.Secrets, "refresh_token"), proxyURL(account))
			if err != nil {
				return AccountValidationResult{Valid: false, Warnings: []string{sanitizeMessage(err.Error())}}, nil
			}
			return AccountValidationResult{Valid: token.AccessToken != "", Metadata: map[string]any{"provider_id": moduleID, "auth_type": "oauth_refresh"}}, nil
		}
		if secretString(account.Secrets, "authorization_code") != "" && secretString(account.Secrets, "code_verifier") != "" {
			return AccountValidationResult{Valid: true, Metadata: map[string]any{"provider_id": moduleID, "auth_type": "authorization_code"}}, nil
		}
		if secretString(account.Secrets, "session_key") != "" {
			return AccountValidationResult{Valid: true, Metadata: map[string]any{"provider_id": moduleID, "auth_type": "session_key"}}, nil
		}
		return AccountValidationResult{Valid: false, Warnings: []string{"access_token, refresh_token, authorization_code, or session_key is required"}}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("ValidateAccount")}, handler)
}

func refreshAccountHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(ProviderAccount)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req any) (any, error) {
		account := *req.(*ProviderAccount)
		token, err := resolveClaudeToken(ctx, &account)
		if err != nil {
			return nil, err
		}
		applyTokenResponse(&account, token)
		delete(account.Secrets, "authorization_code")
		delete(account.Secrets, "code_verifier")
		if secretString(account.Secrets, "session_key") != "" {
			account.Secrets["session_key_present"] = true
			delete(account.Secrets, "session_key")
		}
		return account, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("RefreshAccount")}, handler)
}

func testAccountHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(TestAccountRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req any) (any, error) {
		start := time.Now()
		request := req.(*TestAccountRequest)
		result, err := testClaudeAccount(ctx, &request.Account)
		if err != nil {
			return TestAccountResult{OK: false, Message: sanitizeMessage(err.Error()), Latency: durationSpec(time.Since(start))}, nil
		}
		result.Latency = durationSpec(time.Since(start))
		return result, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("TestAccount")}, handler)
}

func normalizeErrorHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(UpstreamError)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(_ context.Context, req any) (any, error) {
		upstream := req.(*UpstreamError)
		code := strings.TrimSpace(upstream.Code)
		if code == "" {
			code = "anthropic_oauth_error"
		}
		return NormalizedError{
			Retryable:  upstream.StatusCode == 408 || upstream.StatusCode == 409 || upstream.StatusCode == 429 || upstream.StatusCode >= 500,
			StatusCode: upstream.StatusCode,
			Code:       code,
			Message:    sanitizeMessage(upstream.Message),
			ProviderRaw: map[string]any{
				"provider": moduleID,
			},
		}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("NormalizeError")}, handler)
}

func forwardHandler(_ any, stream grpc.ServerStream) error {
	var req GatewayRequest
	if err := stream.RecvMsg(&req); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	ctx := stream.Context()
	if err := forwardClaude(ctx, stream, req); err != nil {
		return stream.SendMsg(&GatewayEvent{
			Type: "error",
			Error: &NormalizedError{
				Retryable:  false,
				StatusCode: http.StatusBadGateway,
				Code:       "anthropic_oauth_forward_failed",
				Message:    sanitizeMessage(err.Error()),
			},
		})
	}
	return nil
}

func forwardClaude(ctx context.Context, stream grpc.ServerStream, req GatewayRequest) error {
	token, err := tokenForRequest(ctx, &req.Account)
	if err != nil {
		return err
	}
	targetURL, body, isCountTokens, err := prepareAnthropicUpstreamRequest(req)
	if err != nil {
		return err
	}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = http.MethodPost
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	applyAnthropicHeaders(httpReq, req, token, isCountTokens)
	client := httpClient(proxyURL(&req.Account))
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	headers := sanitizeHeaders(resp.Header)
	if err := stream.SendMsg(&GatewayEvent{Type: "headers", StatusCode: resp.StatusCode, Headers: headers}); err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return stream.SendMsg(&GatewayEvent{
			Type: "error",
			Error: &NormalizedError{
				Retryable:  resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500,
				StatusCode: resp.StatusCode,
				Code:       "anthropic_upstream_error",
				Message:    sanitizeMessage(extractAnthropicError(body)),
			},
		})
	}
	if req.Stream || strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return forwardSSE(stream, resp.Body)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	usage := usageFromJSON(respBody)
	if err := stream.SendMsg(&GatewayEvent{Type: "data", Data: json.RawMessage(respBody), Usage: usage}); err != nil {
		return err
	}
	if usage != nil {
		if err := stream.SendMsg(&GatewayEvent{Type: "usage", Usage: usage}); err != nil {
			return err
		}
	}
	return stream.SendMsg(&GatewayEvent{Type: "done"})
}

func forwardSSE(stream grpc.ServerStream, body io.Reader) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var merged TokenUsage
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ":") {
			continue
		}
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if payload == "[DONE]" {
			return stream.SendMsg(&GatewayEvent{Type: "done"})
		}
		raw := json.RawMessage(payload)
		eventUsage := usageFromJSON(raw)
		if eventUsage != nil {
			mergeUsage(&merged, eventUsage)
		}
		if err := stream.SendMsg(&GatewayEvent{Type: "data", Data: raw, Usage: eventUsage}); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if merged.InputTokens > 0 || merged.OutputTokens > 0 {
		merged.TotalTokens = merged.InputTokens + merged.OutputTokens
		if err := stream.SendMsg(&GatewayEvent{Type: "usage", Usage: &merged}); err != nil {
			return err
		}
	}
	return stream.SendMsg(&GatewayEvent{Type: "done"})
}

func prepareAnthropicUpstreamRequest(req GatewayRequest) (string, []byte, bool, error) {
	endpoint := normalizeEndpoint(req.Endpoint)
	body := normalizeModelInBody(append([]byte(nil), req.Body...))
	base := strings.TrimRight(baseURL(&req.Account), "/")
	switch {
	case strings.Contains(endpoint, "/count_tokens"):
		return base + "/v1/messages/count_tokens?beta=true", sanitizeCountTokensRequestBody(body), true, nil
	case endpoint == "" || endpoint == "/" || strings.HasSuffix(endpoint, "/messages"):
		return base + "/v1/messages?beta=true", body, false, nil
	default:
		return base + endpoint, body, strings.Contains(endpoint, "count_tokens"), nil
	}
}

func applyAnthropicHeaders(httpReq *http.Request, req GatewayRequest, token string, isCountTokens bool) {
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("Accept", "text/event-stream, application/json")
	for key, value := range claudeDefaultHeaders {
		httpReq.Header.Set(key, value)
	}
	for key, values := range req.Headers {
		if !forwardableHeader(key) {
			continue
		}
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}
	httpReq.Header.Del("x-api-key")
	httpReq.Header.Del("cookie")
	httpReq.Header.Del("authorization")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	if isCountTokens {
		httpReq.Header.Set("anthropic-beta", countTokensBetaHeader)
		return
	}
	if strings.TrimSpace(httpReq.Header.Get("anthropic-beta")) == "" {
		httpReq.Header.Set("anthropic-beta", messageBetaHeader)
	}
}

func resolveClaudeToken(ctx context.Context, account *ProviderAccount) (*tokenResponse, error) {
	if code := secretString(account.Secrets, "authorization_code"); code != "" {
		return exchangeClaudeCode(ctx, code, secretString(account.Secrets, "code_verifier"), secretString(account.Secrets, "oauth_state"), proxyURL(account), isSetupToken(account))
	}
	if sessionKey := secretString(account.Secrets, "session_key"); sessionKey != "" {
		scope := claudeOAuthScopeAPI
		if isSetupToken(account) {
			scope = claudeOAuthScopeInference
		}
		codeVerifier, err := generateCodeVerifier()
		if err != nil {
			return nil, err
		}
		state, err := generateState()
		if err != nil {
			return nil, err
		}
		codeChallenge := generateCodeChallenge(codeVerifier)
		orgUUID, err := getOrganizationUUID(ctx, sessionKey, proxyURL(account))
		if err != nil {
			return nil, err
		}
		code, err := getAuthorizationCode(ctx, sessionKey, orgUUID, scope, codeChallenge, state, proxyURL(account))
		if err != nil {
			return nil, err
		}
		return exchangeClaudeCode(ctx, code, codeVerifier, state, proxyURL(account), isSetupToken(account))
	}
	refresh := secretString(account.Secrets, "refresh_token")
	if refresh == "" {
		return nil, errors.New("refresh_token, authorization_code, or session_key is required")
	}
	return refreshClaudeToken(ctx, refresh, proxyURL(account))
}

func tokenForRequest(ctx context.Context, account *ProviderAccount) (string, error) {
	if token := accessToken(account); token != "" {
		return token, nil
	}
	token, err := resolveClaudeToken(ctx, account)
	if err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", errors.New("token response did not include access_token")
	}
	return token.AccessToken, nil
}

func exchangeClaudeCode(ctx context.Context, code, codeVerifier, state, proxyRaw string, setupToken bool) (*tokenResponse, error) {
	authCode, codeState := parseAuthorizationCode(code)
	if codeState != "" {
		state = codeState
	}
	body := map[string]any{
		"code":          authCode,
		"grant_type":    "authorization_code",
		"client_id":     claudeOAuthClientID,
		"redirect_uri":  claudeOAuthRedirectURI,
		"code_verifier": codeVerifier,
	}
	if state != "" {
		body["state"] = state
	}
	if setupToken {
		body["expires_in"] = 31536000
	}
	return doClaudeTokenRequest(ctx, body, proxyRaw, "token exchange")
}

func refreshClaudeToken(ctx context.Context, refreshToken, proxyRaw string) (*tokenResponse, error) {
	body := map[string]any{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     claudeOAuthClientID,
	}
	return doClaudeTokenRequest(ctx, body, proxyRaw, "token refresh")
}

func doClaudeTokenRequest(ctx context.Context, body map[string]any, proxyRaw, operation string) (*tokenResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeOAuthTokenURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "axios/1.13.6")
	resp, err := httpClient(proxyRaw).Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s failed: status %d: %s", operation, resp.StatusCode, sanitizeMessage(string(respBody)))
	}
	var out tokenResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func getOrganizationUUID(ctx context.Context, sessionKey, proxyRaw string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, claudeBaseURL+"/api/organizations", nil)
	if err != nil {
		return "", err
	}
	req.AddCookie(&http.Cookie{Name: "sessionKey", Value: sessionKey})
	resp, err := httpClient(proxyRaw).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("get organizations failed: status %d: %s", resp.StatusCode, sanitizeMessage(string(body)))
	}
	var orgs []struct {
		UUID      string  `json:"uuid"`
		Name      string  `json:"name"`
		RavenType *string `json:"raven_type"`
	}
	if err := json.Unmarshal(body, &orgs); err != nil {
		return "", err
	}
	if len(orgs) == 0 {
		return "", errors.New("no Claude organizations found")
	}
	for _, org := range orgs {
		if org.RavenType != nil && *org.RavenType == "team" && org.UUID != "" {
			return org.UUID, nil
		}
	}
	if orgs[0].UUID == "" {
		return "", errors.New("Claude organization UUID is empty")
	}
	return orgs[0].UUID, nil
}

func getAuthorizationCode(ctx context.Context, sessionKey, orgUUID, scope, codeChallenge, state, proxyRaw string) (string, error) {
	target := fmt.Sprintf("%s/v1/oauth/%s/authorize", claudeBaseURL, orgUUID)
	reqBody := map[string]any{
		"response_type":         "code",
		"client_id":             claudeOAuthClientID,
		"organization_uuid":     orgUUID,
		"redirect_uri":          claudeOAuthRedirectURI,
		"scope":                 scope,
		"state":                 state,
		"code_challenge":        codeChallenge,
		"code_challenge_method": "S256",
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.AddCookie(&http.Cookie{Name: "sessionKey", Value: sessionKey})
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Origin", "https://claude.ai")
	req.Header.Set("Referer", "https://claude.ai/new")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient(proxyRaw).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("get authorization code failed: status %d: %s", resp.StatusCode, sanitizeMessage(string(body)))
	}
	var result struct {
		RedirectURI string `json:"redirect_uri"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.RedirectURI == "" {
		return "", errors.New("no redirect_uri in Claude authorization response")
	}
	parsed, err := url.Parse(result.RedirectURI)
	if err != nil {
		return "", err
	}
	authCode := parsed.Query().Get("code")
	responseState := parsed.Query().Get("state")
	if authCode == "" {
		return "", errors.New("no authorization code in Claude redirect_uri")
	}
	if responseState != "" {
		return authCode + "#" + responseState, nil
	}
	return authCode, nil
}

func applyTokenResponse(account *ProviderAccount, token *tokenResponse) {
	if account.Secrets == nil {
		account.Secrets = map[string]any{}
	}
	account.Secrets["access_token"] = token.AccessToken
	if token.RefreshToken != "" {
		account.Secrets["refresh_token"] = token.RefreshToken
	}
	if account.Metadata == nil {
		account.Metadata = map[string]any{}
	}
	account.Metadata["refreshed_at"] = time.Now().UTC().Format(time.RFC3339)
	account.Metadata["expires_in"] = token.ExpiresIn
	account.Metadata["expires_at"] = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	account.Metadata["scope"] = token.Scope
	if token.Organization != nil && token.Organization.UUID != "" {
		account.Metadata["org_uuid"] = token.Organization.UUID
	}
	if token.Account != nil {
		if token.Account.UUID != "" {
			account.Metadata["account_uuid"] = token.Account.UUID
		}
		if token.Account.EmailAddress != "" {
			account.Metadata["email_address"] = token.Account.EmailAddress
		}
	}
}

func testClaudeAccount(ctx context.Context, account *ProviderAccount) (TestAccountResult, error) {
	token, err := tokenForRequest(ctx, account)
	if err != nil {
		return TestAccountResult{OK: false, Message: sanitizeMessage(err.Error())}, nil
	}
	body, err := json.Marshal(map[string]any{
		"model":      defaultTestModel,
		"max_tokens": 8,
		"stream":     false,
		"messages": []map[string]any{{
			"role":    "user",
			"content": "ping",
		}},
	})
	if err != nil {
		return TestAccountResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL, bytes.NewReader(body))
	if err != nil {
		return TestAccountResult{}, err
	}
	applyAnthropicHeaders(httpReq, GatewayRequest{Account: *account}, token, false)
	resp, err := httpClient(proxyURL(account)).Do(httpReq)
	if err != nil {
		return TestAccountResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TestAccountResult{OK: false, Message: fmt.Sprintf("Anthropic API returned %d: %s", resp.StatusCode, sanitizeMessage(extractAnthropicError(respBody)))}, nil
	}
	return TestAccountResult{OK: true, Message: "Anthropic OAuth account is usable", Metadata: map[string]any{"provider_id": moduleID, "model": defaultTestModel}}, nil
}

func countTokensHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(TokenCountRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req any) (any, error) {
		request := req.(*TokenCountRequest)
		body, err := json.Marshal(map[string]any{"model": request.Model, "messages": request.Messages})
		if err != nil {
			return nil, err
		}
		account := request.Account
		account.Config = mergeMaps(account.Config, request.Config)
		gatewayReq := GatewayRequest{
			Endpoint: "/v1/messages/count_tokens",
			Method:   http.MethodPost,
			Body:     body,
			Account:  account,
		}
		collector := &collectingServerStream{ctx: ctx}
		if err := forwardClaude(ctx, collector, gatewayReq); err != nil {
			return nil, err
		}
		var out struct {
			InputTokens int64 `json:"input_tokens"`
		}
		if len(collector.lastData) > 0 {
			_ = json.Unmarshal(collector.lastData, &out)
		}
		return TokenCountResponse{Usage: TokenUsage{InputTokens: out.InputTokens, TotalTokens: out.InputTokens}}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("CountTokens")}, handler)
}

type collectingServerStream struct {
	grpc.ServerStream
	ctx      context.Context
	lastData json.RawMessage
}

func (s *collectingServerStream) Context() context.Context { return s.ctx }

func (s *collectingServerStream) SendMsg(m any) error {
	if event, ok := m.(*GatewayEvent); ok && event.Type == "data" {
		s.lastData = append(json.RawMessage(nil), event.Data...)
	}
	return nil
}

func normalizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || endpoint == "/" {
		return "/v1/messages"
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	return endpoint
}

func normalizeModelInBody(body []byte) []byte {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return body
	}
	model := strings.TrimSpace(stringFromMap(root, "model"))
	if mapped, ok := modelIDOverrides[model]; ok {
		root["model"] = mapped
		out, err := json.Marshal(root)
		if err == nil {
			return out
		}
	}
	return body
}

func sanitizeCountTokensRequestBody(body []byte) []byte {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return body
	}
	for _, key := range []string{"stream", "temperature", "top_p", "top_k", "stop_sequences", "max_tokens"} {
		delete(root, key)
	}
	out, err := json.Marshal(root)
	if err != nil {
		return body
	}
	return out
}

func parseAuthorizationCode(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if parsed, err := url.Parse(raw); err == nil && parsed.Query().Get("code") != "" {
		return parsed.Query().Get("code"), parsed.Query().Get("state")
	}
	if idx := strings.Index(raw, "#"); idx >= 0 {
		return raw[:idx], raw[idx+1:]
	}
	return raw, ""
}

func buildAuthorizationURL(state, codeChallenge, scope string) string {
	encodedRedirectURI := url.QueryEscape(claudeOAuthRedirectURI)
	encodedScope := strings.ReplaceAll(url.QueryEscape(scope), "%20", "+")
	return fmt.Sprintf("%s?code=true&client_id=%s&response_type=code&redirect_uri=%s&scope=%s&code_challenge=%s&code_challenge_method=S256&state=%s",
		claudeOAuthAuthorizeURL,
		claudeOAuthClientID,
		encodedRedirectURI,
		encodedScope,
		codeChallenge,
		state,
	)
}

func generateState() (string, error) {
	bytes, err := randomBytes(32)
	if err != nil {
		return "", err
	}
	return base64URLEncode(bytes), nil
}

func generateCodeVerifier() (string, error) {
	bytes, err := randomBytes(32)
	if err != nil {
		return "", err
	}
	return base64URLEncode(bytes), nil
}

func generateCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64URLEncode(sum[:])
}

func randomBytes(n int) ([]byte, error) {
	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	return bytes, err
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func accessToken(account *ProviderAccount) string {
	if account == nil {
		return ""
	}
	return strings.TrimSpace(secretString(account.Secrets, "access_token"))
}

func proxyURL(account *ProviderAccount) string {
	if account == nil {
		return ""
	}
	return firstNonEmpty(
		stringFromMap(account.Config, "proxy_url"),
		stringFromMap(account.Metadata, "proxy_url"),
	)
}

func baseURL(account *ProviderAccount) string {
	if account == nil {
		return anthropicAPIBaseURL
	}
	if v := strings.TrimRight(stringFromMap(account.Config, "base_url"), "/"); v != "" {
		return v
	}
	if v := strings.TrimRight(stringFromMap(account.Metadata, "base_url"), "/"); v != "" {
		return v
	}
	return anthropicAPIBaseURL
}

func isSetupToken(account *ProviderAccount) bool {
	if account == nil {
		return false
	}
	if boolFromMap(account.Config, "setup_token") || boolFromMap(account.Metadata, "setup_token") {
		return true
	}
	scope := firstNonEmpty(stringFromMap(account.Config, "oauth_scope"), stringFromMap(account.Metadata, "oauth_scope"), secretString(account.Secrets, "scope"))
	return strings.TrimSpace(scope) == claudeOAuthScopeInference
}

func httpClient(proxyRaw string) *http.Client {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	proxyRaw = strings.TrimSpace(proxyRaw)
	if proxyRaw != "" {
		if parsed, err := url.Parse(proxyRaw); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return &http.Client{Timeout: 120 * time.Second, Transport: transport}
}

func forwardableHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "", "authorization", "x-api-key", "cookie", "set-cookie", "content-length", "connection", "host":
		return false
	default:
		return true
	}
}

func sanitizeHeaders(headers http.Header) map[string][]string {
	out := map[string][]string{}
	for key, values := range headers {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "authorization", "x-api-key", "cookie", "set-cookie", "www-authenticate":
			continue
		default:
			out[key] = append([]string(nil), values...)
		}
	}
	return out
}

func extractAnthropicError(body []byte) string {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err == nil {
		if errObj, ok := root["error"].(map[string]any); ok {
			if msg, ok := errObj["message"].(string); ok && strings.TrimSpace(msg) != "" {
				return msg
			}
		}
	}
	return string(body)
}

func usageFromJSON(body []byte) *TokenUsage {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil
	}
	usage := mapFromAny(root["usage"])
	if len(usage) == 0 {
		if message := mapFromAny(root["message"]); len(message) > 0 {
			usage = mapFromAny(message["usage"])
		}
	}
	if len(usage) == 0 {
		if delta := mapFromAny(root["delta"]); len(delta) > 0 {
			usage = mapFromAny(delta["usage"])
		}
	}
	if len(usage) == 0 {
		return nil
	}
	input := firstPositiveInt64(usage["input_tokens"], usage["cache_creation_input_tokens"], usage["cache_read_input_tokens"])
	output := numberToInt64(usage["output_tokens"])
	total := numberToInt64(usage["total_tokens"])
	if total == 0 {
		total = input + output
	}
	return &TokenUsage{InputTokens: input, OutputTokens: output, TotalTokens: total}
}

func mergeUsage(target *TokenUsage, patch *TokenUsage) {
	if patch == nil {
		return
	}
	if patch.InputTokens > 0 {
		target.InputTokens = patch.InputTokens
	}
	if patch.OutputTokens > 0 {
		target.OutputTokens = patch.OutputTokens
	}
	if target.TotalTokens == 0 {
		target.TotalTokens = target.InputTokens + target.OutputTokens
	}
}

func firstPositiveInt64(values ...any) int64 {
	var total int64
	for _, value := range values {
		total += numberToInt64(value)
	}
	return total
}

func numberToInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		n, _ := typed.Int64()
		return n
	default:
		return 0
	}
}

func mapFromAny(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

func durationSpec(d time.Duration) *DurationSpec {
	return &DurationSpec{Seconds: int64(d / time.Second), Nanos: int32(d % time.Second)}
}

func secretString(secrets map[string]any, key string) string {
	return stringFromMap(secrets, key)
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func boolFromMap(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	switch typed := values[key].(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func mergeMaps(first, second map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range first {
		out[key] = value
	}
	for key, value := range second {
		out[key] = value
	}
	return out
}

func sanitizeMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func methodName(method string) string {
	return "/" + providerAdapterService + "/" + method
}
