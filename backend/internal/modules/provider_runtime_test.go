package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProviderRegistryRegisterResolveUnregister(t *testing.T) {
	registry := NewProviderRegistry()
	adapter := &fakeProviderAdapter{id: "lightbridge.provider.test"}

	registry.Register(adapter)
	got, err := registry.Resolve(adapter.id)
	require.NoError(t, err)
	require.Equal(t, adapter, got)
	require.Contains(t, registry.IDs(), adapter.id)

	registry.Unregister(adapter.id)
	_, err = registry.Resolve(adapter.id)
	require.ErrorIs(t, err, ErrProviderNotRegistered)
}

func TestHTTPProviderAdapterProtocol(t *testing.T) {
	socketDir, err := os.MkdirTemp("/tmp", "lb-mod-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(socketDir)
	})
	socketPath := filepath.Join(socketDir, "provider.sock")
	server := newProviderProtocolTestServer(t, socketPath)
	defer func() {
		_ = server.Close()
	}()

	adapter := NewHTTPProviderAdapter("lightbridge.provider.test", socketPath)
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

	embeddings, err := adapter.Embed(ctx, EmbeddingRequest{Model: "test-model", Input: "hello"})
	require.NoError(t, err)
	require.Equal(t, []Embedding{{Index: 0, Vector: []float64{0.1, 0.2}}}, embeddings.Embeddings)

	tokens, err := adapter.CountTokens(ctx, TokenCountRequest{Model: "test-model", Input: "hello"})
	require.NoError(t, err)
	require.Equal(t, int64(3), tokens.Usage.TotalTokens)

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

	gatewayEvents, err := adapter.Forward(ctx, GatewayRequest{
		Endpoint: "/v1/chat/completions",
		Method:   http.MethodPost,
		Stream:   true,
	})
	require.NoError(t, err)
	require.Equal(t, GatewayEvent{Type: "headers", StatusCode: 200, Headers: map[string][]string{"content-type": {"text/event-stream"}}}, <-gatewayEvents)
	require.Equal(t, GatewayEvent{Type: "data", Data: json.RawMessage(`{"delta":"hello"}`)}, <-gatewayEvents)
	require.Equal(t, GatewayEvent{Type: "done"}, <-gatewayEvents)
	_, ok := <-gatewayEvents
	require.False(t, ok)

	events, err := adapter.ChatStream(ctx, ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
		Stream:   true,
	})
	require.NoError(t, err)
	require.Equal(t, ChatEvent{Type: "delta", Delta: "hello"}, <-events)
	require.Equal(t, ChatEvent{Type: "done", FinishReason: "stop"}, <-events)
	_, chatOK := <-events
	require.False(t, chatOK)
}

func TestHTTPProviderAdapterForwardAcceptsLargeGatewayEvent(t *testing.T) {
	socketDir, err := os.MkdirTemp("/tmp", "lb-mod-large-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(socketDir)
	})
	socketPath := filepath.Join(socketDir, "provider.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	mux := http.NewServeMux()
	largePayload := strings.Repeat("x", 128*1024)
	mux.HandleFunc("/provider/forward", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		require.NoError(t, json.NewEncoder(w).Encode(GatewayEvent{
			Type: "data",
			Data: json.RawMessage(`"` + largePayload + `"`),
		}))
	})
	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
	})

	adapter := NewHTTPProviderAdapter("lightbridge.provider.test", socketPath)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events, err := adapter.Forward(ctx, GatewayRequest{Endpoint: "/v1/messages"})
	require.NoError(t, err)
	event := <-events
	require.Equal(t, "data", event.Type)
	require.Len(t, event.Data, len(largePayload)+2)
	_, ok := <-events
	require.False(t, ok)
}

