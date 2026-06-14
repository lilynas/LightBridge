//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// authenticityRepoStub 嵌入完整的 accountRepoStub 以满足 AccountRepository 接口，
// 仅覆盖真伪检测路径用到的 GetByID 与 UpdateExtra（记录写入）。
type authenticityRepoStub struct {
	accountRepoStub
	account     *Account
	extraWrites []map[string]any
}

func (r *authenticityRepoStub) GetByID(_ context.Context, _ int64) (*Account, error) {
	return r.account, nil
}

func (r *authenticityRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	cp := make(map[string]any, len(updates))
	for k, v := range updates {
		cp[k] = v
	}
	r.extraWrites = append(r.extraWrites, cp)
	if r.account != nil {
		if r.account.Extra == nil {
			r.account.Extra = map[string]any{}
		}
		for k, v := range updates {
			r.account.Extra[k] = v
		}
	}
	return nil
}

func TestDetectThinkingSignatureError_MatchesRealAnthropicMessage(t *testing.T) {
	// 真 Anthropic 拒绝伪造签名时的典型错误体。
	realBody := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Invalid signature in thinking block"}}`)
	require.True(t, detectThinkingSignatureError(realBody), "真实 Anthropic 的 signature 错误必须被识别为真")

	// 套壳/中转假冒：返回正常响应体（无 signature 错误）。
	normalBody := []byte(`{"id":"msg_1","content":[{"type":"text","text":"hi"}]}`)
	require.False(t, detectThinkingSignatureError(normalBody))

	require.False(t, detectThinkingSignatureError([]byte(``)), "空响应体不应判为签名错误")
}

func TestParseAuthenticityPassthrough_DetectsSignatureDelta(t *testing.T) {
	svc := &GatewayService{}

	// content_block_start with type=thinking
	st := &thinkingSignatureState{enabled: true}
	svc.parseAuthenticityPassthrough(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`, st)
	require.True(t, st.sawThinkingBlock, "应识别到 thinking 块")
	require.False(t, st.sawSignature, "尚未出现 signature_delta")

	// content_block_delta with signature_delta + 非空 signature
	svc.parseAuthenticityPassthrough(`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"WcrN...signature-bytes"}}`, st)
	require.True(t, st.sawSignature, "应识别到合法 signature_delta")

	// 空 signature 不应触发
	st2 := &thinkingSignatureState{enabled: true}
	svc.parseAuthenticityPassthrough(`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"  "}}`, st2)
	require.False(t, st2.sawSignature, "空 signature 不算合法签名")

	// thinking 未开启时不应统计
	st3 := &thinkingSignatureState{enabled: false}
	svc.parseAuthenticityPassthrough(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`, st3)
	require.False(t, st3.sawThinkingBlock, "thinking 未开启时不统计")
}

func TestEvaluateAuthenticityPassive_GenuineClearsCounter(t *testing.T) {
	account := &Account{ID: 42, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Extra: map[string]any{AccountExtraKeyAuthenticitySuspicious: 2}} // 已累计 2 次可疑
	repo := &authenticityRepoStub{account: account}
	svc := &GatewayService{accountRepo: repo, settingService: newAuthenticityTestSettingService(true, 3)}

	// 检测到合法 signature → 判真，计数清零。
	svc.evaluateAuthenticityPassive(context.Background(), account, &thinkingSignatureState{enabled: true, sawSignature: true})

	require.Len(t, repo.extraWrites, 1, "应写一次 Extra")
	w := repo.extraWrites[0]
	require.Equal(t, AuthenticityVerdictGenuine, w[AccountExtraKeyAuthenticityVerdict])
	require.Equal(t, AuthenticityMethodPassive, w[AccountExtraKeyAuthenticityMethod])
	require.EqualValues(t, 0, w[AccountExtraKeyAuthenticitySuspicious], "确认真后应清零可疑计数")
}

