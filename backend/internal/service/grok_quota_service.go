package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/WilliamWang1721/LightBridge/internal/pkg/errors"
	"github.com/WilliamWang1721/LightBridge/internal/pkg/xai"
)

const (
	grokQuotaUpstreamTimeout = 20 * time.Second
	grokQuotaProbeInput      = "."
	grokQuotaDefaultModel    = "grok-4.5"
)

type GrokQuotaProbeResult struct {
	Source          string             `json:"source"`
	Snapshot        *xai.QuotaSnapshot `json:"snapshot,omitempty"`
	StatusCode      int                `json:"status_code,omitempty"`
	HeadersObserved bool               `json:"headers_observed"`
	ResetSupported  bool               `json:"reset_supported"`
	FetchedAt       int64              `json:"fetched_at"`
}

type GrokQuotaResetResult struct {
	Supported bool   `json:"supported"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type GrokQuotaService struct {
	accountRepo   AccountRepository
	proxyRepo     ProxyRepository
	tokenProvider *GrokTokenProvider
	httpUpstream  HTTPUpstream
}

func NewGrokQuotaService(
	accountRepo AccountRepository,
	proxyRepo ProxyRepository,
	tokenProvider *GrokTokenProvider,
	httpUpstream HTTPUpstream,
) *GrokQuotaService {
	return &GrokQuotaService{
		accountRepo:   accountRepo,
		proxyRepo:     proxyRepo,
		tokenProvider: tokenProvider,
		httpUpstream:  httpUpstream,
	}
}

func (s *GrokQuotaService) ProbeUsage(ctx context.Context, accountID int64) (*GrokQuotaProbeResult, error) {
	account, token, proxyURL, err := s.prepareProbe(ctx, accountID)
	if err != nil {
		return nil, err
	}

	body, err := buildGrokQuotaProbeBody(account)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadRequest, "GROK_QUOTA_PROBE_BODY_ERROR", "failed to build probe body: %v", err)
	}
	usingAPI := account.GrokUsingAPI()
	resolvedBaseURL, err := account.GetGrokChatBaseURL()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadRequest, "GROK_QUOTA_BASE_URL_INVALID", "invalid Grok base_url: %v", err)
	}
	targetURL, err := xai.BuildChatResponsesURL(account.GetCredential("base_url"), usingAPI)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadRequest, "GROK_QUOTA_BASE_URL_INVALID", "invalid Grok base_url: %v", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, grokQuotaUpstreamTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_QUOTA_PROBE_REQUEST_BUILD_FAILED", "failed to build upstream request: %v", err)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileGrok))
	xai.ApplyChatHeaders(req, token, false, usingAPI, resolvedBaseURL, "")
	if usingAPI {
		req.Header.Set("User-Agent", "lightbridge-grok-quota-probe/1.1")
	} else if strings.TrimSpace(req.Header.Get("User-Agent")) == "" {
		req.Header.Set("User-Agent", "lightbridge-grok-quota-probe/1.1")
	}

	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, maxInt(account.Concurrency, 1))
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_QUOTA_PROBE_REQUEST_FAILED", "upstream probe failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	snapshot := xai.ObserveQuotaHeaders(resp.Header, resp.StatusCode, "active_probe")
	_ = s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{
		grokQuotaSnapshotExtraKey: snapshot,
	})

	result := &GrokQuotaProbeResult{
		Source:          "active_probe",
		Snapshot:        snapshot,
		StatusCode:      resp.StatusCode,
		HeadersObserved: snapshot.HeadersObserved,
		ResetSupported:  false,
		FetchedAt:       time.Now().Unix(),
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		// A probe-side 429 is authoritative evidence that the account reached
		// Grok Build but is temporarily rate limited. Persist the cooldown so
		// all scheduler instances skip it immediately instead of issuing another
		// request as soon as account creation/reauthorization returns.
		cooldown := 2 * time.Minute
		if snapshot != nil && snapshot.RetryAfterSeconds != nil && *snapshot.RetryAfterSeconds > 0 {
			cooldown = time.Duration(*snapshot.RetryAfterSeconds) * time.Second
			if cooldown > 24*time.Hour {
				cooldown = 24 * time.Hour
			}
		}
		until := time.Now().Add(cooldown)
		if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, "grok build availability probe rate limited"); err != nil {
			slog.Warn("grok_quota_probe_cooldown_persist_failed", "account_id", account.ID, "error", err)
		} else {
			account.TempUnschedulableUntil = &until
			account.TempUnschedulableReason = "grok build availability probe rate limited"
		}
		return result, nil
	}
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		summary := summarizeGrokProbeFailure(resp.StatusCode, string(bodyBytes))
		slog.Warn("grok_quota_probe_failed", "account_id", account.ID, "status", resp.StatusCode, "classification", summary)
		return nil, infraerrors.Newf(mapUpstreamStatusCode(resp.StatusCode), "GROK_QUOTA_PROBE_UPSTREAM_ERROR", "upstream returned %d: %s", resp.StatusCode, summary)
	}
	return result, nil
}

// IsGrokProbeAuthorizationFailure reports whether a failed availability probe
// proves that the account cannot currently authenticate or is not entitled to
// the selected Grok upstream. Transient network, 429, and 5xx failures must not
// permanently disable an otherwise valid account.
func IsGrokProbeAuthorizationFailure(err error) bool {
	if err == nil {
		return false
	}
	code := infraerrors.Code(err)
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return true
	}
	if code != http.StatusBadRequest {
		return false
	}
	text := strings.ToLower(strings.Join([]string{
		infraerrors.Reason(err),
		infraerrors.Message(err),
	}, " "))
	for _, marker := range []string{
		"personal-team-blocked",
		"spending-limit",
		"not entitled",
		"not_entitled",
		"invalid token",
		"invalid_token",
		"token context",
		"token_context",
		"unauthorized",
		"forbidden",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// GrokProbeAuthorizationErrorMessage returns a stable, non-secret operator
// message suitable for persisting on an account. The upstream response body is
// intentionally not persisted because it may contain identifiers.
const GrokProbeAuthorizationErrorPrefix = "Grok Build availability verification failed"

func GrokProbeAuthorizationErrorMessage(err error) string {
	reason := strings.TrimSpace(infraerrors.Reason(err))
	if reason == "" {
		reason = "GROK_BUILD_AVAILABILITY_CHECK_FAILED"
	}
	return GrokProbeAuthorizationErrorPrefix + " (" + reason + "). Re-authorize the account using the Grok Build OAuth flow and verify that the subscription is entitled to Grok Build."
}

// summarizeGrokProbeFailure keeps only stable classification markers. Upstream
// error bodies can contain account or team identifiers and must not be logged,
// returned to administrators, or persisted verbatim.
func summarizeGrokProbeFailure(status int, raw string) string {
	lower := strings.ToLower(raw)
	for _, marker := range []string{
		"personal-team-blocked:spending-limit",
		"personal-team-blocked",
		"spending-limit",
		"not_entitled",
		"not entitled",
		"invalid_token",
		"invalid token",
		"unauthorized",
		"forbidden",
		"model_not_found",
		"model not found",
	} {
		if strings.Contains(lower, marker) {
			return marker
		}
	}
	switch status {
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusBadRequest:
		return "upstream request rejected"
	default:
		return "upstream availability check failed"
	}
}

func (s *GrokQuotaService) ResetQuota(ctx context.Context, accountID int64) (*GrokQuotaResetResult, error) {
	if _, err := s.loadGrokOAuthAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return nil, infraerrors.New(http.StatusNotImplemented, "GROK_QUOTA_RESET_UNSUPPORTED", "xAI does not expose a Grok subscription quota reset endpoint for OAuth accounts")
}

func (s *GrokQuotaService) prepareProbe(ctx context.Context, accountID int64) (*Account, string, string, error) {
	if s == nil || s.tokenProvider == nil || s.httpUpstream == nil {
		return nil, "", "", infraerrors.New(http.StatusInternalServerError, "GROK_QUOTA_NOT_CONFIGURED", "grok quota service is not configured")
	}
	account, err := s.loadGrokOAuthAccount(ctx, accountID)
	if err != nil {
		return nil, "", "", err
	}

	token, err := s.tokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		if errors.Is(err, ErrGrokBuildTokenContextMissing) {
			return nil, "", "", infraerrors.New(http.StatusForbidden, "GROK_BUILD_TOKEN_CONTEXT_MISSING", "Grok Build access token was not issued for the grok-build OAuth context; re-authorize the account")
		}
		return nil, "", "", infraerrors.Newf(http.StatusBadGateway, "GROK_QUOTA_TOKEN_UNAVAILABLE", "failed to acquire access token: %v", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, "", "", infraerrors.New(http.StatusBadGateway, "GROK_QUOTA_TOKEN_UNAVAILABLE", "access token is empty")
	}

	return account, token, s.resolveProxyURL(ctx, account), nil
}

func (s *GrokQuotaService) resolveProxyURL(ctx context.Context, account *Account) string {
	if account == nil || account.ProxyID == nil {
		return ""
	}
	switch {
	case account.Proxy != nil:
		return account.Proxy.URL()
	case s != nil && s.proxyRepo != nil:
		if proxy, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && proxy != nil {
			return proxy.URL()
		}
	}
	return ""
}

func (s *GrokQuotaService) loadGrokOAuthAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "GROK_QUOTA_NOT_CONFIGURED", "grok quota service is not configured")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusNotFound, "GROK_QUOTA_ACCOUNT_NOT_FOUND", "account not found: %v", err)
	}
	if account == nil {
		return nil, infraerrors.New(http.StatusNotFound, "GROK_QUOTA_ACCOUNT_NOT_FOUND", "account not found")
	}
	if account.Platform != PlatformGrok {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_QUOTA_INVALID_PLATFORM", "account is not a Grok account")
	}
	if account.Type != AccountTypeOAuth {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_QUOTA_INVALID_TYPE", "account is not an OAuth account")
	}
	return account, nil
}

func buildGrokQuotaProbeBody(account *Account) ([]byte, error) {
	model := grokQuotaDefaultModel
	if account != nil {
		if mapped, matched := account.ResolveMappedModel(model); matched && strings.TrimSpace(mapped) != "" {
			model = strings.TrimSpace(mapped)
		}
	}
	return json.Marshal(map[string]any{
		"model":             model,
		"input":             grokQuotaProbeInput,
		"max_output_tokens": 1,
		"store":             false,
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
