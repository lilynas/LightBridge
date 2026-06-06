package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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
	moduleID               = "lightbridge.provider.mock"
)

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

type mockProvider struct{}

func (mockProvider) mustEmbedUnimplementedProviderService() {}

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

type EmbeddingRequest struct {
	Model         string         `json:"model"`
	Input         any            `json:"input,omitempty"`
	CredentialRef string         `json:"credential_ref,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
}

type EmbeddingResponse struct {
	Model      string      `json:"model,omitempty"`
	Embeddings []Embedding `json:"embeddings"`
	Usage      *TokenUsage `json:"usage,omitempty"`
}

type Embedding struct {
	Index  int       `json:"index"`
	Vector []float64 `json:"vector"`
}

type TokenCountRequest struct {
	Model    string         `json:"model"`
	Messages []ChatMessage  `json:"messages,omitempty"`
	Input    any            `json:"input,omitempty"`
	Config   map[string]any `json:"config,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type TokenCountResponse struct {
	Usage TokenUsage `json:"usage"`
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
			{MethodName: "Embed", Handler: embedHandler},
			{MethodName: "CountTokens", Handler: countTokensHandler},
		},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "Forward",
				Handler:       forwardHandler,
				ServerStreams: true,
				ClientStreams: true,
			},
		},
	}, mockProvider{})
}

func metadataHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(emptyMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) {
		return ProviderMetadata{
			ID:              moduleID,
			DisplayName:     "Mock Provider",
			CredentialTypes: []string{"api_key"},
			Supports:        map[string]bool{"chat": true, "stream": true, "embedding": true, "tokens": true},
			Extra: map[string]any{
				"downstream_protocols": []string{"openai-compatible"},
				"endpoints":            []string{"/v1/chat/completions"},
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
			{ID: "mock-chat", DisplayName: "Mock Chat", Capabilities: map[string]bool{"chat": true}},
			{ID: "mock-stream", DisplayName: "Mock Stream", Capabilities: map[string]bool{"chat": true, "stream": true}},
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
	handler := func(_ context.Context, req any) (any, error) {
		account := req.(*ProviderAccount)
		if strings.TrimSpace(secretString(account.Secrets, "mock_api_key")) == "" {
			return AccountValidationResult{Valid: false, Warnings: []string{"mock_api_key is required"}}, nil
		}
		return AccountValidationResult{Valid: true, Metadata: map[string]any{"provider_id": moduleID}}, nil
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
	handler := func(_ context.Context, req any) (any, error) {
		account := *req.(*ProviderAccount)
		if account.Metadata == nil {
			account.Metadata = map[string]any{}
		}
		account.Metadata["refreshed_at"] = time.Now().UTC().Format(time.RFC3339)
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
	handler := func(context.Context, any) (any, error) {
		return TestAccountResult{
			OK:      true,
			Message: "mock provider account is usable",
			Latency: &DurationSpec{
				Nanos: int32(15 * time.Millisecond),
			},
			Metadata: map[string]any{"provider_id": moduleID},
		}, nil
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
		return NormalizedError{
			Retryable:  upstream.StatusCode == 429 || upstream.StatusCode >= 500,
			StatusCode: upstream.StatusCode,
			Code:       "provider_error",
			Message:    upstream.Message,
			ProviderRaw: map[string]any{
				"code": upstream.Code,
			},
		}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("NormalizeError")}, handler)
}

func embedHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(EmbeddingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(_ context.Context, req any) (any, error) {
		request := req.(*EmbeddingRequest)
		return EmbeddingResponse{
			Model: request.Model,
			Embeddings: []Embedding{
				{Index: 0, Vector: []float64{0.1, 0.2, 0.3}},
			},
			Usage: &TokenUsage{InputTokens: 1, TotalTokens: 1},
		}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("Embed")}, handler)
}

func countTokensHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(TokenCountRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) {
		return TokenCountResponse{Usage: TokenUsage{InputTokens: 3, TotalTokens: 3}}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName("CountTokens")}, handler)
}

func forwardHandler(_ any, stream grpc.ServerStream) error {
	var req GatewayRequest
	if err := stream.RecvMsg(&req); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	events := []GatewayEvent{
		{Type: "headers", StatusCode: 200, Headers: map[string][]string{"content-type": {"text/event-stream"}}},
		{Type: "data", Data: json.RawMessage(`{"choices":[{"delta":{"content":"hello"}}]}`)},
		{Type: "data", Data: json.RawMessage(`{"choices":[{"delta":{"content":" from mock"}}]}`)},
		{Type: "usage", Usage: &TokenUsage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7}},
		{Type: "done"},
	}
	for _, event := range events {
		if err := stream.SendMsg(&event); err != nil {
			return err
		}
	}
	return nil
}

func secretString(secrets map[string]any, key string) string {
	if secrets == nil {
		return ""
	}
	value, ok := secrets[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func methodName(method string) string {
	return "/" + providerAdapterService + "/" + method
}
