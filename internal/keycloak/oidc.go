// Package keycloak implements the OpenID Connect parts used by this service.
// It deliberately uses the discovery document and JWKS published by Keycloak rather
// than accepting an unverified user identity from the browser.
package keycloak

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yang/wormhole_backend/internal/config"
)

const (
	metadataCacheTTL = time.Hour
	jwksCacheTTL     = 10 * time.Minute
	maxResponseBytes = 1 << 20 // 1 MiB
)

// Identity 是经过签名、issuer、audience、nonce 和时间声明验证后的 OIDC 身份信息。
type Identity struct {
	Subject           string
	PreferredUsername string
	Email             string
	Name              string
}

// Client 负责与 Keycloak 的 OIDC discovery、authorization、token 和 JWKS 端点交互。
type Client struct {
	issuerURL    string
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       []string
	httpClient   *http.Client

	metadataMu        sync.Mutex
	metadata          providerMetadata
	metadataExpiresAt time.Time

	jwksMu        sync.Mutex
	jwksURI       string
	keys          []jsonWebKey
	jwksExpiresAt time.Time
}

type providerMetadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	IDToken     string `json:"id_token"`
}

type idTokenClaims struct {
	jwt.RegisteredClaims
	Nonce             string `json:"nonce"`
	AuthorizedParty   string `json:"azp"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
}

type jsonWebKeySet struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// NewClient 从应用配置构造 Keycloak OIDC 客户端。该函数不访问网络；discovery
// 在首次登录时执行，因此暂时不可用的 Keycloak 不会阻止进程启动。
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	if err := cfg.ValidateKeycloak(); err != nil {
		return nil, err
	}
	return &Client{
		issuerURL:    strings.TrimRight(cfg.KeycloakIssuerURL, "/"),
		clientID:     cfg.KeycloakClientID,
		clientSecret: cfg.KeycloakClientSecret,
		redirectURL:  cfg.KeycloakRedirectURL,
		scopes:       append([]string(nil), cfg.KeycloakScopes...),
		httpClient: &http.Client{
			Timeout: cfg.KeycloakHTTPTimeout,
		},
	}, nil
}

// AuthorizationURL 创建浏览器应跳转到的 Keycloak 授权地址。
func (c *Client) AuthorizationURL(ctx context.Context, state, nonce, verifier string) (string, error) {
	if state == "" || nonce == "" || verifier == "" {
		return "", errors.New("OIDC state, nonce and PKCE verifier are required")
	}
	metadata, err := c.discovery(ctx)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"client_id":             []string{c.clientID},
		"redirect_uri":          []string{c.redirectURL},
		"response_type":         []string{"code"},
		"scope":                 []string{strings.Join(c.scopes, " ")},
		"state":                 []string{state},
		"nonce":                 []string{nonce},
		"code_challenge":        []string{base64.RawURLEncoding.EncodeToString(hash[:])},
		"code_challenge_method": []string{"S256"},
	}
	return metadata.AuthorizationEndpoint + "?" + query.Encode(), nil
}

// ExchangeAndVerify 用授权码换取 token，并严格验证返回的 ID token。
func (c *Client) ExchangeAndVerify(ctx context.Context, code, verifier, expectedNonce string) (Identity, error) {
	if code == "" || verifier == "" || expectedNonce == "" {
		return Identity{}, errors.New("authorization code, verifier and nonce are required")
	}
	metadata, err := c.discovery(ctx)
	if err != nil {
		return Identity{}, err
	}

	form := url.Values{
		"grant_type":    []string{"authorization_code"},
		"code":          []string{code},
		"redirect_uri":  []string{c.redirectURL},
		"code_verifier": []string{verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Identity{}, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	// BFF 使用 confidential client：client secret 只在服务器与 Keycloak 的后信道中使用。
	req.SetBasicAuth(c.clientID, c.clientSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Identity{}, fmt.Errorf("call Keycloak token endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
		return Identity{}, fmt.Errorf("Keycloak token endpoint returned HTTP %d", resp.StatusCode)
	}

	var token tokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&token); err != nil {
		return Identity{}, fmt.Errorf("decode Keycloak token response: %w", err)
	}
	if token.IDToken == "" {
		return Identity{}, errors.New("Keycloak token response does not contain id_token")
	}
	return c.verifyIDToken(ctx, metadata, token.IDToken, expectedNonce)
}

func (c *Client) discovery(ctx context.Context) (providerMetadata, error) {
	c.metadataMu.Lock()
	defer c.metadataMu.Unlock()

	if !c.metadataExpiresAt.IsZero() && time.Now().Before(c.metadataExpiresAt) {
		return c.metadata, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.issuerURL+"/.well-known/openid-configuration", nil)
	if err != nil {
		return providerMetadata{}, fmt.Errorf("create OIDC discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return providerMetadata{}, fmt.Errorf("discover Keycloak OIDC endpoints: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
		return providerMetadata{}, fmt.Errorf("Keycloak discovery endpoint returned HTTP %d", resp.StatusCode)
	}

	var metadata providerMetadata
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&metadata); err != nil {
		return providerMetadata{}, fmt.Errorf("decode Keycloak discovery document: %w", err)
	}
	if strings.TrimRight(metadata.Issuer, "/") != c.issuerURL {
		return providerMetadata{}, fmt.Errorf("Keycloak discovery issuer mismatch: got %q", metadata.Issuer)
	}
	for name, endpoint := range map[string]string{
		"authorization_endpoint": metadata.AuthorizationEndpoint,
		"token_endpoint":         metadata.TokenEndpoint,
		"jwks_uri":               metadata.JWKSURI,
	} {
		if !isAbsoluteHTTPURL(endpoint) {
			return providerMetadata{}, fmt.Errorf("invalid Keycloak %s", name)
		}
	}

	c.metadata = metadata
	c.metadataExpiresAt = time.Now().Add(metadataCacheTTL)
	return metadata, nil
}

func (c *Client) verifyIDToken(ctx context.Context, metadata providerMetadata, rawToken, expectedNonce string) (Identity, error) {
	keys, err := c.keySet(ctx, metadata.JWKSURI, false)
	if err != nil {
		return Identity{}, err
	}
	identity, err := c.parseIDToken(rawToken, metadata, keys, expectedNonce)
	if err == nil {
		return identity, nil
	}

	// Keycloak 轮换签名密钥时，缓存中的 kid 可能已过期。刷新一次再验证；
	// 如果仍失败，保留最终验证错误而不是接受未经验证的 token。
	freshKeys, refreshErr := c.keySet(ctx, metadata.JWKSURI, true)
	if refreshErr != nil {
		return Identity{}, err
	}
	return c.parseIDToken(rawToken, metadata, freshKeys, expectedNonce)
}

func (c *Client) parseIDToken(rawToken string, metadata providerMetadata, keys []jsonWebKey, expectedNonce string) (Identity, error) {
	claims := &idTokenClaims{}
	_, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		keyFunc(keys),
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512", "EdDSA"}),
		jwt.WithIssuer(metadata.Issuer),
		jwt.WithAudience(c.clientID),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(time.Minute),
	)
	if err != nil {
		return Identity{}, fmt.Errorf("verify Keycloak ID token: %w", err)
	}
	if claims.Subject == "" {
		return Identity{}, errors.New("Keycloak ID token does not contain sub")
	}
	if claims.Nonce != expectedNonce {
		return Identity{}, errors.New("Keycloak ID token nonce mismatch")
	}
	if len(claims.Audience) > 1 && claims.AuthorizedParty != c.clientID {
		return Identity{}, errors.New("Keycloak ID token azp does not match client")
	}

	return Identity{
		Subject:           claims.Subject,
		PreferredUsername: strings.TrimSpace(claims.PreferredUsername),
		Email:             strings.TrimSpace(claims.Email),
		Name:              strings.TrimSpace(claims.Name),
	}, nil
}

func (c *Client) keySet(ctx context.Context, jwksURI string, forceRefresh bool) ([]jsonWebKey, error) {
	c.jwksMu.Lock()
	defer c.jwksMu.Unlock()

	if !forceRefresh && c.jwksURI == jwksURI && !c.jwksExpiresAt.IsZero() && time.Now().Before(c.jwksExpiresAt) {
		return append([]jsonWebKey(nil), c.keys...), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("create Keycloak JWKS request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Keycloak JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
		return nil, fmt.Errorf("Keycloak JWKS endpoint returned HTTP %d", resp.StatusCode)
	}

	var set jsonWebKeySet
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&set); err != nil {
		return nil, fmt.Errorf("decode Keycloak JWKS: %w", err)
	}
	if len(set.Keys) == 0 {
		return nil, errors.New("Keycloak JWKS contains no keys")
	}

	c.jwksURI = jwksURI
	c.keys = append([]jsonWebKey(nil), set.Keys...)
	c.jwksExpiresAt = time.Now().Add(jwksCacheTTL)
	return append([]jsonWebKey(nil), set.Keys...), nil
}

func keyFunc(keys []jsonWebKey) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		alg, _ := token.Header["alg"].(string)
		if kid == "" || alg == "" {
			return nil, errors.New("ID token header lacks kid or alg")
		}
		for _, key := range keys {
			if key.Kid != kid {
				continue
			}
			if key.Use != "" && key.Use != "sig" {
				return nil, errors.New("JWK is not a signing key")
			}
			if key.Alg != "" && key.Alg != alg {
				return nil, errors.New("ID token algorithm does not match JWK")
			}
			return publicKeyFromJWK(key, alg)
		}
		return nil, fmt.Errorf("no Keycloak JWK for kid %q", kid)
	}
}

func publicKeyFromJWK(key jsonWebKey, algorithm string) (any, error) {
	switch key.Kty {
	case "RSA":
		if !strings.HasPrefix(algorithm, "RS") && !strings.HasPrefix(algorithm, "PS") {
			return nil, errors.New("RSA JWK used with non-RSA algorithm")
		}
		n, err := decodeBase64URL(key.N)
		if err != nil {
			return nil, fmt.Errorf("decode RSA modulus: %w", err)
		}
		e, err := decodeBase64URL(key.E)
		if err != nil {
			return nil, fmt.Errorf("decode RSA exponent: %w", err)
		}
		exponent := new(big.Int).SetBytes(e)
		if !exponent.IsInt64() || exponent.Int64() <= 1 || exponent.Int64() > int64(^uint(0)>>1) {
			return nil, errors.New("invalid RSA exponent")
		}
		modulus := new(big.Int).SetBytes(n)
		if modulus.Sign() <= 0 {
			return nil, errors.New("invalid RSA modulus")
		}
		return &rsa.PublicKey{N: modulus, E: int(exponent.Int64())}, nil

	case "EC":
		if !strings.HasPrefix(algorithm, "ES") {
			return nil, errors.New("EC JWK used with non-ECDSA algorithm")
		}
		curve, err := curveFromName(key.Crv)
		if err != nil {
			return nil, err
		}
		x, err := decodeBase64URL(key.X)
		if err != nil {
			return nil, fmt.Errorf("decode EC x coordinate: %w", err)
		}
		y, err := decodeBase64URL(key.Y)
		if err != nil {
			return nil, fmt.Errorf("decode EC y coordinate: %w", err)
		}
		publicKey := &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
		if !curve.IsOnCurve(publicKey.X, publicKey.Y) {
			return nil, errors.New("EC JWK point is not on curve")
		}
		return publicKey, nil

	case "OKP":
		if key.Crv != "Ed25519" || algorithm != "EdDSA" {
			return nil, errors.New("unsupported OKP JWK or algorithm")
		}
		x, err := decodeBase64URL(key.X)
		if err != nil {
			return nil, fmt.Errorf("decode Ed25519 public key: %w", err)
		}
		if len(x) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 public key length")
		}
		return ed25519.PublicKey(x), nil

	default:
		return nil, fmt.Errorf("unsupported JWK key type %q", key.Kty)
	}
}

func curveFromName(name string) (elliptic.Curve, error) {
	switch name {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported EC curve %q", name)
	}
}

func decodeBase64URL(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return base64.URLEncoding.DecodeString(value)
}

func isAbsoluteHTTPURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}
