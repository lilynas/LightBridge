package service

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/WilliamWang1721/LightBridge/internal/pkg/errors"
	"github.com/WilliamWang1721/LightBridge/internal/pkg/xai"
)

const grokDefaultAccessTokenTTL = 6 * time.Hour

type GrokOAuthClient interface {
	ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*xai.TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error)
}

type GrokOAuthTokenService interface {
	RefreshAccountToken(ctx context.Context, account *Account) (*GrokTokenInfo, error)
	BuildAccountCredentials(tokenInfo *GrokTokenInfo) map[string]any
}

type GrokOAuthSessionStore interface {
	Set(ctx context.Context, sessionID string, session *xai.OAuthSession, ttl time.Duration) error
	Consume(ctx context.Context, sessionID string) (*xai.OAuthSession, bool, error)
	Stop()
}

type localGrokOAuthSessionStore struct {
	store *xai.SessionStore
}

func (s *localGrokOAuthSessionStore) Set(_ context.Context, sessionID string, session *xai.OAuthSession, _ time.Duration) error {
	s.store.Set(sessionID, session)
	return nil
}

func (s *localGrokOAuthSessionStore) Consume(_ context.Context, sessionID string) (*xai.OAuthSession, bool, error) {
	session, ok := s.store.Consume(sessionID)
	return session, ok, nil
}

func (s *localGrokOAuthSessionStore) Stop() {
	if s != nil && s.store != nil {
		s.store.Stop()
	}
}

type GrokOAuthService struct {
	sessionStore GrokOAuthSessionStore
	proxyRepo    ProxyRepository
	oauthClient  GrokOAuthClient
}

func NewGrokOAuthService(proxyRepo ProxyRepository, oauthClient GrokOAuthClient, stores ...GrokOAuthSessionStore) *GrokOAuthService {
	var store GrokOAuthSessionStore
	if len(stores) > 0 {
		store = stores[0]
	}
	if store == nil {
		store = &localGrokOAuthSessionStore{store: xai.NewSessionStore()}
	}
	return &GrokOAuthService{
		sessionStore: store,
		proxyRepo:    proxyRepo,
		oauthClient:  oauthClient,
	}
}

func ProvideGrokOAuthService(proxyRepo ProxyRepository, oauthClient GrokOAuthClient, sessionStore GrokOAuthSessionStore) *GrokOAuthService {
	return NewGrokOAuthService(proxyRepo, oauthClient, sessionStore)
}

type GrokAuthURLResult struct {
	AuthURL   string        `json:"auth_url"`
	SessionID string        `json:"session_id"`
	State     string        `json:"state"`
	OAuthMode xai.OAuthMode `json:"oauth_mode"`
}

func (s *GrokOAuthService) GenerateAuthURL(ctx context.Context, proxyID *int64, redirectURI string, modeValues ...string) (*GrokAuthURLResult, error) {
	modeRaw := ""
	if len(modeValues) > 0 {
		modeRaw = modeValues[0]
	}
	mode, err := xai.ParseOAuthMode(modeRaw)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadRequest, "GROK_OAUTH_INVALID_MODE", "%v", err)
	}
	state, err := xai.GenerateState()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_OAUTH_STATE_FAILED", "failed to generate state: %v", err)
	}
	nonce, err := xai.GenerateNonce()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_OAUTH_NONCE_FAILED", "failed to generate nonce: %v", err)
	}
	codeVerifier, err := xai.GenerateCodeVerifier()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_OAUTH_VERIFIER_FAILED", "failed to generate code verifier: %v", err)
	}
	sessionID, err := xai.GenerateSessionID()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_OAUTH_SESSION_FAILED", "failed to generate session ID: %v", err)
	}

	proxyURL, err := s.proxyURL(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	redirectURI = xai.EffectiveRedirectURI(redirectURI)
	codeChallenge := xai.GenerateCodeChallenge(codeVerifier)

	authURL, err := xai.BuildAuthorizationURLForMode(mode, state, codeChallenge, redirectURI, nonce)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadRequest, "GROK_OAUTH_INVALID_AUTHORIZE_URL", "%v", err)
	}

	if err := s.sessionStore.Set(ctx, sessionID, &xai.OAuthSession{
		Mode:          mode,
		State:         state,
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		ClientID:      xai.EffectiveClientID(),
		Scope:         xai.EffectiveScope(),
		ProxyURL:      proxyURL,
		RedirectURI:   redirectURI,
		CreatedAt:     time.Now(),
	}, xai.SessionTTL); err != nil {
		return nil, infraerrors.Newf(http.StatusServiceUnavailable, "GROK_OAUTH_SESSION_STORE_FAILED", "failed to persist oauth session: %v", err)
	}

	return &GrokAuthURLResult{
		AuthURL:   authURL,
		SessionID: sessionID,
		State:     state,
		OAuthMode: mode,
	}, nil
}