func TestSidecarProviderRuntimeRecordsRuntimeUpdates(t *testing.T) {
	store := &fakeRuntimeStore{}
	runtime := NewSidecarProviderRuntimeWithStore(t.TempDir(), NewProviderRegistry(), store)
	now := time.Date(2026, 5, 31, 8, 0, 0, 0, time.UTC)
	pid := 123

	runtime.recordRuntime(context.Background(), RuntimeInstanceUpdate{
		ModuleID:        "lightbridge.provider.test",
		Status:          RuntimeStatusRunning,
		PID:             &pid,
		SocketPath:      "/tmp/test.sock",
		StartedAt:       &now,
		LastHeartbeatAt: &now,
	})

	require.Len(t, store.updates, 1)
	require.Equal(t, RuntimeStatusRunning, store.updates[0].Status)
	require.Equal(t, &pid, store.updates[0].PID)
}

func TestValidateProviderMetadataIdentityRequiresModuleIDMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, validateProviderMetadataIdentity(ctx, "lightbridge.provider.test", &fakeProviderAdapter{id: "lightbridge.provider.test"}, nil))
	err := validateProviderMetadataIdentity(ctx, "lightbridge.provider.test", &fakeProviderAdapter{id: "lightbridge.provider.other"}, nil)
	require.ErrorContains(t, err, "must match module id")
}

func TestSidecarProviderRuntimeCreatesModuleLogFiles(t *testing.T) {
	baseDir := t.TempDir()
	runtime := NewSidecarProviderRuntimeWithStore(baseDir, NewProviderRegistry(), nil)

	stdout, stderr, err := runtime.openLogFiles("lightbridge.provider/test")
	require.NoError(t, err)
	_, _ = stdout.WriteString("hello stdout\n")
	_, _ = stderr.WriteString("hello stderr\n")
	closeSidecarLogs(stdout, stderr)

	stdoutPath := filepath.Join(baseDir, "modules-runtime", "logs", "lightbridge.provider_test.stdout.log")
	stderrPath := filepath.Join(baseDir, "modules-runtime", "logs", "lightbridge.provider_test.stderr.log")
	stdoutBytes, err := os.ReadFile(stdoutPath)
	require.NoError(t, err)
	stderrBytes, err := os.ReadFile(stderrPath)
	require.NoError(t, err)
	require.Contains(t, string(stdoutBytes), "hello stdout")
	require.Contains(t, string(stderrBytes), "hello stderr")
}

