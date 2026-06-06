package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

const (
	providerAdapterService = "lightbridge.modules.ProviderAdapter"
	providerJSONCodecName  = "json"
)

func init() {
	encoding.RegisterCodec(grpcJSONCodec{})
}

type grpcJSONCodec struct{}

func (grpcJSONCodec) Name() string {
	return providerJSONCodecName
}

func (grpcJSONCodec) Marshal(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(v)
}

func (grpcJSONCodec) Unmarshal(data []byte, v any) error {
	if len(data) == 0 || v == nil {
		return nil
	}
	return json.Unmarshal(data, v)
}

type emptyMessage struct{}

type GRPCProviderAdapter struct {
	moduleID string
	conn     *grpc.ClientConn
}

func NewGRPCProviderAdapter(moduleID, socketPath string) (*GRPCProviderAdapter, error) {
	if moduleID == "" {
		return nil, errors.New("module id is required")
	}
	if socketPath == "" {
		return nil, errors.New("provider socket path is required")
	}
	conn, err := grpc.DialContext(
		context.Background(),
		"unix:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		}),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(providerJSONCodecName), grpc.ForceCodec(grpcJSONCodec{})),
	)
	if err != nil {
		return nil, fmt.Errorf("dial provider grpc sidecar %s: %w", moduleID, err)
	}
	return &GRPCProviderAdapter{moduleID: moduleID, conn: conn}, nil
}

func (a *GRPCProviderAdapter) Close() error {
	if a == nil || a.conn == nil {
		return nil
	}
	return a.conn.Close()
}

func (a *GRPCProviderAdapter) ID() string {
	return a.moduleID
}

func (a *GRPCProviderAdapter) Metadata(ctx context.Context) (*ProviderMetadata, error) {
	var out ProviderMetadata
	if err := a.invoke(ctx, "Metadata", emptyMessage{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *GRPCProviderAdapter) HealthCheck(ctx context.Context) error {
	return a.invoke(ctx, "HealthCheck", emptyMessage{}, &emptyMessage{})
}

func (a *GRPCProviderAdapter) ListModels(ctx context.Context, req ListModelsRequest) (*ListModelsResponse, error) {
	var out ListModelsResponse
	if err := a.invoke(ctx, "ListModels", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *GRPCProviderAdapter) ValidateAccount(ctx context.Context, account ProviderAccount) (*AccountValidationResult, error) {
	var out AccountValidationResult
	if err := a.invoke(ctx, "ValidateAccount", account, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *GRPCProviderAdapter) RefreshAccount(ctx context.Context, account ProviderAccount) (*ProviderAccount, error) {
	var out ProviderAccount
	if err := a.invoke(ctx, "RefreshAccount", account, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *GRPCProviderAdapter) Forward(ctx context.Context, req GatewayRequest) (<-chan GatewayEvent, error) {
	desc := &grpc.StreamDesc{
		StreamName:    "Forward",
		ServerStreams: true,
		ClientStreams: true,
	}
	stream, err := a.conn.NewStream(ctx, desc, providerAdapterMethod("Forward"), grpc.CallContentSubtype(providerJSONCodecName), grpc.ForceCodec(grpcJSONCodec{}))
	if err != nil {
		return nil, err
	}
	if err := stream.SendMsg(req); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}

	events := make(chan GatewayEvent)
	go func() {
		defer close(events)
		for {
			var event GatewayEvent
			if err := stream.RecvMsg(&event); err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				events <- GatewayEvent{Type: "error", Error: normalizedGRPCStreamError(err)}
				return
			}
			events <- event
		}
	}()
	return events, nil
}

func (a *GRPCProviderAdapter) TestAccount(ctx context.Context, req TestAccountRequest) (*TestAccountResult, error) {
	var out TestAccountResult
	if err := a.invoke(ctx, "TestAccount", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *GRPCProviderAdapter) NormalizeError(ctx context.Context, upstreamError UpstreamError) (*NormalizedError, error) {
	var out NormalizedError
	if err := a.invoke(ctx, "NormalizeError", upstreamError, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *GRPCProviderAdapter) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	forwardEvents, err := a.Forward(ctx, GatewayRequest{
		Endpoint: "/v1/chat/completions",
		Method:   "POST",
		Stream:   req.Stream,
		Body:     mustJSONRawMessage(req),
		Metadata: map[string]any{"compat_method": "ChatStream"},
	})
	if err != nil {
		return nil, err
	}
	events := make(chan ChatEvent)
	go func() {
		defer close(events)
		for event := range forwardEvents {
			events <- gatewayEventToChatEvent(event)
		}
	}()
	return events, nil
}

func (a *GRPCProviderAdapter) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	var out EmbeddingResponse
	if err := a.invoke(ctx, "Embed", req, &out); err != nil {
		if status.Code(err) == codes.Unimplemented {
			return nil, fmt.Errorf("provider %s does not implement Embed", a.moduleID)
		}
		return nil, err
	}
	return &out, nil
}

func (a *GRPCProviderAdapter) CountTokens(ctx context.Context, req TokenCountRequest) (*TokenCountResponse, error) {
	var out TokenCountResponse
	if err := a.invoke(ctx, "CountTokens", req, &out); err != nil {
		if status.Code(err) == codes.Unimplemented {
			return nil, fmt.Errorf("provider %s does not implement CountTokens", a.moduleID)
		}
		return nil, err
	}
	return &out, nil
}

func (a *GRPCProviderAdapter) invoke(ctx context.Context, method string, input any, output any) error {
	return a.conn.Invoke(ctx, providerAdapterMethod(method), input, output, grpc.CallContentSubtype(providerJSONCodecName), grpc.ForceCodec(grpcJSONCodec{}))
}

func providerAdapterMethod(method string) string {
	return "/" + providerAdapterService + "/" + method
}

func normalizedGRPCStreamError(err error) *NormalizedError {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return &NormalizedError{Message: err.Error()}
	}
	return &NormalizedError{
		Retryable: st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded,
		Code:      st.Code().String(),
		Message:   st.Message(),
		ProviderRaw: map[string]any{
			"grpc_code": st.Code().String(),
		},
	}
}

func mustJSONRawMessage(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func gatewayEventToChatEvent(event GatewayEvent) ChatEvent {
	switch event.Type {
	case "data":
		return ChatEvent{Type: "delta", Delta: json.RawMessage(event.Data), Usage: event.Usage, Metadata: event.Metadata}
	case "usage":
		return ChatEvent{Type: "usage", Usage: event.Usage, Metadata: event.Metadata}
	case "done":
		return ChatEvent{Type: "done", Metadata: event.Metadata}
	case "error":
		message := ""
		if event.Error != nil {
			message = event.Error.Message
		}
		return ChatEvent{Type: "error", Error: message, Metadata: event.Metadata}
	default:
		return ChatEvent{Type: event.Type, Metadata: event.Metadata}
	}
}