type GrokExchangeCodeInput struct {
	SessionID   string
	Code        string
	State       string
	RedirectURI string
	ProxyID     *int64
}

type GrokTokenInfo struct {
	AccessToken       string              `json:"access_token"`
	RefreshToken      string              `json:"refresh_token,omitempty"`
	IDToken           string              `json:"id_token,omitempty"`
	TokenType         string              `json:"token_type,omitempty"`
	ExpiresIn         int64               `json:"expires_in"`
	ExpiresAt         int64               `json:"expires_at"`
	ClientID          string              `json:"client_id,omitempty"`
	Scope             string              `json:"scope,omitempty"`
	Email             string              `json:"email,omitempty"`
	BaseURL           string              `json:"base_url,omitempty"`
	AuthKind          string              `json:"auth_kind,omitempty"`
	UsingAPI          bool                `json:"using_api"`
	SubscriptionTier  string              `json:"subscription_tier,omitempty"`
	EntitlementStatus string              `json:"entitlement_status,omitempty"`
	OAuthMode         xai.OAuthMode       `json:"oauth_mode"`
	TokenCapability   xai.TokenCapability `json:"token_capability"`
	TokenReferrer     string              `json:"token_referrer,omitempty"`
}

func (s *GrokOAuthService) ExchangeCode(ctx context.Context, input *GrokExchangeCodeInput) (*GrokTokenInfo, error) {
	if input == nil {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_INVALID_INPUT", "input is required")
	}
	session, ok, err := s.sessionStore.Consume(ctx, input.SessionID)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusServiceUnavailable, "GROK_OAUTH_SESSION_STORE_FAILED", "failed to consume oauth session: %v", err)
	}
	if !ok {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_SESSION_NOT_FOUND", "session not found or expired")
	}

	parsed := xai.ParseAuthorizationInput(input.Code)
	code := strings.TrimSpace(parsed.Code)
	if code == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_CODE_REQUIRED", "authorization code is required")
	}
	state := strings.TrimSpace(input.State)
	if state == "" {
		state = strings.TrimSpace(parsed.State)
	}
	if state == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_STATE_REQUIRED", "oauth state is required")
	}
	if subtle.ConstantTimeCompare([]byte(state), []byte(session.State)) != 1 {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_INVALID_STATE", "invalid oauth state")
	}

	// Proxy and redirect URI are frozen when the OAuth session is created.
	// Allowing the exchange request to override them weakens session binding and
	// makes authorization and token exchange use different network identities.
	proxyURL := session.ProxyURL
	redirectURI := session.RedirectURI

	tokenResp, err := s.oauthClient.ExchangeCode(ctx, code, session.CodeVerifier, redirectURI, proxyURL, session.ClientID)
	if err != nil {
		return nil, err
	}
	return s.tokenInfoFromResponse(tokenResp, session.ClientID, session.Mode, nil)
}

