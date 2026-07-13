package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/xai"
	"github.com/WilliamWang1721/LightBridge/internal/service"
	"github.com/redis/go-redis/v9"
)

const grokOAuthSessionKeyPrefix = "oauth:grok:session:"

var consumeGrokOAuthSessionScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if value then
  redis.call("DEL", KEYS[1])
end
return value
`)

type grokOAuthSessionStore struct {
	redis *redis.Client
}

var _ service.GrokOAuthSessionStore = (*grokOAuthSessionStore)(nil)

func NewGrokOAuthSessionStore(redisClient *redis.Client) service.GrokOAuthSessionStore {
	return &grokOAuthSessionStore{redis: redisClient}
}

func (s *grokOAuthSessionStore) Set(ctx context.Context, sessionID string, session *xai.OAuthSession, ttl time.Duration) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("grok oauth session id is required")
	}
	if s == nil || s.redis == nil {
		return errors.New("grok oauth session store is unavailable")
	}
	if session == nil {
		return errors.New("grok oauth session is nil")
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal grok oauth session: %w", err)
	}
	if ttl <= 0 {
		ttl = xai.SessionTTL
	}
	return s.redis.Set(ctx, grokOAuthSessionKey(sessionID), payload, ttl).Err()
}

func (s *grokOAuthSessionStore) Consume(ctx context.Context, sessionID string) (*xai.OAuthSession, bool, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, false, errors.New("grok oauth session id is required")
	}
	if s == nil || s.redis == nil {
		return nil, false, errors.New("grok oauth session store is unavailable")
	}
	value, err := consumeGrokOAuthSessionScript.Run(ctx, s.redis, []string{grokOAuthSessionKey(sessionID)}).Text()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("consume grok oauth session: %w", err)
	}
	var session xai.OAuthSession
	if err := json.Unmarshal([]byte(value), &session); err != nil {
		return nil, false, fmt.Errorf("unmarshal grok oauth session: %w", err)
	}
	if session.CreatedAt.IsZero() || time.Since(session.CreatedAt) > xai.SessionTTL {
		return nil, false, nil
	}
	return &session, true, nil
}

// Stop is intentionally a no-op. The shared Redis client is owned by the
// application container and is closed by its own lifecycle manager.
func (s *grokOAuthSessionStore) Stop() {}

func grokOAuthSessionKey(sessionID string) string {
	return grokOAuthSessionKeyPrefix + sessionID
}