func TestSidecarProviderRuntimeStartsCoreBridgeForSidecar(t *testing.T) {
	if os.Getenv("LIGHTBRIDGE_TEST_SIDECAR") == "1" {
		runSidecarProviderRuntimeHelper(t)
		return
	}

	baseDir := t.TempDir()
	installDir := t.TempDir()
	envFile := filepath.Join(t.TempDir(), "sidecar-env.json")
	t.Setenv("LIGHTBRIDGE_TEST_ENV_FILE", envFile)

	wrapperPath := filepath.Join(installDir, "sidecar.sh")
	wrapper := "#!/bin/sh\nLIGHTBRIDGE_TEST_SIDECAR=1 exec " + shellQuote(os.Args[0]) + " -test.run '^TestSidecarProviderRuntimeStartsCoreBridgeForSidecar$'\n"
	require.NoError(t, os.WriteFile(wrapperPath, []byte(wrapper), 0o755))

	registry := NewProviderRegistry()
	runtime := NewSidecarProviderRuntimeWithStoreAndBridge(baseDir, registry, &fakeRuntimeStore{}, &fakeCoreBridge{})
	module := InstalledModule{
		ID:          "lightbridge.provider.runtime-test",
		Name:        "Runtime Test Provider",
		Type:        ModuleTypeProvider,
		Version:     "0.1.0",
		InstallPath: installDir,
		Manifest: Manifest{
			ID:           "lightbridge.provider.runtime-test",
			Name:         "Runtime Test Provider",
			Type:         ModuleTypeProvider,
			Version:      "0.1.0",
			Capabilities: []Capability{CapabilityProviderAdapter},
			Backend: &BackendSpec{
				Kind:     BackendKindSidecar,
				Command:  "sidecar.sh",
				Protocol: BackendProtocolConnect,
				Healthcheck: &HealthcheckSpec{
					Timeout: DurationSpec{Duration: 2 * time.Second},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, runtime.StartProvider(ctx, module))
	t.Cleanup(func() {
		_ = runtime.StopProvider(context.Background(), module.ID)
	})

	_, err := registry.Resolve(module.ID)
	require.NoError(t, err)

	var env sidecarRuntimeHelperEnv
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(envFile)
		if err != nil {
			return false
		}
		return json.Unmarshal(data, &env) == nil && env.CoreBridgeSocket != ""
	}, 2*time.Second, 25*time.Millisecond)

	require.Equal(t, module.ID, env.ModuleID)
	require.Equal(t, module.Version, env.ModuleVersion)
	require.Equal(t, RuntimeSocketPath(baseDir, module.ID), env.ModuleSocket)
	require.Equal(t, CoreBridgeSocketPath(baseDir, module.ID), env.CoreBridgeSocket)
	require.FileExists(t, env.ModuleSocket)
	require.FileExists(t, env.CoreBridgeSocket)

	bridgeClient, err := NewGRPCCoreBridgeClient(env.CoreBridgeSocket)
	require.NoError(t, err)
	config, err := bridgeClient.GetModuleConfig(ctx, CoreBridgeModuleConfigRequest{ModuleID: "spoofed", Key: "runtime-test"})
	require.NoError(t, err)
	require.Equal(t, module.ID, config.ModuleID)
	require.NoError(t, bridgeClient.Close())

	require.NoError(t, runtime.StopProvider(context.Background(), module.ID))
	requireSocketRemoved(t, env.ModuleSocket)
	requireSocketRemoved(t, env.CoreBridgeSocket)
}

func TestGRPCProviderAdapterTalksToMockProviderExampleSidecar(t *testing.T) {
	if testing.Short() {
		t.Skip("mock provider example sidecar build is skipped in short mode")
	}

	exampleBackendDir := filepath.Clean("../../../examples/modules/lightbridge-provider-mock/backend")
	require.DirExists(t, exampleBackendDir)
	binaryPath := filepath.Join(t.TempDir(), "lightbridge-provider-mock")
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer buildCancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binaryPath, ".")
	build.Dir = exampleBackendDir
	buildOutput, err := build.CombinedOutput()
	require.NoError(t, err, string(buildOutput))

	socketDir, err := os.MkdirTemp("/tmp", "lb-mock-provider-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(socketDir)
	})
	socketPath := filepath.Join(socketDir, "provider.sock")
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	cmd := exec.CommandContext(runCtx, binaryPath)
	cmd.Env = append(os.Environ(),
		"LIGHTBRIDGE_MODULE_SOCKET="+socketPath,
		"LIGHTBRIDGE_MODULE_ID=lightbridge.provider.mock",
		"LIGHTBRIDGE_MODULE_VERSION=0.1.0",
	)
	var sidecarStdout bytes.Buffer
	var sidecarStderr bytes.Buffer
	cmd.Stdout = &sidecarStdout
	cmd.Stderr = &sidecarStderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		runCancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		_ = os.Remove(socketPath)
	})

	require.Eventuallyf(t, func() bool {
		info, err := os.Stat(socketPath)
		return err == nil && !info.IsDir()
	}, 5*time.Second, 50*time.Millisecond, "sidecar stdout=%s stderr=%s", sidecarStdout.String(), sidecarStderr.String())

	adapter, err := NewGRPCProviderAdapter("lightbridge.provider.mock", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, adapter.Close())
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.Eventuallyf(t, func() bool {
		attemptCtx, attemptCancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer attemptCancel()
		return adapter.HealthCheck(attemptCtx) == nil
	}, 5*time.Second, 50*time.Millisecond, "sidecar stdout=%s stderr=%s", sidecarStdout.String(), sidecarStderr.String())

	metadata, err := adapter.Metadata(ctx)
	require.NoError(t, err)
	require.Equal(t, "lightbridge.provider.mock", metadata.ID)
	require.True(t, metadata.Supports["chat"])
	require.True(t, metadata.Supports["stream"])

	models, err := adapter.ListModels(ctx, ListModelsRequest{})
	require.NoError(t, err)
	require.Len(t, models.Models, 2)
	require.Equal(t, "mock-chat", models.Models[0].ID)
	require.Equal(t, "mock-stream", models.Models[1].ID)

	accountTest, err := adapter.TestAccount(ctx, TestAccountRequest{
		Account: ProviderAccount{
			ProviderID: "lightbridge.provider.mock",
			Secrets: map[string]any{
				"mock_api_key": "mock-local-key",
			},
		},
		Mode: "health",
	})
	require.NoError(t, err)
	require.True(t, accountTest.OK)

	events, err := adapter.Forward(ctx, GatewayRequest{
		DownstreamProtocol: "chat_completions",
		Endpoint:           "/v1/chat/completions",
		Method:             http.MethodPost,
		Stream:             true,
		Body:               json.RawMessage(`{"model":"mock-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`),
		Account: ProviderAccount{
			ProviderID: "lightbridge.provider.mock",
			Secrets: map[string]any{
				"mock_api_key": "mock-local-key",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, GatewayEvent{Type: "headers", StatusCode: 200, Headers: map[string][]string{"content-type": {"text/event-stream"}}}, <-events)
	require.Equal(t, GatewayEvent{Type: "data", Data: json.RawMessage(`{"choices":[{"delta":{"content":"hello"}}]}`)}, <-events)
	require.Equal(t, GatewayEvent{Type: "data", Data: json.RawMessage(`{"choices":[{"delta":{"content":" from mock"}}]}`)}, <-events)
	require.Equal(t, GatewayEvent{Type: "usage", Usage: &TokenUsage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7}}, <-events)
	require.Equal(t, GatewayEvent{Type: "done"}, <-events)
	_, ok := <-events
	require.False(t, ok)
}

func TestSidecarProviderRuntimeStartsMockProviderExampleSidecar(t *testing.T) {
	if testing.Short() {
		t.Skip("mock provider example sidecar build is skipped in short mode")
	}

	baseDir, err := os.MkdirTemp("/tmp", "lb-mock-runtime-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(baseDir)
	})

	exampleBackendDir := filepath.Clean("../../../examples/modules/lightbridge-provider-mock/backend")
	require.DirExists(t, exampleBackendDir)
	installDir := t.TempDir()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	backendRel := filepath.Join("backend", platform, "lightbridge-provider-mock")
	binaryPath := filepath.Join(installDir, backendRel)
	require.NoError(t, os.MkdirAll(filepath.Dir(binaryPath), 0o755))
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer buildCancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binaryPath, ".")
	build.Dir = exampleBackendDir
	buildOutput, err := build.CombinedOutput()
	require.NoError(t, err, string(buildOutput))
	require.NoError(t, os.Chmod(binaryPath, 0o755))

	registry := NewProviderRegistry()
	runtimeStore := &fakeRuntimeStore{}
	providerRuntime := NewSidecarProviderRuntimeWithStore(baseDir, registry, runtimeStore)
	module := InstalledModule{
		ID:          "lightbridge.provider.mock",
		Name:        "Mock Provider",
		Type:        ModuleTypeProvider,
		Version:     "0.1.0",
		InstallPath: installDir,
		Manifest: Manifest{
			ID:           "lightbridge.provider.mock",
			Name:         "Mock Provider",
			Type:         ModuleTypeProvider,
			Version:      "0.1.0",
			Capabilities: []Capability{CapabilityProviderAdapter},
			Backend: &BackendSpec{
				Kind:     BackendKindSidecar,
				Command:  "./" + filepath.ToSlash(backendRel),
				Protocol: BackendProtocolGRPC,
				Healthcheck: &HealthcheckSpec{
					Timeout: DurationSpec{Duration: 2 * time.Second},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, providerRuntime.StartProvider(ctx, module))
	t.Cleanup(func() {
		_ = providerRuntime.StopProvider(context.Background(), module.ID)
	})

	adapter, err := registry.Resolve(module.ID)
	require.NoError(t, err)
	metadata, err := adapter.Metadata(ctx)
	require.NoError(t, err)
	require.Equal(t, module.ID, metadata.ID)
	require.FileExists(t, RuntimeSocketPath(baseDir, module.ID))
	require.NotEmpty(t, runtimeStore.updates)
	require.Equal(t, RuntimeStatusRunning, runtimeStore.updates[len(runtimeStore.updates)-1].Status)

	events, err := adapter.Forward(ctx, GatewayRequest{
		DownstreamProtocol: "chat_completions",
		Endpoint:           "/v1/chat/completions",
		Method:             http.MethodPost,
		Stream:             true,
		Body:               json.RawMessage(`{"model":"mock-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`),
		Account: ProviderAccount{
			ProviderID: module.ID,
			Secrets: map[string]any{
				"mock_api_key": "mock-local-key",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "headers", (<-events).Type)
	require.Equal(t, GatewayEvent{Type: "data", Data: json.RawMessage(`{"choices":[{"delta":{"content":"hello"}}]}`)}, <-events)
	require.Equal(t, GatewayEvent{Type: "data", Data: json.RawMessage(`{"choices":[{"delta":{"content":" from mock"}}]}`)}, <-events)
	require.Equal(t, GatewayEvent{Type: "usage", Usage: &TokenUsage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7}}, <-events)
	require.Equal(t, GatewayEvent{Type: "done"}, <-events)
	_, ok := <-events
	require.False(t, ok)

	require.NoError(t, providerRuntime.StopProvider(context.Background(), module.ID))
	_, err = registry.Resolve(module.ID)
	require.ErrorIs(t, err, ErrProviderNotRegistered)
	requireSocketRemoved(t, RuntimeSocketPath(baseDir, module.ID))
}

func newProviderProtocolTestServer(t *testing.T, socketPath string) *http.Server {
	t.Helper()
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("/provider/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/provider/metadata", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, ProviderMetadata{
			ID:          "lightbridge.provider.test",
			DisplayName: "Test Provider",
			Supports:    map[string]bool{"chat": true},
		})
	})
	mux.HandleFunc("/provider/models", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, ListModelsResponse{
			Models: []ModelInfo{{ID: "test-model", Capabilities: map[string]bool{"chat": true}}},
		})
	})
	mux.HandleFunc("/provider/embed", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, EmbeddingResponse{
			Embeddings: []Embedding{{Index: 0, Vector: []float64{0.1, 0.2}}},
		})
	})
	mux.HandleFunc("/provider/count-tokens", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, TokenCountResponse{Usage: TokenUsage{InputTokens: 3, TotalTokens: 3}})
	})
	mux.HandleFunc("/provider/validate-account", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, AccountValidationResult{Valid: true})
	})
	mux.HandleFunc("/provider/refresh-account", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, ProviderAccount{ID: "acct_1", Metadata: map[string]any{"status": "refreshed"}})
	})
	mux.HandleFunc("/provider/test-account", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, TestAccountResult{OK: true})
	})
	mux.HandleFunc("/provider/normalize-error", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, NormalizedError{Retryable: true, StatusCode: 429, Message: "rate limited"})
	})
	mux.HandleFunc("/provider/forward", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"type":"headers","status_code":200,"headers":{"content-type":["text/event-stream"]}}` + "\n"))
		_, _ = w.Write([]byte(`{"type":"data","data":{"delta":"hello"}}` + "\n"))
		_, _ = w.Write([]byte(`{"type":"done"}` + "\n"))
	})
	mux.HandleFunc("/provider/chat-stream", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"type":"delta","delta":"hello"}` + "\n"))
		_, _ = w.Write([]byte(`{"type":"done","finish_reason":"stop"}` + "\n"))
	})

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
	})
	return server
}

