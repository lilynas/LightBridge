package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/logger"
	"github.com/WilliamWang1721/LightBridge/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	grokReasoningReplayTTL           = 30 * time.Minute
	grokReasoningReplayMaxEntries    = 2048
	grokReasoningReplayEvictBatch    = 64
	grokReasoningReplayMaxItems      = 32
	grokReasoningReplayMaxEntryBytes = 2 << 20
)

// GrokReasoningReplayCache is an optional distributed cache extension provided
// by the production GatewayCache implementation. Keeping it separate from
// GatewayCache avoids widening the core sticky-session contract and preserves
// compatibility with existing test doubles and alternative cache adapters.
type GrokReasoningReplayCache interface {
	GetGrokReasoningReplay(ctx context.Context, key string) ([]byte, error)
	SetGrokReasoningReplay(ctx context.Context, key string, value []byte, ttl time.Duration) error
	DeleteGrokReasoningReplay(ctx context.Context, keys ...string) error
}

type grokReasoningReplayEntry struct {
	items     []json.RawMessage
	expiresAt time.Time
	updatedAt time.Time
}

type grokReasoningReplayStore struct {
	remote GrokReasoningReplayCache
	mu     sync.Mutex
	items  map[string]grokReasoningReplayEntry
}

type grokReasoningReplayRecord struct {
	Items []json.RawMessage `json:"items"`
}

type grokReasoningReplayScope struct {
	tenant             string
	model              string
	sessionID          string
	previousResponseID string
}

func newGrokReasoningReplayStore(remote GrokReasoningReplayCache) *grokReasoningReplayStore {
	return &grokReasoningReplayStore{
		remote: remote,
		items:  make(map[string]grokReasoningReplayEntry),
	}
}

func (s *OpenAIGatewayService) getGrokReasoningReplayStore() *grokReasoningReplayStore {
	if s == nil {
		return nil
	}
	s.grokReplayOnce.Do(func() {
		var remote GrokReasoningReplayCache
		if candidate, ok := s.cache.(GrokReasoningReplayCache); ok {
			remote = candidate
		}
		s.grokReplayStore = newGrokReasoningReplayStore(remote)
	})
	return s.grokReplayStore
}

func (s *grokReasoningReplayStore) get(ctx context.Context, key string) ([]json.RawMessage, bool) {
	key = strings.TrimSpace(key)
	if s == nil || key == "" {
		return nil, false
	}
	now := time.Now()
	s.mu.Lock()
	if entry, ok := s.items[key]; ok {
		if now.Before(entry.expiresAt) {
			entry.updatedAt = now
			entry.expiresAt = now.Add(grokReasoningReplayTTL)
			s.items[key] = entry
			items := cloneGrokReplayItems(entry.items)
			s.mu.Unlock()
			return items, true
		}
		delete(s.items, key)
	}
	s.mu.Unlock()

	if s.remote == nil {
		return nil, false
	}
	raw, err := s.remote.GetGrokReasoningReplay(detachedGrokReplayContext(ctx), key)
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	var record grokReasoningReplayRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		_ = s.remote.DeleteGrokReasoningReplay(detachedGrokReplayContext(ctx), key)
		return nil, false
	}
	normalized, ok := normalizeGrokReplayItems(record.Items)
	if !ok {
		_ = s.remote.DeleteGrokReasoningReplay(detachedGrokReplayContext(ctx), key)
		return nil, false
	}
	s.putLocal(key, normalized, now)
	return cloneGrokReplayItems(normalized), true
}

func (s *grokReasoningReplayStore) set(ctx context.Context, keys []string, items []json.RawMessage) bool {
	if s == nil {
		return false
	}
	normalized, ok := normalizeGrokReplayItems(items)
	if !ok {
		return false
	}
	uniqueKeys := uniqueNonEmptyStrings(keys)
	if len(uniqueKeys) == 0 {
		return false
	}
	now := time.Now()
	for _, key := range uniqueKeys {
		s.putLocal(key, normalized, now)
	}
	if s.remote == nil {
		return true
	}
	raw, err := json.Marshal(grokReasoningReplayRecord{Items: normalized})
	if err != nil {
		return true
	}
	cacheCtx := detachedGrokReplayContext(ctx)
	for _, key := range uniqueKeys {
		if err := s.remote.SetGrokReasoningReplay(cacheCtx, key, raw, grokReasoningReplayTTL); err != nil {
			logger.LegacyPrintf("service.grok_reasoning_replay", "distributed replay cache set failed: %v", err)
		}
	}
	return true
}

func (s *grokReasoningReplayStore) delete(ctx context.Context, keys ...string) {
	if s == nil {
		return
	}
	keys = uniqueNonEmptyStrings(keys)
	if len(keys) == 0 {
		return
	}
	s.mu.Lock()
	for _, key := range keys {
		delete(s.items, key)
	}
	s.mu.Unlock()
	if s.remote != nil {
		_ = s.remote.DeleteGrokReasoningReplay(detachedGrokReplayContext(ctx), keys...)
	}
}

