package modules

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const coreBridgeService = "lightbridge.modules.CoreBridge"

type CoreBridge interface {
	GetUserSummary(ctx context.Context, req CoreBridgeUserRequest) (*CoreBridgeUserSummary, error)
	GetAccountCredentials(ctx context.Context, req CoreBridgeAccountRequest) (*CoreBridgeAccountCredentials, error)
	WriteAuditLog(ctx context.Context, req CoreBridgeAuditLog) error
	GetModuleConfig(ctx context.Context, req CoreBridgeModuleConfigRequest) (*CoreBridgeModuleConfig, error)
	UpdateProviderRuntimeStatus(ctx context.Context, req CoreBridgeRuntimeStatusRequest) error
}

type CoreBridgeUserRequest struct {
	ModuleID string `json:"module_id,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	Email    string `json:"email,omitempty"`
}

type CoreBridgeUserSummary struct {
	ModuleID string         `json:"module_id,omitempty"`
	UserID   string         `json:"user_id"`
	Username string         `json:"username,omitempty"`
	Email    string         `json:"email,omitempty"`
	Role     string         `json:"role,omitempty"`
	Groups   []string       `json:"groups,omitempty"`
	Enabled  bool           `json:"enabled"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CoreBridgeAccountRequest struct {
	ModuleID      string `json:"module_id,omitempty"`
	AccountID     string `json:"account_id,omitempty"`
	ProviderID    string `json:"provider_id,omitempty"`
	CredentialRef string `json:"credential_ref,omitempty"`
	Purpose       string `json:"purpose,omitempty"`
}

type CoreBridgeAccountCredentials struct {
	ModuleID      string         `json:"module_id,omitempty"`
	AccountID     string         `json:"account_id,omitempty"`
	ProviderID    string         `json:"provider_id,omitempty"`
	DisplayName   string         `json:"display_name,omitempty"`
	CredentialRef string         `json:"credential_ref,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	Secrets       map[string]any `json:"secrets,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
}

type CoreBridgeAuditLog struct {
	ModuleID     string         `json:"module_id,omitempty"`
	ActorUserID  string         `json:"actor_user_id,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type,omitempty"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Severity     string         `json:"severity,omitempty"`
	Message      string         `json:"message,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	OccurredAt   *time.Time     `json:"occurred_at,omitempty"`
}

type CoreBridgeModuleConfigRequest struct {
	ModuleID string `json:"module_id,omitempty"`
	Key      string `json:"key,omitempty"`
}

type CoreBridgeModuleConfig struct {
	ModuleID string         `json:"module_id,omitempty"`
	Key      string         `json:"key,omitempty"`
	Config   map[string]any `json:"config,omitempty"`
	Value    any            `json:"value,omitempty"`
}

type CoreBridgeRuntimeStatusRequest struct {
	ModuleID        string         `json:"module_id,omitempty"`
	ProviderID      string         `json:"provider_id,omitempty"`
	Status          RuntimeStatus  `json:"status"`
	Message         string         `json:"message,omitempty"`
	LastError       string         `json:"last_error,omitempty"`
	LastHeartbeatAt *time.Time     `json:"last_heartbeat_at,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type GRPCCoreBridgeClient struct {
	conn *grpc.ClientConn
}

func NewGRPCCoreBridgeClient(socketPath string) (*GRPCCoreBridgeClient, error) {
	if socketPath == "" {
		return nil, errors.New("core bridge socket path is required")
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
		return nil, fmt.Errorf("dial core bridge grpc socket: %w", err)
	}
	return &GRPCCoreBridgeClient{conn: conn}, nil
}

func (c *GRPCCoreBridgeClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *GRPCCoreBridgeClient) GetUserSummary(ctx context.Context, req CoreBridgeUserRequest) (*CoreBridgeUserSummary, error) {
	var out CoreBridgeUserSummary
	if err := c.invoke(ctx, "GetUserSummary", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *GRPCCoreBridgeClient) GetAccountCredentials(ctx context.Context, req CoreBridgeAccountRequest) (*CoreBridgeAccountCredentials, error) {
	var out CoreBridgeAccountCredentials
	if err := c.invoke(ctx, "GetAccountCredentials", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *GRPCCoreBridgeClient) WriteAuditLog(ctx context.Context, req CoreBridgeAuditLog) error {
	return c.invoke(ctx, "WriteAuditLog", req, &emptyMessage{})
}

func (c *GRPCCoreBridgeClient) GetModuleConfig(ctx context.Context, req CoreBridgeModuleConfigRequest) (*CoreBridgeModuleConfig, error) {
	var out CoreBridgeModuleConfig
	if err := c.invoke(ctx, "GetModuleConfig", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *GRPCCoreBridgeClient) UpdateProviderRuntimeStatus(ctx context.Context, req CoreBridgeRuntimeStatusRequest) error {
	return c.invoke(ctx, "UpdateProviderRuntimeStatus", req, &emptyMessage{})
}

func (c *GRPCCoreBridgeClient) invoke(ctx context.Context, method string, input any, output any) error {
	return c.conn.Invoke(ctx, coreBridgeMethod(method), input, output, grpc.CallContentSubtype(providerJSONCodecName), grpc.ForceCodec(grpcJSONCodec{}))
}

type grpcCoreBridgeService interface {
	mustEmbedUnimplementedGRPCCoreBridgeService()
}

type coreBridgeGRPCServer struct {
	moduleID string
	bridge   CoreBridge
}

func (s *coreBridgeGRPCServer) mustEmbedUnimplementedGRPCCoreBridgeService() {}

func RegisterCoreBridgeService(server *grpc.Server, moduleID string, bridge CoreBridge) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: coreBridgeService,
		HandlerType: (*grpcCoreBridgeService)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "GetUserSummary", Handler: coreBridgeGetUserSummaryHandler},
			{MethodName: "GetAccountCredentials", Handler: coreBridgeGetAccountCredentialsHandler},
			{MethodName: "WriteAuditLog", Handler: coreBridgeWriteAuditLogHandler},
			{MethodName: "GetModuleConfig", Handler: coreBridgeGetModuleConfigHandler},
			{MethodName: "UpdateProviderRuntimeStatus", Handler: coreBridgeUpdateProviderRuntimeStatusHandler},
		},
	}, &coreBridgeGRPCServer{moduleID: moduleID, bridge: bridge})
}

func StartCoreBridgeServer(moduleID, socketPath string, bridge CoreBridge) (io.Closer, error) {
	if moduleID == "" {
		return nil, errors.New("module id is required")
	}
	if socketPath == "" {
		return nil, errors.New("core bridge socket path is required")
	}
	if bridge == nil {
		return nil, errors.New("core bridge implementation is required")
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, fmt.Errorf("create core bridge socket dir: %w", err)
	}
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen core bridge socket: %w", err)
	}
	server := grpc.NewServer()
	RegisterCoreBridgeService(server, moduleID, bridge)
	runtime := &coreBridgeServerRuntime{server: server, socketPath: socketPath}
	go func() {
		_ = server.Serve(listener)
	}()
	return runtime, nil
}

type coreBridgeServerRuntime struct {
	server     *grpc.Server
	socketPath string
	stopOnce   sync.Once
}

func (r *coreBridgeServerRuntime) Close() error {
	if r == nil {
		return nil
	}
	r.stopOnce.Do(func() {
		if r.server != nil {
			r.server.Stop()
		}
		if r.socketPath != "" {
			_ = os.Remove(r.socketPath)
		}
	})
	return nil
}

func coreBridgeGetUserSummaryHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(CoreBridgeUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req any) (any, error) {
		s := srv.(*coreBridgeGRPCServer)
		if s.bridge == nil {
			return nil, status.Error(codes.FailedPrecondition, "core bridge is not configured")
		}
		request := *req.(*CoreBridgeUserRequest)
		request.ModuleID = s.moduleID
		out, err := s.bridge.GetUserSummary(ctx, request)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = &CoreBridgeUserSummary{}
		}
		out.ModuleID = s.moduleID
		return out, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: coreBridgeMethod("GetUserSummary")}, handler)
}

func coreBridgeGetAccountCredentialsHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(CoreBridgeAccountRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req any) (any, error) {
		s := srv.(*coreBridgeGRPCServer)
		if s.bridge == nil {
			return nil, status.Error(codes.FailedPrecondition, "core bridge is not configured")
		}
		request := *req.(*CoreBridgeAccountRequest)
		request.ModuleID = s.moduleID
		out, err := s.bridge.GetAccountCredentials(ctx, request)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = &CoreBridgeAccountCredentials{}
		}
		out.ModuleID = s.moduleID
		return out, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: coreBridgeMethod("GetAccountCredentials")}, handler)
}

func coreBridgeWriteAuditLogHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(CoreBridgeAuditLog)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req any) (any, error) {
		s := srv.(*coreBridgeGRPCServer)
		if s.bridge == nil {
			return nil, status.Error(codes.FailedPrecondition, "core bridge is not configured")
		}
		request := *req.(*CoreBridgeAuditLog)
		request.ModuleID = s.moduleID
		return emptyMessage{}, s.bridge.WriteAuditLog(ctx, request)
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: coreBridgeMethod("WriteAuditLog")}, handler)
}

func coreBridgeGetModuleConfigHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(CoreBridgeModuleConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req any) (any, error) {
		s := srv.(*coreBridgeGRPCServer)
		if s.bridge == nil {
			return nil, status.Error(codes.FailedPrecondition, "core bridge is not configured")
		}
		request := *req.(*CoreBridgeModuleConfigRequest)
		request.ModuleID = s.moduleID
		out, err := s.bridge.GetModuleConfig(ctx, request)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = &CoreBridgeModuleConfig{}
		}
		out.ModuleID = s.moduleID
		return out, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: coreBridgeMethod("GetModuleConfig")}, handler)
}

func coreBridgeUpdateProviderRuntimeStatusHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(CoreBridgeRuntimeStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req any) (any, error) {
		s := srv.(*coreBridgeGRPCServer)
		if s.bridge == nil {
			return nil, status.Error(codes.FailedPrecondition, "core bridge is not configured")
		}
		request := *req.(*CoreBridgeRuntimeStatusRequest)
		request.ModuleID = s.moduleID
		return emptyMessage{}, s.bridge.UpdateProviderRuntimeStatus(ctx, request)
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: coreBridgeMethod("UpdateProviderRuntimeStatus")}, handler)
}

func coreBridgeMethod(method string) string {
	return "/" + coreBridgeService + "/" + method
}