type sidecarRuntimeHelperEnv struct {
	ModuleID         string `json:"module_id"`
	ModuleVersion    string `json:"module_version"`
	ModuleSocket     string `json:"module_socket"`
	CoreBridgeSocket string `json:"core_bridge_socket"`
}

func runSidecarProviderRuntimeHelper(t *testing.T) {
	t.Helper()
	socketPath := os.Getenv("LIGHTBRIDGE_MODULE_SOCKET")
	require.NotEmpty(t, socketPath)

	env := sidecarRuntimeHelperEnv{
		ModuleID:         os.Getenv("LIGHTBRIDGE_MODULE_ID"),
		ModuleVersion:    os.Getenv("LIGHTBRIDGE_MODULE_VERSION"),
		ModuleSocket:     socketPath,
		CoreBridgeSocket: os.Getenv("LIGHTBRIDGE_CORE_BRIDGE_SOCKET"),
	}
	envFile := os.Getenv("LIGHTBRIDGE_TEST_ENV_FILE")
	require.NotEmpty(t, envFile)
	envJSON, err := json.Marshal(env)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(envFile, envJSON, 0o644))

	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("/provider/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/provider/metadata", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, ProviderMetadata{
			ID:          env.ModuleID,
			DisplayName: "Runtime Test Provider",
			Supports:    map[string]bool{"chat": true},
		})
	})
	server := &http.Server{Handler: mux}
	_ = server.Serve(listener)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

