package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/LightBridge/internal/modules"
	"github.com/Wei-Shaw/LightBridge/internal/pkg/pagination"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProviderModuleBridgeForwardUsesRegisteredProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := modules.NewProviderRegistry()
	adapter := &fakeModuleProviderAdapter{
		id: "lightbridge.provider.test",
		events: []modules.GatewayEvent{
			{
				Type: "headers",
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
			{
				Type:  "data",
				Data:  json.RawMessage(`{"ok":true}`),
				Usage: nil,
			},
			{
				Type:  "usage",
				Usage: &modules.TokenUsage{InputTokens: 7, OutputTokens: 11, TotalTokens: 18},
			},
			{Type: "done"},
		},
	}
	registry.Register(adapter)

	svc := &GatewayService{}
	svc.SetProviderRegistry(registry)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	ctx.Request.Header.Set("Authorization", "Bearer downstream-key")
	ctx.Request.Header.Set("X-Client", "test-client")

	result, handled, err := svc.forwardModuleProvider(context.Background(), ctx, &Account{
		ID:         42,
		Name:       "Module Test",
		Platform:   "lightbridge.provider.test",
		ProviderID: "lightbridge.provider.test",
		Type:       AccountTypeModule,
		Credentials: map[string]any{
			"api_key": "secret",
		},
		Extra: map[string]any{
			"module_id": "lightbridge.provider.test",
		},
	}, &ParsedRequest{
		Body:   []byte(`{"model":"test-model"}`),
		Model:  "test-model",
		Stream: false,
	}, time.Now())

	require.NoError(t, err)
	require.True(t, handled)
	require.NotNil(t, result)
	require.Equal(t, "test-model", result.Model)
	require.Equal(t, 7, result.Usage.InputTokens)
	require.Equal(t, 11, result.Usage.OutputTokens)
	require.JSONEq(t, `{"ok":true}`, recorder.Body.String())
	require.Equal(t, "test-client", adapter.seenReq.Headers["X-Client"][0])
	require.Empty(t, adapter.seenReq.Headers["Authorization"])
	require.Equal(t, "secret", adapter.seenReq.Account.Secrets["api_key"])
}

