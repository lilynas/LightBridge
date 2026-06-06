package modules

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type ProviderRuntime interface {
	StartProvider(ctx context.Context, module InstalledModule) error
	StopProvider(ctx context.Context, moduleID string) error
}

type SidecarProviderRuntime struct {
	baseDir      string
	registry     *ProviderRegistry
	runtimeStore RuntimeStore
	coreBridge   CoreBridge

	mu        sync.Mutex
	instances map[string]*sidecarProviderInstance
}

type sidecarProviderInstance struct {
	cmd              *exec.Cmd
	socket           string
	adapter          ProviderAdapter
	closer           io.Closer
	coreBridgeSocket string
	coreBridgeCloser io.Closer
	stdout           *os.File
	stderr           *os.File
	stopping         bool
}

func NewSidecarProviderRuntime(baseDir string, registry *ProviderRegistry) *SidecarProviderRuntime {
	return NewSidecarProviderRuntimeWithStore(baseDir, registry, nil)
}

func NewSidecarProviderRuntimeWithStore(baseDir string, registry *ProviderRegistry, runtimeStore RuntimeStore) *SidecarProviderRuntime {
	return NewSidecarProviderRuntimeWithStoreAndBridge(baseDir, registry, runtimeStore, nil)
}

func NewSidecarProviderRuntimeWithStoreAndBridge(baseDir string, registry *ProviderRegistry, runtimeStore RuntimeStore, bridge CoreBridge) *SidecarProviderRuntime {
	if baseDir == "" {
		baseDir = "data"
	}
	return &SidecarProviderRuntime{
		baseDir:      baseDir,
		registry:     registry,
		runtimeStore: runtimeStore,
		coreBridge:   bridge,
		instances:    make(map[string]*sidecarProviderInstance),
	}
}

func (r *SidecarProviderRuntime) StartProvider(ctx context.Context, module InstalledModule) error {
	if r == nil || r.registry == nil {
		return errors.New("provider runtime is not configured")
	}
	if !slices.Contains(module.Manifest.Capabilities, CapabilityProviderAdapter) {
		return nil
	}
	if module.Manifest.Backend == nil {
		return fmt.Errorf("module %s has no backend spec", module.ID)
	}
	backend := module.Manifest.Backend
	if backend.Kind != BackendKindSidecar {
		return fmt.Errorf("module %s backend kind %q is not supported", module.ID, backend.Kind)
	}

	socketPath := r.socketPath(module)
	adapter, closer, err := newSidecarProviderAdapter(module.ID, socketPath, backend.Protocol)
	if err != nil {
		return err
	}

	r.mu.Lock()
	if previous := r.instances[module.ID]; previous != nil {
		r.stopLocked(context.Background(), module.ID, previous, RuntimeStatusStopped, "")
	}
	r.mu.Unlock()

	commandPath := filepath.Join(module.InstallPath, filepath.Clean(backend.Command))
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return fmt.Errorf("create module runtime dir: %w", err)
	}
	_ = os.Remove(socketPath)

	stdout, stderr, err := r.openLogFiles(module.ID)
	if err != nil {
		if closer != nil {
			_ = closer.Close()
		}
		return err
	}

	coreBridgeSocket := ""
	var coreBridgeCloser io.Closer
	if r.coreBridge != nil {
		coreBridgeSocket = CoreBridgeSocketPath(r.baseDir, module.ID)
		coreBridgeCloser, err = StartCoreBridgeServer(module.ID, coreBridgeSocket, r.coreBridge)
		if err != nil {
			if closer != nil {
				_ = closer.Close()
			}
			closeSidecarLogs(stdout, stderr)
			return err
		}
	}

	env := append(os.Environ(),
		"LIGHTBRIDGE_MODULE_ID="+module.ID,
		"LIGHTBRIDGE_MODULE_VERSION="+module.Version,
		"LIGHTBRIDGE_MODULE_INSTALL_PATH="+module.InstallPath,
		"LIGHTBRIDGE_MODULE_SOCKET="+socketPath,
	)
	if coreBridgeSocket != "" {
		env = append(env, "LIGHTBRIDGE_CORE_BRIDGE_SOCKET="+coreBridgeSocket)
	}

	cmd := exec.Command(commandPath)
	cmd.Dir = module.InstallPath
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = env
	now := time.Now().UTC()
	r.recordRuntime(ctx, RuntimeInstanceUpdate{
		ModuleID:        module.ID,
		Status:          RuntimeStatusStarting,
		SocketPath:      socketPath,
		StartedAt:       &now,
		LastHeartbeatAt: &now,
	})
	if err := cmd.Start(); err != nil {
		if closer != nil {
			_ = closer.Close()
		}
		closeCoreBridgeRuntime(coreBridgeCloser, coreBridgeSocket)
		closeSidecarLogs(stdout, stderr)
		r.recordRuntime(ctx, RuntimeInstanceUpdate{
			ModuleID:   module.ID,
			Status:     RuntimeStatusFailed,
			SocketPath: socketPath,
			LastError:  err.Error(),
		})
		return fmt.Errorf("start provider sidecar %s: %w", module.ID, err)
	}

	instance := &sidecarProviderInstance{
		cmd:              cmd,
		socket:           socketPath,
		adapter:          adapter,
		closer:           closer,
		coreBridgeSocket: coreBridgeSocket,
		coreBridgeCloser: coreBridgeCloser,
		stdout:           stdout,
		stderr:           stderr,
	}
	if err := waitProviderHealthy(ctx, adapter, backend.Healthcheck); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		cleanupSidecarProviderInstance(instance)
		r.recordRuntime(ctx, RuntimeInstanceUpdate{
			ModuleID:   module.ID,
			Status:     RuntimeStatusFailed,
			SocketPath: socketPath,
			LastError:  err.Error(),
		})
		return err
	}
	if err := validateProviderMetadataIdentity(ctx, module.ID, adapter, backend.Healthcheck); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		cleanupSidecarProviderInstance(instance)
		r.recordRuntime(ctx, RuntimeInstanceUpdate{
			ModuleID:   module.ID,
			Status:     RuntimeStatusFailed,
			SocketPath: socketPath,
			LastError:  err.Error(),
		})
		return err
	}

	r.registry.Register(adapter)
	r.mu.Lock()
	r.instances[module.ID] = instance
	r.mu.Unlock()
	pid := cmd.Process.Pid
	r.recordRuntime(ctx, RuntimeInstanceUpdate{
		ModuleID:        module.ID,
		Status:          RuntimeStatusRunning,
		PID:             &pid,
		SocketPath:      socketPath,
		StartedAt:       &now,
		LastHeartbeatAt: &now,
	})
	go r.waitProviderExit(module.ID, instance)
	return nil
}

