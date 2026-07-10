package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// OIDCLoginState 记录一次浏览器登录往返所需的短期数据。
// 它会被 HMAC 签名后保存在 HttpOnly Cookie 中，因此服务端无需依赖单机内存保存
// state/nonce/PKCE verifier，也可部署多个实例。
type OIDCLoginState struct {
	State     string `json:"state"`
	Nonce     string `json:"nonce"`
	Verifier  string `json:"verifier"`
	ReturnTo  string `json:"return_to"`
	ExpiresAt int64  `json:"expires_at"`
}

// OIDCStateManager 负责生成并验证一次性 OIDC 登录状态。
type OIDCStateManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewOIDCStateManager 构造状态管理器。secret 应来自独立的高熵环境变量。
func NewOIDCStateManager(secret string, ttl time.Duration) (*OIDCStateManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("OIDC state secret must be at least 32 bytes")
	}
	if ttl <= 0 {
		return nil, errors.New("OIDC state ttl must be positive")
	}
	return &OIDCStateManager{
		secret: []byte(secret),
		ttl:    ttl,
		now:    time.Now,
	}, nil
}

// NewLoginState 创建包含 CSRF state、OIDC nonce 和 PKCE verifier 的登录状态。
func (m *OIDCStateManager) NewLoginState(returnTo string) (OIDCLoginState, error) {
	state, err := randomURLSafeString(32)
	if err != nil {
		return OIDCLoginState{}, fmt.Errorf("generate state: %w", err)
	}
	nonce, err := randomURLSafeString(32)
	if err != nil {
		return OIDCLoginState{}, fmt.Errorf("generate nonce: %w", err)
	}
	// RFC 7636 要求 code_verifier 介于 43 到 128 个字符。32 个随机字节
	// 经 base64url 编码恰好得到 43 个字符。
	verifier, err := randomURLSafeString(32)
	if err != nil {
		return OIDCLoginState{}, fmt.Errorf("generate verifier: %w", err)
	}

	return OIDCLoginState{
		State:     state,
		Nonce:     nonce,
		Verifier:  verifier,
		ReturnTo:  returnTo,
		ExpiresAt: m.now().Add(m.ttl).Unix(),
	}, nil
}

// Encode 对状态进行 JSON 编码并追加 HMAC-SHA-256 签名。
func (m *OIDCStateManager) Encode(state OIDCLoginState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal OIDC state: %w", err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(encodedPayload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedPayload + "." + signature, nil
}

// Decode 验证 Cookie 中的签名和有效期，并返回登录状态。
func (m *OIDCStateManager) Decode(value string) (OIDCLoginState, error) {
	payload, signature, ok := strings.Cut(value, ".")
	if !ok || payload == "" || signature == "" {
		return OIDCLoginState{}, errors.New("malformed OIDC state cookie")
	}

	received, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return OIDCLoginState{}, errors.New("invalid OIDC state signature")
	}
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	if !hmac.Equal(received, mac.Sum(nil)) {
		return OIDCLoginState{}, errors.New("OIDC state cookie signature mismatch")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return OIDCLoginState{}, errors.New("invalid OIDC state payload")
	}
	var state OIDCLoginState
	if err := json.Unmarshal(decoded, &state); err != nil {
		return OIDCLoginState{}, errors.New("invalid OIDC state JSON")
	}
	if state.State == "" || state.Nonce == "" || state.Verifier == "" || state.ExpiresAt == 0 {
		return OIDCLoginState{}, errors.New("incomplete OIDC state")
	}
	if m.now().Unix() > state.ExpiresAt {
		return OIDCLoginState{}, errors.New("OIDC state expired")
	}
	return state, nil
}

func randomURLSafeString(bytesLen int) (string, error) {
	bytes := make([]byte, bytesLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