func TestProviderModuleBridgeGatewayForwardAsChatCompletionsUsesRegisteredProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := modules.NewProviderRegistry()
	adapter := &fakeModuleProviderAdapter{
		id: "lightbridge.provider.test",
		events: []modules.GatewayEvent{
			{
				Type: "headers",
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
			{
				Type: "data",
				Data: json.RawMessage(`{"id":"chatcmpl-module","choices":[{"message":{"role":"assistant","content":"ok"}}]}`),
			},
			{
				Type:  "usage",
				Usage: &modules.TokenUsage{InputTokens: 2, OutputTokens: 4, TotalTokens: 6},
			},
		},
	}
	registry.Register(adapter)

	svc := &GatewayService{}
	svc.SetProviderRegistry(registry)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Authorization", "Bearer downstream-key")
	ctx.Request.Header.Set("X-Client", "test-client")

	body := []byte(`{"model":"module-model","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	result, err := svc.ForwardAsChatCompletions(context.Background(), ctx, &Account{
		ID:         44,
		Name:       "Module Chat",
		Platform:   PlatformModule,
		ProviderID: "lightbridge.provider.test",
		Type:       AccountTypeModule,
		Credentials: map[string]any{
			"api_key": "secret",
		},
		Extra: map[string]any{
			"module_id": "lightbridge.provider.test",
		},
	}, body, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "module-model", result.Model)
	require.Equal(t, 2, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.JSONEq(t, `{"id":"chatcmpl-module","choices":[{"message":{"role":"assistant","content":"ok"}}]}`, recorder.Body.String())
	require.Equal(t, "/v1/chat/completions", adapter.seenReq.Endpoint)
	require.Equal(t, "chat_completions", adapter.seenReq.DownstreamProtocol)
	require.JSONEq(t, string(body), string(adapter.seenReq.Body))
	require.Equal(t, "test-client", adapter.seenReq.Headers["X-Client"][0])
	require.Empty(t, adapter.seenReq.Headers["Authorization"])
	require.Equal(t, "secret", adapter.seenReq.Account.Secrets["api_key"])
}

func TestProviderModuleBridgeGatewayForwardAsChatCompletionsUsesForwardBodyOverParsedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := modules.NewProviderRegistry()
	adapter := &fakeModuleProviderAdapter{
		id: "lightbridge.provider.test",
		events: []modules.GatewayEvent{
			{
				Type: "data",
				Data: json.RawMessage(`{"id":"chatcmpl-module","choices":[{"message":{"role":"assistant","content":"mapped"}}]}`),
			},
		},
	}
	registry.Register(adapter)

	svc := &GatewayService{}
	svc.SetProviderRegistry(registry)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	originalBody := []byte(`{"model":"client-model","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	forwardBody := []byte(`{"model":"mapped-model","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	parsed, err := ParseGatewayRequest(originalBody, "chat_completions")
	require.NoError(t, err)

	result, err := svc.ForwardAsChatCompletions(context.Background(), ctx, &Account{
		ID:         45,
		Name:       "Module Chat",
		Platform:   PlatformModule,
		ProviderID: "lightbridge.provider.test",
		Type:       AccountTypeModule,
		Extra: map[string]any{
			"module_id": "lightbridge.provider.test",
		},
	}, forwardBody, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "mapped-model", result.Model)
	require.False(t, result.Stream)
	require.JSONEq(t, string(forwardBody), string(adapter.seenReq.Body))
	require.Equal(t, "mapped-model", adapter.seenReq.Metadata["model"])
}

func TestProviderModuleBridgeGatewayForwardAsChatCompletionsRejectsInvalidJSONBeforeProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := modules.NewProviderRegistry()
	adapter := &fakeModuleProviderAdapter{id: "lightbridge.provider.test"}
	registry.Register(adapter)

	svc := &GatewayService{}
	svc.SetProviderRegistry(registry)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	result, err := svc.ForwardAsChatCompletions(context.Background(), ctx, &Account{
		ID:         46,
		Name:       "Module Chat",
		Platform:   PlatformModule,
		ProviderID: "lightbridge.provider.test",
		Type:       AccountTypeModule,
		Extra: map[string]any{
			"module_id": "lightbridge.provider.test",
		},
	}, []byte(`{"model":`), nil)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "invalid json")
	require.Equal(t, 0, adapter.forwardCalls)
}

func TestProviderModuleBridgeDoesNotHandleLegacyProvider(t *testing.T) {
	registry := modules.NewProviderRegistry()
	registry.Register(&fakeModuleProviderAdapter{id: PlatformAnthropic})

	svc := &GatewayService{}
	svc.SetProviderRegistry(registry)

	_, handled, err := svc.forwardModuleProvider(context.Background(), nil, &Account{
		ID:       1,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
	}, &ParsedRequest{Body: []byte(`{}`)}, time.Now())

	require.NoError(t, err)
	require.False(t, handled)
}

func TestProviderModuleBridgeModuleAccountRequiresRegisteredProvider(t *testing.T) {
	svc := &GatewayService{}
	svc.SetProviderRegistry(modules.NewProviderRegistry())

	_, handled, err := svc.forwardModuleProvider(context.Background(), nil, &Account{
		ID:         9,
		Platform:   PlatformModule,
		ProviderID: "lightbridge.provider.missing",
		Type:       AccountTypeModule,
		Extra: map[string]any{
			"module_id": "lightbridge.provider.missing",
		},
	}, &ParsedRequest{Body: []byte(`{}`)}, time.Now())

	require.Error(t, err)
	require.True(t, handled)
	require.Contains(t, err.Error(), "not registered")
}

func TestProviderModuleBridgeModuleAccountRequiresProviderID(t *testing.T) {
	svc := &GatewayService{}
	svc.SetProviderRegistry(modules.NewProviderRegistry())

	_, handled, err := svc.forwardModuleProvider(context.Background(), nil, &Account{
		ID:       10,
		Platform: PlatformModule,
		Type:     AccountTypeModule,
	}, &ParsedRequest{Body: []byte(`{}`)}, time.Now())

	require.Error(t, err)
	require.True(t, handled)
	require.Contains(t, err.Error(), "has no provider_id")
}