func (s *grokReasoningReplayStore) putLocal(key string, items []json.RawMessage, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = grokReasoningReplayEntry{
		items:     cloneGrokReplayItems(items),
		expiresAt: now.Add(grokReasoningReplayTTL),
		updatedAt: now,
	}
	if len(s.items) <= grokReasoningReplayMaxEntries {
		return
	}
	for count := 0; count < grokReasoningReplayEvictBatch && len(s.items) > 0; count++ {
		oldestKey := ""
		var oldest time.Time
		for candidate, entry := range s.items {
			if oldestKey == "" || entry.updatedAt.Before(oldest) {
				oldestKey = candidate
				oldest = entry.updatedAt
			}
		}
		if oldestKey == "" {
			break
		}
		delete(s.items, oldestKey)
	}
}

func detachedGrokReplayContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func resolveGrokReasoningReplayScope(c *gin.Context, body []byte, model string) grokReasoningReplayScope {
	tenant := ""
	if apiKey := getAPIKeyFromContext(c); apiKey != nil && apiKey.ID > 0 {
		groupID := int64(0)
		if apiKey.GroupID != nil {
			groupID = *apiKey.GroupID
		}
		tenant = fmt.Sprintf("apikey:%d:group:%d", apiKey.ID, groupID)
	}
	return grokReasoningReplayScope{
		tenant:             tenant,
		model:              strings.TrimSpace(model),
		sessionID:          resolveGrokConversationID(c, body, model),
		previousResponseID: strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String()),
	}
}

func (s grokReasoningReplayScope) cacheKey(kind, value string) string {
	if strings.TrimSpace(s.tenant) == "" || strings.TrimSpace(s.model) == "" || strings.TrimSpace(value) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{"v1", s.tenant, s.model, kind, strings.TrimSpace(value)}, "\x00")))
	return "grok_reasoning_replay:v1:" + hex.EncodeToString(sum[:])
}

func (s grokReasoningReplayScope) lookupKeys() []string {
	return uniqueNonEmptyStrings([]string{
		s.cacheKey("response", s.previousResponseID),
		s.cacheKey("session", s.sessionID),
	})
}

func (s grokReasoningReplayScope) storageKeys(responseID string) []string {
	return uniqueNonEmptyStrings([]string{
		s.cacheKey("response", responseID),
		s.cacheKey("session", s.sessionID),
	})
}

func (s *OpenAIGatewayService) prepareGrokReasoningReplayRequest(ctx context.Context, c *gin.Context, body []byte, model string) ([]byte, grokReasoningReplayScope, bool) {
	scope := resolveGrokReasoningReplayScope(c, body, model)
	updated := body
	injected := false
	store := s.getGrokReasoningReplayStore()
	if store != nil {
		for _, key := range scope.lookupKeys() {
			items, ok := store.get(ctx, key)
			if !ok {
				continue
			}
			filtered := filterGrokReplayItemsForInput(updated, items)
			if len(filtered) > 0 {
				if next, changed := insertGrokReplayItems(updated, filtered); changed {
					updated = next
					injected = true
				}
			}
			break
		}
	}
	// Grok Build is treated as a stateless upstream. Downstream clients may use
	// previous_response_id, but Build cannot be relied on to retain the response
	// across expiry, accounts, or LightBridge instances. The replay items above
	// carry the actual continuation state instead.
	if gjson.GetBytes(updated, "previous_response_id").Exists() {
		if next, err := sjson.DeleteBytes(updated, "previous_response_id"); err == nil {
			updated = next
		}
	}
	return updated, scope, injected
}

func (s *OpenAIGatewayService) cacheGrokReasoningReplay(ctx context.Context, scope grokReasoningReplayScope, completed []byte) {
	if strings.TrimSpace(scope.tenant) == "" || len(completed) == 0 {
		return
	}
	response := gjson.GetBytes(completed, "response")
	if !response.Exists() || !response.IsObject() {
		response = gjson.ParseBytes(completed)
	}
	responseID := strings.TrimSpace(response.Get("id").String())
	output := response.Get("output")
	if !output.IsArray() {
		return
	}
	items := make([]json.RawMessage, 0, len(output.Array()))
	for _, item := range output.Array() {
		switch strings.TrimSpace(item.Get("type").String()) {
		case "reasoning":
			encrypted := item.Get("encrypted_content")
			if encrypted.Type != gjson.String || !xai.IsValidGrokEncryptedContent(encrypted.String()) {
				continue
			}
			items = append(items, json.RawMessage(item.Raw))
		case "function_call", "custom_tool_call":
			if strings.TrimSpace(item.Get("call_id").String()) == "" {
				continue
			}
			items = append(items, json.RawMessage(item.Raw))
		}
	}
	keys := scope.storageKeys(responseID)
	store := s.getGrokReasoningReplayStore()
	if store == nil {
		return
	}
	if len(items) == 0 || !store.set(ctx, keys, items) {
		store.delete(ctx, keys...)
	}
}

func (s *OpenAIGatewayService) clearGrokReasoningReplay(ctx context.Context, scope grokReasoningReplayScope) {
	if store := s.getGrokReasoningReplayStore(); store != nil {
		store.delete(ctx, scope.lookupKeys()...)
	}
}