func TestEvaluateAuthenticityPassive_SuspiciousIncrementsBeforeThreshold(t *testing.T) {
	account := &Account{ID: 43, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Extra: map[string]any{}}
	repo := &authenticityRepoStub{account: account}
	svc := &GatewayService{accountRepo: repo, settingService: newAuthenticityTestSettingService(true, 3)}

	// 开了 thinking 但无 signature → 可疑计数 +1（未到阈值 3）。
	svc.evaluateAuthenticityPassive(context.Background(), account, &thinkingSignatureState{enabled: true, sawThinkingBlock: true})

	require.Len(t, repo.extraWrites, 1)
	w := repo.extraWrites[0]
	require.EqualValues(t, 1, w[AccountExtraKeyAuthenticitySuspicious])
	_, hasVerdict := w[AccountExtraKeyAuthenticityVerdict]
	require.False(t, hasVerdict, "未到阈值不应写 verdict")
}

func TestEvaluateAuthenticityPassive_CounterfeitAtThreshold(t *testing.T) {
	account := &Account{ID: 44, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Extra: map[string]any{AccountExtraKeyAuthenticitySuspicious: 2}} // 已累计 2 次
	repo := &authenticityRepoStub{account: account}
	svc := &GatewayService{accountRepo: repo, settingService: newAuthenticityTestSettingService(true, 3)}

	// 第 3 次（达阈值）→ 判假冒。
	svc.evaluateAuthenticityPassive(context.Background(), account, &thinkingSignatureState{enabled: true, sawThinkingBlock: true})

	require.Len(t, repo.extraWrites, 1)
	w := repo.extraWrites[0]
	require.Equal(t, AuthenticityVerdictCounterfeit, w[AccountExtraKeyAuthenticityVerdict])
	require.Equal(t, AuthenticityMethodPassive, w[AccountExtraKeyAuthenticityMethod])
	require.EqualValues(t, 3, w[AccountExtraKeyAuthenticitySuspicious])
}

func TestEvaluateAuthenticityPassive_DisabledNoOp(t *testing.T) {
	account := &Account{ID: 45, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Extra: map[string]any{}}
	repo := &authenticityRepoStub{account: account}
	svc := &GatewayService{accountRepo: repo, settingService: newAuthenticityTestSettingService(false, 3)}

	// 总开关关闭：不判定、不写。
	svc.evaluateAuthenticityPassive(context.Background(), account, &thinkingSignatureState{enabled: true, sawThinkingBlock: true})
	require.Empty(t, repo.extraWrites, "总开关关闭时不应写 Extra")
}

func TestProbeAuthenticityResult_ExtraMapShape(t *testing.T) {
	now := time.Now()
	r := &ClaudeAuthenticityResult{
		Verdict: AuthenticityVerdictGenuine, Method: AuthenticityMethodProbe,
		CheckedAt: now, Detail: "ok",
	}
	m := r.ExtraMap()
	require.Equal(t, AuthenticityVerdictGenuine, m[AccountExtraKeyAuthenticityVerdict])
	require.Equal(t, AuthenticityMethodProbe, m[AccountExtraKeyAuthenticityMethod])
	require.Equal(t, "ok", m[AccountExtraKeyAuthenticityDetail])
	require.NotEmpty(t, m[AccountExtraKeyAuthenticityCheckedAt])

	require.Nil(t, (*ClaudeAuthenticityResult)(nil).ExtraMap(), "nil 结果应返回 nil")
}

// newAuthenticityTestSettingService 用现有 settingRepoStub 构造一个 SettingService，
// 其 GetAuthenticitySettings 返回预设配置（enabled + threshold）。
func newAuthenticityTestSettingService(enabled bool, threshold int) *SettingService {
	payload, _ := json.Marshal(AuthenticitySettings{Enabled: enabled, PassiveThreshold: threshold})
	return NewSettingService(&settingRepoStub{
		values: map[string]string{SettingKeyAuthenticitySettings: string(payload)},
	}, nil)
}