func TestProviderModuleBridgeModuleTypeDoesNotFallbackToPlatformProviderID(t *testing.T) {
	svc := &GatewayService{}
	svc.SetProviderRegistry(modules.NewProviderRegistry())

	_, handled, err := svc.forwardModuleProvider(context.Background(), nil, &Account{
		ID:       11,
		Platform: "lightbridge.provider.test",
		Type:     AccountTypeModule,
	}, &ParsedRequest{Body: []byte(`{}`)}, time.Now())

	require.Error(t, err)
	require.True(t, handled)
	require.Contains(t, err.Error(), "has no provider_id")
}

func TestProviderModuleBridgeNormalizeAccountProviderID(t *testing.T) {
	require.Equal(t, PlatformAnthropic, normalizeAccountProviderID("", PlatformAnthropic, AccountTypeAPIKey, nil))
	require.Equal(t, "lightbridge.provider.test", normalizeAccountProviderID(" lightbridge.provider.test ", PlatformModule, AccountTypeModule, nil))
	require.Equal(t, "lightbridge.provider.extra", normalizeAccountProviderID("", PlatformModule, AccountTypeModule, map[string]any{
		"provider_id": " lightbridge.provider.extra ",
	}))
	require.Empty(t, normalizeAccountProviderID("", PlatformModule, AccountTypeModule, nil))
	require.Empty(t, normalizeAccountProviderID("", "lightbridge.provider.test", AccountTypeModule, nil))
}

func TestProviderModuleBridgeAccountProviderMatches(t *testing.T) {
	require.True(t, accountProviderMatches(&Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
	}, PlatformAnthropic))
	require.True(t, accountProviderMatches(&Account{
		Platform:   PlatformModule,
		ProviderID: "lightbridge.provider.test",
		Type:       AccountTypeModule,
	}, "lightbridge.provider.test"))
	require.True(t, accountProviderMatches(&Account{
		Platform: PlatformModule,
		Type:     AccountTypeModule,
		Extra: map[string]any{
			"provider_id": "lightbridge.provider.extra",
		},
	}, "lightbridge.provider.extra"))
	require.False(t, accountProviderMatches(&Account{
		Platform: PlatformModule,
		Type:     AccountTypeModule,
	}, PlatformModule))
	require.False(t, accountProviderMatches(&Account{
		Platform: "lightbridge.provider.test",
		Type:     AccountTypeModule,
	}, "lightbridge.provider.test"))
}

func TestProviderModuleBridgeSchedulerRebuildPlatformsForAccount(t *testing.T) {
	require.Equal(t, []string{PlatformAnthropic}, schedulerRebuildPlatformsForAccount(&Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
	}))
	require.Equal(t, []string{"lightbridge.provider.test"}, schedulerRebuildPlatformsForAccount(&Account{
		Platform:   PlatformModule,
		ProviderID: "lightbridge.provider.test",
		Type:       AccountTypeModule,
	}))
	require.Equal(t, []string{"lightbridge.provider.extra"}, schedulerRebuildPlatformsForAccount(&Account{
		Platform: PlatformModule,
		Type:     AccountTypeModule,
		Extra: map[string]any{
			"provider_id": "lightbridge.provider.extra",
		},
	}))
	require.Empty(t, schedulerRebuildPlatformsForAccount(&Account{
		Platform: PlatformModule,
		Type:     AccountTypeModule,
	}))
	require.Equal(t, []string{PlatformAntigravity, PlatformAnthropic, PlatformGemini}, schedulerRebuildPlatformsForAccount(&Account{
		Platform: PlatformAntigravity,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"mixed_scheduling": true,
		},
	}))
}