type fakeRuntimeStore struct {
	updates []RuntimeInstanceUpdate
}

func (s *fakeRuntimeStore) UpdateRuntimeInstance(_ context.Context, update RuntimeInstanceUpdate) error {
	s.updates = append(s.updates, update)
	return nil
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(value))
}

type fakeProviderAdapter struct {
	id string
}

func (a *fakeProviderAdapter) ID() string {
	return a.id
}

func (a *fakeProviderAdapter) Metadata(context.Context) (*ProviderMetadata, error) {
	return &ProviderMetadata{ID: a.id}, nil
}

func (a *fakeProviderAdapter) HealthCheck(context.Context) error {
	return nil
}

func (a *fakeProviderAdapter) ListModels(context.Context, ListModelsRequest) (*ListModelsResponse, error) {
	return &ListModelsResponse{}, nil
}

func (a *fakeProviderAdapter) ValidateAccount(context.Context, ProviderAccount) (*AccountValidationResult, error) {
	return &AccountValidationResult{Valid: true}, nil
}

func (a *fakeProviderAdapter) RefreshAccount(_ context.Context, account ProviderAccount) (*ProviderAccount, error) {
	return &account, nil
}

func (a *fakeProviderAdapter) Forward(context.Context, GatewayRequest) (<-chan GatewayEvent, error) {
	events := make(chan GatewayEvent)
	close(events)
	return events, nil
}

func (a *fakeProviderAdapter) TestAccount(context.Context, TestAccountRequest) (*TestAccountResult, error) {
	return &TestAccountResult{OK: true}, nil
}

func (a *fakeProviderAdapter) NormalizeError(_ context.Context, upstreamError UpstreamError) (*NormalizedError, error) {
	return &NormalizedError{StatusCode: upstreamError.StatusCode, Message: upstreamError.Message}, nil
}

func (a *fakeProviderAdapter) ChatStream(context.Context, ChatRequest) (<-chan ChatEvent, error) {
	events := make(chan ChatEvent)
	close(events)
	return events, nil
}

func (a *fakeProviderAdapter) Embed(context.Context, EmbeddingRequest) (*EmbeddingResponse, error) {
	return &EmbeddingResponse{}, nil
}

func (a *fakeProviderAdapter) CountTokens(context.Context, TokenCountRequest) (*TokenCountResponse, error) {
	return &TokenCountResponse{}, nil
}