func (r *SidecarProviderRuntime) StopProvider(ctx context.Context, moduleID string) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	instance := r.instances[moduleID]
	if instance == nil {
		if r.registry != nil {
			r.registry.Unregister(moduleID)
		}
		return nil
	}
	return r.stopLocked(ctx, moduleID, instance, RuntimeStatusStopped, "")
}

func (r *SidecarProviderRuntime) stopLocked(ctx context.Context, moduleID string, instance *sidecarProviderInstance, status RuntimeStatus, lastError string) error {
	if r.registry != nil {
		r.registry.Unregister(moduleID)
	}
	delete(r.instances, moduleID)
	now := time.Now().UTC()
	r.recordRuntime(ctx, RuntimeInstanceUpdate{
		ModuleID:  moduleID,
		Status:    status,
		StoppedAt: &now,
		LastError: lastError,
	})
	if instance == nil || instance.cmd == nil || instance.cmd.Process == nil {
		cleanupSidecarProviderInstance(instance)
		return nil
	}
	instance.stopping = true
	if err := instance.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		cleanupSidecarProviderInstance(instance)
		return fmt.Errorf("stop provider sidecar %s: %w", moduleID, err)
	}
	if instance.socket != "" {
		_ = os.Remove(instance.socket)
	}
	if instance.closer != nil {
		_ = instance.closer.Close()
	}
	closeCoreBridgeRuntime(instance.coreBridgeCloser, instance.coreBridgeSocket)
	return nil
}

func (r *SidecarProviderRuntime) waitProviderExit(moduleID string, instance *sidecarProviderInstance) {
	err := instance.cmd.Wait()
	cleanupSidecarProviderInstance(instance)

	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.instances[moduleID]
	if current != instance {
		return
	}
	if r.registry != nil {
		r.registry.Unregister(moduleID)
	}
	delete(r.instances, moduleID)
	status := RuntimeStatusStopped
	lastError := ""
	if !instance.stopping {
		status = RuntimeStatusCrashed
		if err != nil {
			lastError = err.Error()
		} else {
			lastError = "provider sidecar exited"
		}
	}
	now := time.Now().UTC()
	r.recordRuntime(context.Background(), RuntimeInstanceUpdate{
		ModuleID:  moduleID,
		Status:    status,
		StoppedAt: &now,
		LastError: lastError,
	})
}