func TestProviderModuleBridgeBulkAccountEventRebuildsModuleProviderBuckets(t *testing.T) {
	const providerID = "lightbridge.provider.test"
	ctx := context.Background()
	cache := &moduleProviderBulkEventCache{}
	repo := &moduleProviderBulkEventRepo{
		accounts: []*Account{
			{
				ID:          1001,
				Platform:    PlatformModule,
				ProviderID:  providerID,
				Type:        AccountTypeModule,
				Status:      StatusActive,
				Schedulable: true,
				GroupIDs:    []int64{42},
			},
		},
	}
	svc := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)

	err := svc.handleBulkAccountEvent(ctx, map[string]any{
		"account_ids": []any{int64(1001)},
		"group_ids":   []any{int64(42)},
	}, make(map[batchSeenKey]struct{}))

	require.NoError(t, err)
	require.Contains(t, cache.snapshotBuckets, SchedulerBucket{GroupID: 42, Platform: providerID, Mode: SchedulerModeSingle})
	require.Contains(t, cache.snapshotBuckets, SchedulerBucket{GroupID: 42, Platform: providerID, Mode: SchedulerModeForced})
	require.NotContains(t, cache.snapshotBuckets, SchedulerBucket{GroupID: 42, Platform: PlatformModule, Mode: SchedulerModeSingle})
}

func TestProviderModuleBridgeForwardAcceptsUsageOnDataEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := modules.NewProviderRegistry()
	adapter := &fakeModuleProviderAdapter{
		id: "lightbridge.provider.test",
		events: []modules.GatewayEvent{
			{
				Type:  "data",
				Data:  json.RawMessage(`{"ok":true}`),
				Usage: &modules.TokenUsage{InputTokens: 3, OutputTokens: 5, TotalTokens: 8},
			},
		},
	}
	registry.Register(adapter)

	svc := &GatewayService{}
	svc.SetProviderRegistry(registry)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	result, handled, err := svc.forwardModuleProvider(context.Background(), ctx, &Account{
		ID:         43,
		Platform:   PlatformModule,
		ProviderID: "lightbridge.provider.test",
		Type:       AccountTypeModule,
	}, &ParsedRequest{Body: []byte(`{"model":"test-model"}`)}, time.Now())

	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
}

type fakeModuleProviderAdapter struct {
	id           string
	events       []modules.GatewayEvent
	seenReq      modules.GatewayRequest
	forwardCalls int
}

func (a *fakeModuleProviderAdapter) ID() string {
	return a.id
}

func (a *fakeModuleProviderAdapter) Metadata(context.Context) (*modules.ProviderMetadata, error) {
	return &modules.ProviderMetadata{ID: a.id}, nil
}

func (a *fakeModuleProviderAdapter) HealthCheck(context.Context) error {
	return nil
}

func (a *fakeModuleProviderAdapter) ListModels(context.Context, modules.ListModelsRequest) (*modules.ListModelsResponse, error) {
	return &modules.ListModelsResponse{}, nil
}

func (a *fakeModuleProviderAdapter) ValidateAccount(context.Context, modules.ProviderAccount) (*modules.AccountValidationResult, error) {
	return &modules.AccountValidationResult{Valid: true}, nil
}

func (a *fakeModuleProviderAdapter) RefreshAccount(context.Context, modules.ProviderAccount) (*modules.ProviderAccount, error) {
	return &modules.ProviderAccount{}, nil
}

func (a *fakeModuleProviderAdapter) Forward(_ context.Context, req modules.GatewayRequest) (<-chan modules.GatewayEvent, error) {
	a.forwardCalls++
	a.seenReq = req
	ch := make(chan modules.GatewayEvent, len(a.events))
	for _, event := range a.events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (a *fakeModuleProviderAdapter) TestAccount(context.Context, modules.TestAccountRequest) (*modules.TestAccountResult, error) {
	return &modules.TestAccountResult{OK: true}, nil
}

func (a *fakeModuleProviderAdapter) NormalizeError(context.Context, modules.UpstreamError) (*modules.NormalizedError, error) {
	return &modules.NormalizedError{}, nil
}

func (a *fakeModuleProviderAdapter) ChatStream(context.Context, modules.ChatRequest) (<-chan modules.ChatEvent, error) {
	ch := make(chan modules.ChatEvent)
	close(ch)
	return ch, nil
}

func (a *fakeModuleProviderAdapter) Embed(context.Context, modules.EmbeddingRequest) (*modules.EmbeddingResponse, error) {
	return &modules.EmbeddingResponse{}, nil
}

func (a *fakeModuleProviderAdapter) CountTokens(context.Context, modules.TokenCountRequest) (*modules.TokenCountResponse, error) {
	return &modules.TokenCountResponse{}, nil
}

type moduleProviderBulkEventCache struct {
	snapshotBuckets []SchedulerBucket
}

func (c *moduleProviderBulkEventCache) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}

