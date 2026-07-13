package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/WilliamWang1721/LightBridge/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	stickySessionPrefix       = "sticky_session:"
	grokReasoningReplayPrefix = "grok_reasoning_replay:v1:"
)

type gatewayCache struct {
	rdb *redis.Client
}

func NewGatewayCache(rdb *redis.Client) service.GatewayCache {
	return &gatewayCache{rdb: rdb}
}

// buildSessionKey 构建 session key，包含 groupID 实现分组隔离
// 格式: sticky_session:{groupID}:{sessionHash}
func buildSessionKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%s%d:%s", stickySessionPrefix, groupID, sessionHash)
}

func (c *gatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Get(ctx, key).Int64()
}

func (c *gatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Set(ctx, key, accountID, ttl).Err()
}

func (c *gatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// DeleteSessionAccountID 删除粘性会话与账号的绑定关系。
// 当检测到绑定的账号不可用（如状态错误、禁用、不可调度等）时调用，
// 以便下次请求能够重新选择可用账号。
//
// DeleteSessionAccountID removes the sticky session binding for the given session.
// Called when the bound account becomes unavailable (e.g., error status, disabled,
// or unschedulable), allowing subsequent requests to select a new available account.
func (c *gatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Del(ctx, key).Err()
}

func buildGrokReasoningReplayKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, grokReasoningReplayPrefix)
	return grokReasoningReplayPrefix + key
}

func (c *gatewayCache) GetGrokReasoningReplay(ctx context.Context, key string) ([]byte, error) {
	value, err := c.rdb.Get(ctx, buildGrokReasoningReplayKey(key)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return value, err
}

func (c *gatewayCache) SetGrokReasoningReplay(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, buildGrokReasoningReplayKey(key), value, ttl).Err()
}

func (c *gatewayCache) DeleteGrokReasoningReplay(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	redisKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		redisKeys = append(redisKeys, buildGrokReasoningReplayKey(key))
	}
	if len(redisKeys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, redisKeys...).Err()
}