func (r *SidecarProviderRuntime) openLogFiles(moduleID string) (*os.File, *os.File, error) {
	logDir := filepath.Join(r.baseDir, "modules-runtime", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create module runtime log dir: %w", err)
	}
	baseName := safeRuntimeFilename(moduleID)
	stdout, err := os.OpenFile(filepath.Join(logDir, baseName+".stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open module stdout log: %w", err)
	}
	stderr, err := os.OpenFile(filepath.Join(logDir, baseName+".stderr.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = stdout.Close()
		return nil, nil, fmt.Errorf("open module stderr log: %w", err)
	}
	return stdout, stderr, nil
}

func (r *SidecarProviderRuntime) recordRuntime(ctx context.Context, update RuntimeInstanceUpdate) {
	if r == nil || r.runtimeStore == nil {
		return
	}
	_ = r.runtimeStore.UpdateRuntimeInstance(ctx, update)
}

func closeSidecarLogs(files ...*os.File) {
	for _, file := range files {
		if file != nil {
			_ = file.Close()
		}
	}
}

func cleanupSidecarProviderInstance(instance *sidecarProviderInstance) {
	if instance == nil {
		return
	}
	if instance.socket != "" {
		_ = os.Remove(instance.socket)
	}
	if instance.closer != nil {
		_ = instance.closer.Close()
	}
	closeCoreBridgeRuntime(instance.coreBridgeCloser, instance.coreBridgeSocket)
	closeSidecarLogs(instance.stdout, instance.stderr)
}

func closeCoreBridgeRuntime(closer io.Closer, socketPath string) {
	if closer != nil {
		_ = closer.Close()
	}
	if socketPath != "" {
		_ = os.Remove(socketPath)
	}
}

func safeRuntimeFilename(value string) string {
	if value == "" {
		return "module"
	}
	out := make([]rune, 0, len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}

func (r *SidecarProviderRuntime) socketPath(module InstalledModule) string {
	if module.Manifest.Backend != nil && module.Manifest.Backend.Socket != "" {
		socket := filepath.Clean(module.Manifest.Backend.Socket)
		if filepath.IsAbs(socket) {
			return socket
		}
		return filepath.Join(r.baseDir, socket)
	}
	return RuntimeSocketPath(r.baseDir, module.ID)
}

func waitProviderHealthy(ctx context.Context, adapter ProviderAdapter, healthcheck *HealthcheckSpec) error {
	timeout := 10 * time.Second
	if healthcheck != nil && healthcheck.Timeout.Duration > 0 {
		timeout = healthcheck.Timeout.Duration
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		if err := adapter.HealthCheck(deadlineCtx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("provider sidecar healthcheck failed: %w", lastErr)
		case <-ticker.C:
		}
	}
}

func validateProviderMetadataIdentity(ctx context.Context, moduleID string, adapter ProviderAdapter, healthcheck *HealthcheckSpec) error {
	timeout := 10 * time.Second
	if healthcheck != nil && healthcheck.Timeout.Duration > 0 {
		timeout = healthcheck.Timeout.Duration
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	metadata, err := adapter.Metadata(deadlineCtx)
	if err != nil {
		return fmt.Errorf("provider sidecar metadata failed: %w", err)
	}
	if metadata == nil {
		return fmt.Errorf("provider sidecar metadata is empty")
	}
	providerID := strings.TrimSpace(metadata.ID)
	if providerID == "" {
		return fmt.Errorf("provider sidecar metadata id is required")
	}
	if providerID != moduleID {
		return fmt.Errorf("provider sidecar metadata id %q must match module id %q", providerID, moduleID)
	}
	return nil
}

func newSidecarProviderAdapter(moduleID string, socketPath string, protocol BackendProtocol) (ProviderAdapter, io.Closer, error) {
	switch protocol {
	case BackendProtocolGRPC:
		adapter, err := NewGRPCProviderAdapter(moduleID, socketPath)
		if err != nil {
			return nil, nil, err
		}
		return adapter, adapter, nil
	case BackendProtocolConnect:
		return NewHTTPProviderAdapter(moduleID, socketPath), nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported backend protocol %q", protocol)
	}
}

type HTTPProviderAdapter struct {
	moduleID string
	client   *http.Client
	baseURL  string
}

const providerEventMaxLineSize = 32 * 1024 * 1024

func NewHTTPProviderAdapter(moduleID, socketPath string) *HTTPProviderAdapter {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	return &HTTPProviderAdapter{
		moduleID: moduleID,
		client:   &http.Client{Transport: transport},
		baseURL:  "http://lightbridge-module",
	}
}

func (a *HTTPProviderAdapter) ID() string {
	return a.moduleID
}

func (a *HTTPProviderAdapter) Metadata(ctx context.Context) (*ProviderMetadata, error) {
	var out ProviderMetadata
	if err := a.postJSON(ctx, "/provider/metadata", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *HTTPProviderAdapter) HealthCheck(ctx context.Context) error {
	return a.postJSON(ctx, "/provider/health", nil, nil)
}

func (a *HTTPProviderAdapter) ListModels(ctx context.Context, req ListModelsRequest) (*ListModelsResponse, error) {
	var out ListModelsResponse
	if err := a.postJSON(ctx, "/provider/models", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *HTTPProviderAdapter) ValidateAccount(ctx context.Context, account ProviderAccount) (*AccountValidationResult, error) {
	var out AccountValidationResult
	if err := a.postJSON(ctx, "/provider/validate-account", account, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *HTTPProviderAdapter) RefreshAccount(ctx context.Context, account ProviderAccount) (*ProviderAccount, error) {
	var out ProviderAccount
	if err := a.postJSON(ctx, "/provider/refresh-account", account, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *HTTPProviderAdapter) Forward(ctx context.Context, req GatewayRequest) (<-chan GatewayEvent, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/provider/forward", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		return nil, fmt.Errorf("provider %s forward returned HTTP %d", a.moduleID, resp.StatusCode)
	}

	events := make(chan GatewayEvent)
	go func() {
		defer close(events)
		defer func() { _ = resp.Body.Close() }()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), providerEventMaxLineSize)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var event GatewayEvent
			if err := json.Unmarshal(line, &event); err != nil {
				events <- GatewayEvent{Type: "error", Error: &NormalizedError{Message: err.Error()}}
				return
			}
			events <- event
		}
		if err := scanner.Err(); err != nil {
			events <- GatewayEvent{Type: "error", Error: &NormalizedError{Message: err.Error()}}
		}
	}()
	return events, nil
}

func (a *HTTPProviderAdapter) TestAccount(ctx context.Context, req TestAccountRequest) (*TestAccountResult, error) {
	var out TestAccountResult
	if err := a.postJSON(ctx, "/provider/test-account", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *HTTPProviderAdapter) NormalizeError(ctx context.Context, upstreamError UpstreamError) (*NormalizedError, error) {
	var out NormalizedError
	if err := a.postJSON(ctx, "/provider/normalize-error", upstreamError, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *HTTPProviderAdapter) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/provider/chat-stream", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		return nil, fmt.Errorf("provider %s chat stream returned HTTP %d", a.moduleID, resp.StatusCode)
	}

	events := make(chan ChatEvent)
	go func() {
		defer close(events)
		defer func() { _ = resp.Body.Close() }()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), providerEventMaxLineSize)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var event ChatEvent
			if err := json.Unmarshal(line, &event); err != nil {
				events <- ChatEvent{Type: "error", Error: err.Error()}
				return
			}
			events <- event
		}
		if err := scanner.Err(); err != nil {
			events <- ChatEvent{Type: "error", Error: err.Error()}
		}
	}()
	return events, nil
}

func (a *HTTPProviderAdapter) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	var out EmbeddingResponse
	if err := a.postJSON(ctx, "/provider/embed", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *HTTPProviderAdapter) CountTokens(ctx context.Context, req TokenCountRequest) (*TokenCountResponse, error) {
	var out TokenCountResponse
	if err := a.postJSON(ctx, "/provider/count-tokens", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *HTTPProviderAdapter) postJSON(ctx context.Context, endpoint string, input any, output any) error {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider %s endpoint %s returned HTTP %d", a.moduleID, endpoint, resp.StatusCode)
	}
	if output == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(output)
}