func (c *moduleProviderBulkEventCache) SetSnapshot(_ context.Context, bucket SchedulerBucket, _ []Account) error {
	c.snapshotBuckets = append(c.snapshotBuckets, bucket)
	return nil
}

func (c *moduleProviderBulkEventCache) GetAccount(context.Context, int64) (*Account, error) {
	return nil, nil
}

func (c *moduleProviderBulkEventCache) SetAccount(context.Context, *Account) error {
	return nil
}

func (c *moduleProviderBulkEventCache) DeleteAccount(context.Context, int64) error {
	return nil
}

func (c *moduleProviderBulkEventCache) UpdateLastUsed(context.Context, map[int64]time.Time) error {
	return nil
}

func (c *moduleProviderBulkEventCache) TryLockBucket(context.Context, SchedulerBucket, time.Duration) (bool, error) {
	return true, nil
}

func (c *moduleProviderBulkEventCache) UnlockBucket(context.Context, SchedulerBucket) error {
	return nil
}

func (c *moduleProviderBulkEventCache) ListBuckets(context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}

func (c *moduleProviderBulkEventCache) GetOutboxWatermark(context.Context) (int64, error) {
	return 0, nil
}

func (c *moduleProviderBulkEventCache) SetOutboxWatermark(context.Context, int64) error {
	return nil
}

type moduleProviderBulkEventRepo struct {
	accounts []*Account
}

func (r *moduleProviderBulkEventRepo) Create(context.Context, *Account) error { return nil }

func (r *moduleProviderBulkEventRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	for _, account := range r.accounts {
		if account != nil && account.ID == id {
			return account, nil
		}
	}
	return nil, ErrAccountNotFound
}

func (r *moduleProviderBulkEventRepo) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	var result []*Account
	for _, account := range r.accounts {
		if account == nil {
			continue
		}
		if _, ok := idSet[account.ID]; ok {
			result = append(result, account)
		}
	}
	return result, nil
}

func (r *moduleProviderBulkEventRepo) ExistsByID(_ context.Context, id int64) (bool, error) {
	for _, account := range r.accounts {
		if account != nil && account.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func (r *moduleProviderBulkEventRepo) GetByCRSAccountID(context.Context, string) (*Account, error) {
	return nil, nil
}

func (r *moduleProviderBulkEventRepo) FindByExtraField(context.Context, string, any) ([]Account, error) {
	return nil, nil
}

func (r *moduleProviderBulkEventRepo) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	return nil, nil
}

func (r *moduleProviderBulkEventRepo) Update(context.Context, *Account) error { return nil }

func (r *moduleProviderBulkEventRepo) Delete(context.Context, int64) error { return nil }

func (r *moduleProviderBulkEventRepo) List(context.Context, pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *moduleProviderBulkEventRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *moduleProviderBulkEventRepo) ListByGroup(context.Context, int64) ([]Account, error) {
	return nil, nil
}

func (r *moduleProviderBulkEventRepo) ListActive(context.Context) ([]Account, error) {
	return nil, nil
}

func (r *moduleProviderBulkEventRepo) ListByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}

func (r *moduleProviderBulkEventRepo) UpdateLastUsed(context.Context, int64) error { return nil }

func (r *moduleProviderBulkEventRepo) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) SetError(context.Context, int64, string) error { return nil }

func (r *moduleProviderBulkEventRepo) ClearError(context.Context, int64) error { return nil }

