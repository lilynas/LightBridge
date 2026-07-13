package main

import (
	"testing"
	"time"

	"github.com/WilliamWang1721/LightBridge/internal/config"
	"github.com/WilliamWang1721/LightBridge/internal/handler"
	"github.com/WilliamWang1721/LightBridge/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProvideServiceBuildInfo(t *testing.T) {
	in := handler.BuildInfo{
		Version:   "v-test",
		BuildType: "release",
	}
	out := provideServiceBuildInfo(in)
	require.Equal(t, in.Version, out.Version)
	require.Equal(t, in.BuildType, out.BuildType)
}

func TestProvideCleanup_WithMinimalDependencies_NoPanic(t *testing.T) {
	cfg := &config.Config{}

	openAIOAuthSvc := service.NewOpenAIOAuthService(nil, nil)
	grokOAuthSvc := service.NewGrokOAuthService(nil, nil)
	geminiOAuthSvc := service.NewGeminiOAuthService(nil, nil, nil, nil, cfg)
	antigravityOAuthSvc := service.NewAntigravityOAuthService(nil)

	tokenRefreshSvc := service.NewTokenRefreshService(
		nil,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		grokOAuthSvc,
		nil,
		nil,
		cfg,
		nil,
	)
	accountExpirySvc := service.NewAccountExpiryService(nil, time.Second)
	subscriptionExpirySvc := service.NewSubscriptionExpiryService(nil, time.Second)
	pricingSvc := service.NewPricingService(cfg, nil)
	emailQueueSvc := service.NewEmailQueueService(nil, 1)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	idempotencyCleanupSvc := service.NewIdempotencyCleanupService(nil, cfg)
	schedulerSnapshotSvc := service.NewSchedulerSnapshotService(nil, nil, nil, nil, cfg)
	cleanup := provideCleanup(
		nil, // entClient
		nil, // redis
		nil, // featureRuntime
		schedulerSnapshotSvc,
		tokenRefreshSvc,
		accountExpirySvc,
		subscriptionExpirySvc,
		idempotencyCleanupSvc,
		pricingSvc,
		emailQueueSvc,
		billingCacheSvc,
		&service.UsageRecordWorkerPool{},
		&service.SubscriptionService{},
		openAIOAuthSvc,
		grokOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil, // openAIGateway
		nil, // quotaFlusher
		nil, // aistudioProxyManager
	)

	require.NotPanics(t, func() {
		cleanup()
	})
}
