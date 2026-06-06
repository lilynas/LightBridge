package modules

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

var ErrProviderNotRegistered = errors.New("provider adapter not registered")

type ProviderAdapter interface {
	ID() string
	Metadata(ctx context.Context) (*ProviderMetadata, error)
	HealthCheck(ctx context.Context) error
	ListModels(ctx context.Context, req ListModelsRequest) (*ListModelsResponse, error)
	ValidateAccount(ctx context.Context, account ProviderAccount) (*AccountValidationResult, error)
	RefreshAccount(ctx context.Context, account ProviderAccount) (*ProviderAccount, error)
	Forward(ctx context.Context, req GatewayRequest) (<-chan GatewayEvent, error)
	TestAccount(ctx context.Context, req TestAccountRequest) (*TestAccountResult, error)
	NormalizeError(ctx context.Context, upstreamError UpstreamError) (*NormalizedError, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error)
	Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
	CountTokens(ctx context.Context, req TokenCountRequest) (*TokenCountResponse, error)
}

type ProviderMetadata struct {
	ID              string          `json:"id"`
	DisplayName     string          `json:"display_name"`
	Supports        map[string]bool `json:"supports"`
	CredentialTypes []string        `json:"credential_types,omitempty"`
	Extra           map[string]any  `json:"extra,omitempty"`
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
	UserContext        GatewayUserContext  `json:"user_context,omitempty"`
	Account            ProviderAccount     `json:"account,omitempty"`
	GroupContext       map[string]any      `json:"group_context,omitempty"`
	ProxyContext       map[string]any      `json:"proxy_context,omitempty"`
	Metadata           map[string]any      `json:"metadata,omitempty"`
}

type GatewayUserContext struct {
	UserID string `json:"user_id,omitempty"`
	Email  string `json:"email,omitempty"`
	Role   string `json:"role,omitempty"`
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

type ModelInfo struct {
	ID           string          `json:"id"`
	DisplayName  string          `json:"display_name,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	Metadata     map[string]any  `json:"metadata,omitempty"`
}

type ChatRequest struct {
	Model         string         `json:"model"`
	Messages      []ChatMessage  `json:"messages"`
	Stream        bool           `json:"stream"`
	CredentialRef string         `json:"credential_ref,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	Options       map[string]any `json:"options,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type ChatEvent struct {
	Type         string         `json:"type"`
	Delta        any            `json:"delta,omitempty"`
	Message      any            `json:"message,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
	Usage        *TokenUsage    `json:"usage,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Error        string         `json:"error,omitempty"`
}

type EmbeddingRequest struct {
	Model         string         `json:"model"`
	Input         any            `json:"input"`
	CredentialRef string         `json:"credential_ref,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
}

type EmbeddingResponse struct {
	Model      string         `json:"model,omitempty"`
	Embeddings []Embedding    `json:"embeddings"`
	Usage      *TokenUsage    `json:"usage,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
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

type TokenCountResponse struct {
	Usage TokenUsage `json:"usage"`
}

type TokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens,omitempty"`
	TotalTokens  int64 `json:"total_tokens"`
}

type ProviderRegistry struct {
	mu       sync.RWMutex
	adapters map[string]ProviderAdapter
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{adapters: make(map[string]ProviderAdapter)}
}

func (r *ProviderRegistry) Register(adapter ProviderAdapter) {
	if adapter == nil || adapter.ID() == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.ID()] = adapter
}

func (r *ProviderRegistry) Unregister(providerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.adapters, providerID)
}

func (r *ProviderRegistry) Resolve(providerID string) (ProviderAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[providerID]
	if !ok {
		return nil, ErrProviderNotRegistered
	}
	return adapter, nil
}

func (r *ProviderRegistry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.adapters))
	for id := range r.adapters {
		ids = append(ids, id)
	}
	return ids
}