func (r *moduleProviderBulkEventRepo) SetSchedulable(context.Context, int64, bool) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (r *moduleProviderBulkEventRepo) BindGroups(context.Context, int64, []int64) error { return nil }

func (r *moduleProviderBulkEventRepo) ListSchedulable(context.Context) ([]Account, error) {
	return r.listSchedulableByProvider("")
}

func (r *moduleProviderBulkEventRepo) ListSchedulableByGroupID(_ context.Context, groupID int64) ([]Account, error) {
	return r.listSchedulableByGroupAndProvider(groupID, "")
}

func (r *moduleProviderBulkEventRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]Account, error) {
	return r.listSchedulableByProvider(platform)
}

func (r *moduleProviderBulkEventRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]Account, error) {
	return r.listSchedulableByGroupAndProvider(groupID, platform)
}

func (r *moduleProviderBulkEventRepo) ListSchedulableByPlatforms(_ context.Context, platforms []string) ([]Account, error) {
	return r.listSchedulableByProviders(platforms)
}

func (r *moduleProviderBulkEventRepo) ListSchedulableByGroupIDAndPlatforms(_ context.Context, groupID int64, platforms []string) ([]Account, error) {
	var result []Account
	for _, account := range r.listSchedulableByProviders(platforms) {
		if moduleProviderBulkAccountHasGroup(account, groupID) {
			result = append(result, account)
		}
	}
	return result, nil
}

func (r *moduleProviderBulkEventRepo) ListSchedulableUngroupedByPlatform(_ context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, account := range r.listSchedulableByProvider(platform) {
		if len(account.GroupIDs) == 0 {
			result = append(result, account)
		}
	}
	return result, nil
}

func (r *moduleProviderBulkEventRepo) ListSchedulableUngroupedByPlatforms(_ context.Context, platforms []string) ([]Account, error) {
	var result []Account
	for _, account := range r.listSchedulableByProviders(platforms) {
		if len(account.GroupIDs) == 0 {
			result = append(result, account)
		}
	}
	return result, nil
}

func (r *moduleProviderBulkEventRepo) SetRateLimited(context.Context, int64, time.Time) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) SetModelRateLimit(context.Context, int64, string, time.Time, ...string) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) SetOverloaded(context.Context, int64, time.Time) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) ClearTempUnschedulable(context.Context, int64) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) ClearRateLimit(context.Context, int64) error { return nil }

func (r *moduleProviderBulkEventRepo) ClearAntigravityQuotaScopes(context.Context, int64) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) ClearModelRateLimits(context.Context, int64) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) UpdateExtra(context.Context, int64, map[string]any) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) BulkUpdate(context.Context, []int64, AccountBulkUpdate) (int64, error) {
	return 0, nil
}

func (r *moduleProviderBulkEventRepo) IncrementQuotaUsed(context.Context, int64, float64) error {
	return nil
}

func (r *moduleProviderBulkEventRepo) ResetQuotaUsed(context.Context, int64) error { return nil }

func (r *moduleProviderBulkEventRepo) listSchedulableByGroupAndProvider(groupID int64, providerID string) []Account {
	var result []Account
	for _, account := range r.listSchedulableByProvider(providerID) {
		if groupID <= 0 || moduleProviderBulkAccountHasGroup(account, groupID) {
			result = append(result, account)
		}
	}
	return result
}

func moduleProviderBulkAccountHasGroup(account Account, groupID int64) bool {
	for _, id := range account.GroupIDs {
		if id == groupID {
			return true
		}
	}
	return false
}

func (r *moduleProviderBulkEventRepo) listSchedulableByProvider(providerID string) []Account {
	if providerID == "" {
		return r.listSchedulableByProviders(nil)
	}
	return r.listSchedulableByProviders([]string{providerID})
}

func (r *moduleProviderBulkEventRepo) listSchedulableByProviders(providerIDs []string) []Account {
	var result []Account
	for _, account := range r.accounts {
		if account == nil || !account.IsSchedulable() {
			continue
		}
		if len(providerIDs) > 0 {
			matched := false
			for _, providerID := range providerIDs {
				if accountProviderMatches(account, providerID) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, *account)
	}
	return result
}
