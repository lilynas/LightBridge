package service

import (
	"context"
	"errors"
)

// ClaudeTokenCache token cache interface.
type ClaudeTokenCache = GeminiTokenCache

// ClaudeTokenProvider manages access_token for Anthropic Vertex service accounts.
// Anthropic OAuth is handled by installable provider modules.
type ClaudeTokenProvider struct {
	accountRepo AccountRepository
	tokenCache  ClaudeTokenCache
}

func NewClaudeTokenProvider(
	accountRepo AccountRepository,
	tokenCache ClaudeTokenCache,
) *ClaudeTokenProvider {
	return &ClaudeTokenProvider{
		accountRepo: accountRepo,
		tokenCache:  tokenCache,
	}
}

// SetRefreshAPI injects unified OAuth refresh API and executor.
func (p *ClaudeTokenProvider) SetRefreshAPI(api *OAuthRefreshAPI, executor OAuthRefreshExecutor) {
}

// SetRefreshPolicy injects caller-side refresh policy.
func (p *ClaudeTokenProvider) SetRefreshPolicy(policy ProviderRefreshPolicy) {
}

// GetAccessToken returns a valid access_token.
func (p *ClaudeTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformAnthropic || account.Type != AccountTypeServiceAccount {
		return "", errors.New("not an anthropic service account")
	}
	return p.getServiceAccountAccessToken(ctx, account)
}

func (p *ClaudeTokenProvider) getServiceAccountAccessToken(ctx context.Context, account *Account) (string, error) {
	return getVertexServiceAccountAccessToken(ctx, p.tokenCache, account)
}