func (s *GrokOAuthService) RefreshToken(ctx context.Context, refreshToken, proxyURL, clientID string, modeValues ...xai.OAuthMode) (*GrokTokenInfo, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_NO_REFRESH_TOKEN", "refresh_token is required")
	}
	mode := xai.OAuthModeBuildProxy
	if len(modeValues) > 0 {
		mode = xai.NormalizeOAuthMode(string(modeValues[0]))
	}
	tokenResp, err := s.oauthClient.RefreshToken(ctx, refreshToken, proxyURL, clientID)
	if err != nil {
		return nil, err
	}
	tokenInfo, err := s.tokenInfoFromResponse(tokenResp, clientID, mode, nil)
	if err != nil {
		return nil, err
	}
	if tokenInfo.RefreshToken == "" {
		tokenInfo.RefreshToken = refreshToken
	}
	return tokenInfo, nil
}

func (s *GrokOAuthService) ValidateRefreshToken(ctx context.Context, refreshToken string, proxyID *int64, modeValues ...xai.OAuthMode) (*GrokTokenInfo, error) {
	proxyURL, err := s.proxyURL(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	return s.RefreshToken(ctx, refreshToken, proxyURL, xai.EffectiveClientID(), modeValues...)
}

func (s *GrokOAuthService) RefreshAccountToken(ctx context.Context, account *Account) (*GrokTokenInfo, error) {
	if account == nil || account.Platform != PlatformGrok {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_INVALID_ACCOUNT", "account is not a Grok account")
	}
	if account.Type != AccountTypeOAuth {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_INVALID_ACCOUNT_TYPE", "account is not an OAuth account")
	}

	proxyURL, err := s.proxyURL(ctx, account.ProxyID)
	if err != nil {
		return nil, err
	}
	refreshToken := account.GetCredential("refresh_token")
	if strings.TrimSpace(refreshToken) == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_NO_REFRESH_TOKEN", "no refresh token available")
	}

	clientID := account.GetCredential("client_id")
	mode := account.GrokOAuthMode()
	tokenInfo, err := s.RefreshToken(ctx, refreshToken, proxyURL, clientID, mode)
	if err != nil {
		return nil, err
	}
	if configuredBaseURL := strings.TrimSpace(account.GetCredential("base_url")); configuredBaseURL != "" {
		tokenInfo.BaseURL = configuredBaseURL
	}
	tokenInfo.AuthKind = firstNonEmpty(account.GetCredential("auth_kind"), "oauth")
	tokenInfo.UsingAPI = account.GrokUsingAPI()
	tokenInfo.SubscriptionTier = account.GetCredential("subscription_tier")
	tokenInfo.EntitlementStatus = account.GetCredential("entitlement_status")
	return tokenInfo, nil
}

func (s *GrokOAuthService) BuildAccountCredentials(tokenInfo *GrokTokenInfo) map[string]any {
	if tokenInfo == nil {
		return nil
	}
	expiresAt := time.Unix(tokenInfo.ExpiresAt, 0).UTC().Format(time.RFC3339)
	creds := map[string]any{
		"access_token":                    tokenInfo.AccessToken,
		"expires_at":                      expiresAt,
		"auth_kind":                       "oauth",
		"using_api":                       tokenInfo.UsingAPI,
		GrokCredentialOAuthMode:           string(tokenInfo.OAuthMode),
		GrokCredentialTokenCapability:     string(tokenInfo.TokenCapability),
		GrokCredentialTokenContextChecked: time.Now().UTC().Format(time.RFC3339),
	}
	if tokenInfo.RefreshToken != "" {
		creds["refresh_token"] = tokenInfo.RefreshToken
	}
	if tokenInfo.TokenType != "" {
		creds["token_type"] = tokenInfo.TokenType
	}
	if tokenInfo.IDToken != "" {
		creds["id_token"] = tokenInfo.IDToken
	}
	if tokenInfo.ClientID != "" {
		creds["client_id"] = tokenInfo.ClientID
	}
	if tokenInfo.Scope != "" {
		creds["scope"] = tokenInfo.Scope
	}
	if tokenInfo.Email != "" {
		creds["email"] = tokenInfo.Email
	}
	creds[GrokCredentialTokenReferrer] = tokenInfo.TokenReferrer
	creds[GrokCredentialReauthRequired] = tokenInfo.TokenCapability == xai.TokenCapabilityIncompatible
	if tokenInfo.SubscriptionTier != "" {
		creds["subscription_tier"] = tokenInfo.SubscriptionTier
	}
	if tokenInfo.EntitlementStatus != "" {
		creds["entitlement_status"] = tokenInfo.EntitlementStatus
	}
	baseURL := strings.TrimSpace(tokenInfo.BaseURL)
	if baseURL == "" {
		if tokenInfo.OAuthMode == xai.OAuthModeOfficialAPI {
			baseURL = xai.DefaultBaseURL
		} else {
			baseURL = xai.DefaultCLIBaseURL
		}
	}
	creds["base_url"] = baseURL
	return creds
}

