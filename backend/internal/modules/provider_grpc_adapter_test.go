package modules

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestGRPCProviderAdapterProtocol(t *testing.T) {
	socketDir, err := os.MkdirTemp("/tmp", "lb-grpc-mod-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(socketDir)
	})
	socketPath := filepath.Join(socketDir, "provider.sock")
	server := newGRPCProviderProtocolTestServer(t, socketPath)
	defer server.Stop()

	adapter, err := NewGRPCProviderAdapter("lightbridge.provider.test", socketPath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, adapter.Close())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, adapter.HealthCheck(ctx))

	metadata, err := adapter.Metadata(ctx)
	require.NoError(t, err)
	require.Equal(t, "lightbridge.provider.test", metadata.ID)
	require.True(t, metadata.Supports["chat"])

	models, err := adapter.ListModels(ctx, ListModelsRequest{})
	require.NoError(t, err)
	require.Equal(t, []ModelInfo{{ID: "test-model", Capabilities: map[string]bool{"chat": true}}}, models.Models)

	validation, err := adapter.ValidateAccount(ctx, ProviderAccount{ID: "acct_1"})
	require.NoError(t, err)
	require.True(t, validation.Valid)

	refreshed, err := adapter.RefreshAccount(ctx, ProviderAccount{ID: "acct_1"})
	require.NoError(t, err)
	require.Equal(t, "acct_1", refreshed.ID)
	require.Equal(t, "refreshed", refreshed.Metadata["status"])

	accountTest, err := adapter.TestAccount(ctx, TestAccountRequest{Account: ProviderAccount{ID: "acct_1"}, Mode: "health"})
	require.NoError(t, err)
	require.True(t, accountTest.OK)

	normalized, err := adapter.NormalizeError(ctx, UpstreamError{StatusCode: 429, Message: "rate limited"})
	require.NoError(t, err)
	require.True(t, normalized.Retryable)
	require.Equal(t, 429, normalized.StatusCode)

	events, err := adapter.Forward(ctx, GatewayRequest{
		Endpoint: "/v1/chat/completions",
		Method:   http.MethodPost,
		Stream:   true,
	})
	require.NoError(t, err)
	require.Equal(t, GatewayEvent{Type: "headers", StatusCode: 200, Headers: map[string][]string{"content-type": {"text/event-stream"}}}, <-events)
	require.Equal(t, GatewayEvent{Type: "data", Data: json.RawMessage(`{"delta":"hello"}`)}, <-events)
	require.Equal(t, GatewayEvent{Type: "done"}, <-events)
	_, ok := <-events
	require.False(t, ok)
}

type grpcProviderProtocolTestService interface {
	mustEmbedUnimplementedGRPCProviderProtocolTestService()
}

type grpcProviderProtocolTestServer struct{}

func (s *grpcProviderProtocolTestServer) mustEmbedUnimplementedGRPCProviderProtocolTestService() {}

func newGRPCProviderProtocolTestServer(t *testing.T, socketPath string) *grpc.Server {
	t.Helper()
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	server := grpc.NewServer()
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: providerAdapterService,
		HandlerType: (*grpcProviderProtocolTestService)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Metadata", Handler: grpcMetadataHandler},
			{MethodName: "HealthCheck", Handler: grpcHealthCheckHandler},
			{MethodName: "ListModels", Handler: grpcListModelsHandler},
			{MethodName: "ValidateAccount", Handler: grpcValidateAccountHandler},
			{MethodName: "RefreshAccount", Handler: grpcRefreshAccountHandler},
			{MethodName: "TestAccount", Handler: grpcTestAccountHandler},
			{MethodName: "NormalizeError", Handler: grpcNormalizeErrorHandler},
		},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "Forward",
				Handler:       grpcForwardHandler,
				ServerStreams: true,
				ClientStreams: true,
			},
		},
	}, &grpcProviderProtocolTestServer{})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)
	return server
}

func grpcMetadataHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(emptyMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) {
		return ProviderMetadata{ID: "lightbridge.provider.test", DisplayName: "Test Provider", Supports: map[string]bool{"chat": true}}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: providerAdapterMethod("Metadata")}, handler)
}

func grpcHealthCheckHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(emptyMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) {
		return emptyMessage{}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: providerAdapterMethod("HealthCheck")}, handler)
}

func grpcListModelsHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(ListModelsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) {
		return ListModelsResponse{Models: []ModelInfo{{ID: "test-model", Capabilities: map[string]bool{"chat": true}}}}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: providerAdapterMethod("ListModels")}, handler)
}

func grpcValidateAccountHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(ProviderAccount)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) {
		return AccountValidationResult{Valid: true}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: providerAdapterMethod("ValidateAccount")}, handler)
}

func grpcRefreshAccountHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(ProviderAccount)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(_ context.Context, req any) (any, error) {
		account := *req.(*ProviderAccount)
		account.Metadata = map[string]any{"status": "refreshed"}
		return account, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: providerAdapterMethod("RefreshAccount")}, handler)
}

func grpcTestAccountHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(TestAccountRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(context.Context, any) (any, error) {
		return TestAccountResult{OK: true}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: providerAdapterMethod("TestAccount")}, handler)
}

func grpcNormalizeErrorHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(UpstreamError)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(_ context.Context, req any) (any, error) {
		upstream := req.(*UpstreamError)
		return NormalizedError{Retryable: true, StatusCode: upstream.StatusCode, Message: upstream.Message}, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: providerAdapterMethod("NormalizeError")}, handler)
}

func grpcForwardHandler(_ any, stream grpc.ServerStream) error {
	var req GatewayRequest
	if err := stream.RecvMsg(&req); err != nil {
		return err
	}
	if err := stream.SendMsg(&GatewayEvent{Type: "headers", StatusCode: 200, Headers: map[string][]string{"content-type": {"text/event-stream"}}}); err != nil {
		return err
	}
	if err := stream.SendMsg(&GatewayEvent{Type: "data", Data: json.RawMessage(`{"delta":"hello"}`)}); err != nil {
		return err
	}
	return stream.SendMsg(&GatewayEvent{Type: "done"})
}