func normalizeGrokReplayItems(items []json.RawMessage) ([]json.RawMessage, bool) {
	if len(items) == 0 {
		return nil, false
	}
	capacity := len(items)
	if capacity > grokReasoningReplayMaxItems {
		capacity = grokReasoningReplayMaxItems
	}
	normalized := make([]json.RawMessage, 0, capacity)
	seen := make(map[string]struct{})
	totalBytes := 0
	for _, rawItem := range items {
		if len(normalized) >= grokReasoningReplayMaxItems {
			break
		}
		raw := []byte(rawItem)
		if !gjson.ValidBytes(raw) {
			continue
		}
		item := gjson.ParseBytes(raw)
		typ := strings.TrimSpace(item.Get("type").String())
		key := ""
		switch typ {
		case "reasoning":
			encrypted := item.Get("encrypted_content")
			if encrypted.Type != gjson.String || !xai.IsValidGrokEncryptedContent(encrypted.String()) {
				continue
			}
			key = "reasoning:" + encrypted.String()
		case "function_call", "custom_tool_call":
			callID := strings.TrimSpace(item.Get("call_id").String())
			if callID == "" {
				continue
			}
			key = typ + ":" + callID
		default:
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		if len(raw) > grokReasoningReplayMaxEntryBytes-totalBytes {
			break
		}
		seen[key] = struct{}{}
		copyRaw := append(json.RawMessage(nil), raw...)
		normalized = append(normalized, copyRaw)
		totalBytes += len(copyRaw)
	}
	return normalized, len(normalized) > 0
}

func filterGrokReplayItemsForInput(body []byte, items []json.RawMessage) []json.RawMessage {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return nil
	}
	hasValidReasoning := false
	existingCalls := make(map[string]struct{})
	outputs := make(map[string]string)
	for _, item := range input.Array() {
		typ := strings.TrimSpace(item.Get("type").String())
		switch typ {
		case "reasoning":
			if encrypted := item.Get("encrypted_content"); encrypted.Type == gjson.String && xai.IsValidGrokEncryptedContent(encrypted.String()) {
				hasValidReasoning = true
			}
		case "function_call", "custom_tool_call":
			if callID := strings.TrimSpace(item.Get("call_id").String()); callID != "" {
				existingCalls[typ+":"+callID] = struct{}{}
			}
		case "function_call_output", "custom_tool_call_output":
			if callID := strings.TrimSpace(item.Get("call_id").String()); callID != "" {
				outputs[callID] = callID
			}
		}
	}
	filtered := make([]json.RawMessage, 0, len(items))
	for _, raw := range items {
		item := gjson.ParseBytes(raw)
		typ := strings.TrimSpace(item.Get("type").String())
		switch typ {
		case "reasoning":
			if hasValidReasoning {
				continue
			}
		case "function_call", "custom_tool_call":
			callID := strings.TrimSpace(item.Get("call_id").String())
			if callID == "" {
				continue
			}
			if _, exists := existingCalls[typ+":"+callID]; exists {
				continue
			}
			if _, exists := outputs[callID]; !exists {
				continue
			}
			existingCalls[typ+":"+callID] = struct{}{}
		default:
			continue
		}
		filtered = append(filtered, append(json.RawMessage(nil), raw...))
	}
	return filtered
}

func insertGrokReplayItems(body []byte, replayItems []json.RawMessage) ([]byte, bool) {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() || len(replayItems) == 0 {
		return body, false
	}
	inputItems := input.Array()
	insertIndex := len(inputItems)
	replayCallIDs := make(map[string]struct{})
	for _, raw := range replayItems {
		item := gjson.ParseBytes(raw)
		if typ := strings.TrimSpace(item.Get("type").String()); typ == "function_call" || typ == "custom_tool_call" {
			if callID := strings.TrimSpace(item.Get("call_id").String()); callID != "" {
				replayCallIDs[callID] = struct{}{}
			}
		}
	}
	for index, item := range inputItems {
		typ := strings.TrimSpace(item.Get("type").String())
		if typ != "function_call_output" && typ != "custom_tool_call_output" {
			continue
		}
		callID := strings.TrimSpace(item.Get("call_id").String())
		if callID == "" {
			insertIndex = index
			break
		}
		if _, ok := replayCallIDs[callID]; ok {
			insertIndex = index
			break
		}
	}
	combined := make([]json.RawMessage, 0, len(inputItems)+len(replayItems))
	for index, item := range inputItems {
		if index == insertIndex {
			combined = append(combined, cloneGrokReplayItems(replayItems)...)
		}
		combined = append(combined, json.RawMessage(item.Raw))
	}
	if insertIndex == len(inputItems) {
		combined = append(combined, cloneGrokReplayItems(replayItems)...)
	}
	encoded, err := json.Marshal(combined)
	if err != nil {
		return body, false
	}
	updated, err := sjson.SetRawBytes(body, "input", encoded)
	if err != nil {
		return body, false
	}
	return updated, true
}

func cloneGrokReplayItems(items []json.RawMessage) []json.RawMessage {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, append(json.RawMessage(nil), item...))
	}
	return cloned
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