func (s *GrokOAuthService) Stop() {
	if s != nil && s.sessionStore != nil {
		s.sessionStore.Stop()
	}
}

func (s *GrokOAuthService) tokenInfoFromResponse(tokenResp *xai.TokenResponse, clientID string, mode xai.OAuthMode, existing map[string]any) (*GrokTokenInfo, error) {
	now := time.Now()
	mode = xai.NormalizeOAuthMode(string(mode))
	if tokenResp == nil {
		tokenResp = &xai.TokenResponse{}
	}
	validation := xai.ValidateAccessTokenForMode(tokenResp.AccessToken, mode)
	if !validation.Compatible {
		return nil, infraerrors.Newf(
			http.StatusForbidden,
			"GROK_BUILD_TOKEN_CONTEXT_MISSING",
			"grok_build_token_context_missing: OAuth completed, but the issued access token is not compatible with Grok Build: %s; re-authorize using the Grok Build OAuth flow",
			validation.Reason,
		)
	}
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 && validation.Inspection.ExpiresAt > now.Unix() {
		expiresIn = validation.Inspection.ExpiresAt - now.Unix()
	}
	if expiresIn <= 0 {
		expiresIn = int64(grokDefaultAccessTokenTTL.Seconds())
	}
	baseURL := xai.DefaultCLIBaseURL
	if mode == xai.OAuthModeOfficialAPI {
		baseURL = xai.DefaultBaseURL
	}
	info := &GrokTokenInfo{
		AccessToken:     tokenResp.AccessToken,
		RefreshToken:    tokenResp.RefreshToken,
		IDToken:         tokenResp.IDToken,
		TokenType:       tokenResp.TokenType,
		ExpiresIn:       expiresIn,
		ExpiresAt:       now.Add(time.Duration(expiresIn) * time.Second).Unix(),
		ClientID:        strings.TrimSpace(clientID),
		Scope:           tokenResp.Scope,
		BaseURL:         baseURL,
		AuthKind:        "oauth",
		UsingAPI:        mode.UsingAPI(),
		OAuthMode:       mode,
		TokenCapability: validation.Capability,
		TokenReferrer:   validation.Inspection.Referrer,
	}
	if info.ClientID == "" {
		info.ClientID = xai.EffectiveClientID()
	}
	if info.TokenType == "" {
		info.TokenType = "Bearer"
	}
	if email := parseJWTEmailClaim(tokenResp.IDToken); email != "" {
		info.Email = email
	}
	if info.Email == "" {
		info.Email = validation.Inspection.Email
	}
	if info.Email == "" && existing != nil {
		if email, _ := existing["email"].(string); email != "" {
			info.Email = email
		}
	}
	return info, nil
}

func (s *GrokOAuthService) proxyURL(ctx context.Context, proxyID *int64) (string, error) {
	if proxyID == nil {
		return "", nil
	}
	if s.proxyRepo == nil {
		return "", infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_PROXY_NOT_AVAILABLE", "proxy repository is not available")
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadRequest, "GROK_OAUTH_PROXY_NOT_FOUND", "proxy not found: %v", err)
	}
	if proxy == nil {
		return "", nil
	}
	return proxy.URL(), nil
}

func parseJWTEmailClaim(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.TrimSpace(claims.Email)
}
