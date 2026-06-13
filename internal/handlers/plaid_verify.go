package handlers

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// plaidJWTClaims holds the standard JWT claims plus Plaid's body-hash claim.
type plaidJWTClaims struct {
	RequestBodySHA256 string `json:"request_body_sha256"`
	jwt.RegisteredClaims
}

type plaidJWK struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type plaidJWKSet struct {
	Keys []plaidJWK `json:"keys"`
}

// plaidKeyCache caches Plaid public keys by kid to avoid a JWKS fetch on
// every webhook. TTL is 5 minutes; the cache is rebuilt on expiry or miss.
var (
	plaidKeyCache    = make(map[string]*rsa.PublicKey)
	plaidKeyCacheMu  sync.RWMutex
	plaidKeyCacheAt  time.Time
	plaidKeyCacheTTL = 5 * time.Minute

	plaidHTTPClient = &http.Client{Timeout: 10 * time.Second}
)

func plaidJWKSURL(environment string) string {
	switch strings.ToLower(environment) {
	case "production":
		return "https://production.plaid.com/openid/certs"
	case "development":
		return "https://development.plaid.com/openid/certs"
	default:
		return "https://sandbox.plaid.com/openid/certs"
	}
}

func fetchPlaidPublicKey(ctx context.Context, jwksURL, kid string) (*rsa.PublicKey, error) {
	plaidKeyCacheMu.RLock()
	if time.Since(plaidKeyCacheAt) < plaidKeyCacheTTL {
		if key, ok := plaidKeyCache[kid]; ok {
			plaidKeyCacheMu.RUnlock()
			return key, nil
		}
	}
	plaidKeyCacheMu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build JWKS request: %w", err)
	}
	resp, err := plaidHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Plaid JWKS: %w", err)
	}
	defer resp.Body.Close()

	var jwks plaidJWKSet
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("decode Plaid JWKS: %w", err)
	}

	plaidKeyCacheMu.Lock()
	plaidKeyCache = make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		pub, err := jwkToRSA(k)
		if err != nil {
			continue
		}
		plaidKeyCache[k.Kid] = pub
	}
	plaidKeyCacheAt = time.Now()
	result := plaidKeyCache[kid]
	plaidKeyCacheMu.Unlock()

	if result == nil {
		return nil, fmt.Errorf("kid %q not found in Plaid JWKS", kid)
	}
	return result, nil
}

func jwkToRSA(k plaidJWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

// verifyPlaidWebhookJWT verifies the Plaid-Verification JWT:
//  1. Validates the RS256 signature against Plaid's published JWKS.
//  2. Confirms the JWT body hash matches SHA-256(requestBody).
func (hdl *Handlers) verifyPlaidWebhookJWT(ctx context.Context, token string, body []byte) error {
	jwksURL := plaidJWKSURL(hdl.cfg.PlaidEnvironment)

	claims := &plaidJWTClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return fetchPlaidPublicKey(ctx, jwksURL, kid)
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return fmt.Errorf("JWT invalid: %w", err)
	}

	sum := sha256.Sum256(body)
	expected := hex.EncodeToString(sum[:])
	if claims.RequestBodySHA256 != expected {
		return fmt.Errorf("body hash mismatch")
	}
	return nil
}
