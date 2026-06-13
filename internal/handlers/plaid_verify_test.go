package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

func TestPlaidJWKSURL(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{"production", "https://production.plaid.com/openid/certs"},
		{"development", "https://development.plaid.com/openid/certs"},
		{"sandbox", "https://sandbox.plaid.com/openid/certs"},
		{"", "https://sandbox.plaid.com/openid/certs"},
		{"unknown", "https://sandbox.plaid.com/openid/certs"},
	}
	for _, tc := range tests {
		if got := plaidJWKSURL(tc.env); got != tc.want {
			t.Errorf("plaidJWKSURL(%q) = %q, want %q", tc.env, got, tc.want)
		}
	}
}

func TestJwkToRSA_RoundTrip(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub := &priv.PublicKey

	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())

	got, err := jwkToRSA(plaidJWK{Kid: "k", N: n, E: e})
	if err != nil {
		t.Fatalf("jwkToRSA: %v", err)
	}
	if got.N.Cmp(pub.N) != 0 || got.E != pub.E {
		t.Fatal("round-trip key mismatch")
	}
}

func TestJwkToRSA_InvalidBase64(t *testing.T) {
	if _, err := jwkToRSA(plaidJWK{N: "!!!invalid", E: "AQAB"}); err == nil {
		t.Fatal("expected error for invalid N base64")
	}
	if _, err := jwkToRSA(plaidJWK{N: "AAAA", E: "!!!invalid"}); err == nil {
		t.Fatal("expected error for invalid E base64")
	}
}

func TestFetchPlaidPublicKey_HTTPFallback(t *testing.T) {
	plaidKeyCacheMu.Lock()
	plaidKeyCache = make(map[string]*rsa.PublicKey)
	plaidKeyCacheAt = time.Time{}
	plaidKeyCacheMu.Unlock()
	t.Cleanup(func() {
		plaidKeyCacheMu.Lock()
		plaidKeyCache = make(map[string]*rsa.PublicKey)
		plaidKeyCacheAt = time.Time{}
		plaidKeyCacheMu.Unlock()
	})

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub := &priv.PublicKey
	const kid = "http-test-kid"

	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	body, _ := json.Marshal(plaidJWKSet{Keys: []plaidJWK{{Kid: kid, N: n, E: e}}})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	got, err := fetchPlaidPublicKey(context.Background(), srv.URL, kid)
	if err != nil {
		t.Fatalf("fetchPlaidPublicKey: %v", err)
	}
	if got.N.Cmp(pub.N) != 0 || got.E != pub.E {
		t.Fatal("fetched key mismatch")
	}
}

func TestFetchPlaidPublicKey_KidNotFound(t *testing.T) {
	plaidKeyCacheMu.Lock()
	plaidKeyCache = make(map[string]*rsa.PublicKey)
	plaidKeyCacheAt = time.Time{}
	plaidKeyCacheMu.Unlock()
	t.Cleanup(func() {
		plaidKeyCacheMu.Lock()
		plaidKeyCache = make(map[string]*rsa.PublicKey)
		plaidKeyCacheAt = time.Time{}
		plaidKeyCacheMu.Unlock()
	})

	body, _ := json.Marshal(plaidJWKSet{Keys: []plaidJWK{}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	if _, err := fetchPlaidPublicKey(context.Background(), srv.URL, "missing-kid"); err == nil {
		t.Fatal("expected error for missing kid")
	}
}

func injectTestKey(t *testing.T, kid string) *rsa.PrivateKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	plaidKeyCacheMu.Lock()
	plaidKeyCache = map[string]*rsa.PublicKey{kid: &priv.PublicKey}
	plaidKeyCacheAt = time.Now()
	plaidKeyCacheMu.Unlock()
	t.Cleanup(func() {
		plaidKeyCacheMu.Lock()
		plaidKeyCache = make(map[string]*rsa.PublicKey)
		plaidKeyCacheAt = time.Time{}
		plaidKeyCacheMu.Unlock()
	})
	return priv
}

func signWebhookJWT(t *testing.T, priv *rsa.PrivateKey, kid, bodyHash string) string {
	t.Helper()
	claims := plaidJWTClaims{RequestBodySHA256: bodyHash}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Hour))
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestVerifyPlaidWebhookJWT_Valid(t *testing.T) {
	const kid = "valid-kid"
	priv := injectTestKey(t, kid)

	body := []byte(`{"webhook_type":"TRANSACTIONS"}`)
	sum := sha256.Sum256(body)
	signed := signWebhookJWT(t, priv, kid, hex.EncodeToString(sum[:]))

	hdl := &Handlers{cfg: &config.Config{PlaidEnvironment: "sandbox"}}
	if err := hdl.verifyPlaidWebhookJWT(context.Background(), signed, body); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestVerifyPlaidWebhookJWT_BodyHashMismatch(t *testing.T) {
	const kid = "hash-mismatch-kid"
	priv := injectTestKey(t, kid)

	body := []byte(`{"webhook_type":"TRANSACTIONS"}`)
	signed := signWebhookJWT(t, priv, kid, "0000000000000000000000000000000000000000000000000000000000000000")

	hdl := &Handlers{cfg: &config.Config{PlaidEnvironment: "sandbox"}}
	err := hdl.verifyPlaidWebhookJWT(context.Background(), signed, body)
	if err == nil || err.Error() != "body hash mismatch" {
		t.Fatalf("expected body hash mismatch error, got: %v", err)
	}
}

func TestVerifyPlaidWebhookJWT_BadToken(t *testing.T) {
	hdl := &Handlers{cfg: &config.Config{PlaidEnvironment: "sandbox"}}
	if err := hdl.verifyPlaidWebhookJWT(context.Background(), "not-a-jwt", []byte("body")); err == nil {
		t.Fatal("expected error for malformed JWT")
	}
}
