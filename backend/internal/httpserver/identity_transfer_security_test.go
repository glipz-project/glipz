package httpserver

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"glipz.io/backend/internal/config"
	"glipz.io/backend/internal/repo"
)

func TestIdentityBundleV2EncryptDecrypt(t *testing.T) {
	enc, err := encryptIdentityPrivateKey("correct horse battery staple", "PRIVATE_KEY_BYTES")
	if err != nil {
		t.Fatalf("encryptIdentityPrivateKey: %v", err)
	}
	got, err := decryptIdentityPrivateKey("correct horse battery staple", enc)
	if err != nil {
		t.Fatalf("decryptIdentityPrivateKey: %v", err)
	}
	if got != "PRIVATE_KEY_BYTES" {
		t.Fatalf("decrypted private key = %q", got)
	}
	if _, err := decryptIdentityPrivateKey("wrong horse battery staple", enc); err == nil {
		t.Fatal("decryptIdentityPrivateKey with wrong passphrase succeeded")
	}
}

func TestIdentityBundleV2EncryptDecryptsToValidPortableIdentity(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	enc := base64.RawURLEncoding
	sum := sha256.Sum256(pub)
	identity := repo.PortableIdentity{
		PortableID:                 repo.PortableIDPrefix + enc.EncodeToString(sum[:]),
		AccountPublicKey:           enc.EncodeToString(pub),
		AccountPrivateKeyEncrypted: enc.EncodeToString(priv),
	}

	bundleKey, err := encryptIdentityPrivateKey("correct horse battery staple", identity.AccountPrivateKeyEncrypted)
	if err != nil {
		t.Fatalf("encryptIdentityPrivateKey: %v", err)
	}
	decryptedPrivateKey, err := decryptIdentityPrivateKey("correct horse battery staple", bundleKey)
	if err != nil {
		t.Fatalf("decryptIdentityPrivateKey: %v", err)
	}
	identity.AccountPrivateKeyEncrypted = decryptedPrivateKey
	if _, err := repo.ValidatePortableIdentity(identity); err != nil {
		t.Fatalf("ValidatePortableIdentity: %v", err)
	}
}

func TestTransferTokenHashComparison(t *testing.T) {
	hash := hashTransferToken("token-value")
	if !constantEqualBase64Hash(hash, hashTransferToken("token-value")) {
		t.Fatal("same token did not compare equal")
	}
	if constantEqualBase64Hash(hash, hashTransferToken("other-token")) {
		t.Fatal("different token compared equal")
	}
}

func TestNormalizeTransferOriginRejectsUnsafeOrigins(t *testing.T) {
	if _, _, err := normalizeTransferOrigin("http://example.com"); err == nil {
		t.Fatal("non-local http origin accepted")
	}
	if _, _, err := normalizeTransferOrigin("https://example.com/path"); err == nil {
		t.Fatal("origin with path accepted")
	}
	if _, _, err := normalizeTransferOrigin("http://127.0.0.1:8080"); err == nil {
		t.Fatal("loopback origin accepted without development opt-in")
	}
	if _, local, err := normalizeTransferOriginWithLoopback("http://127.0.0.1:8080", true); err != nil || !local {
		t.Fatalf("local development origin rejected: local=%v err=%v", local, err)
	}
}

func TestTransferReservedAddressesRejected(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "198.18.0.1", "192.0.2.1", "[::1]", "[64:ff9b::a00:1]", "[2002:7f00:1::1]"} {
		t.Run(host, func(t *testing.T) {
			if _, _, err := normalizeTransferOrigin("https://" + host); err == nil {
				t.Fatal("unsafe transfer origin accepted")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			conn, err := publicOutboundDialContext(ctx, "tcp", net.JoinHostPort(strings.Trim(host, "[]"), "443"))
			if conn != nil {
				conn.Close()
			}
			if err == nil {
				t.Fatal("unsafe connection allowed")
			}
		})
	}
}

func TestTransferClientBlocksLoopbackAtConnection(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	for _, allow := range []bool{false, true} {
		s := &Server{cfg: config.Config{IdentityTransferAllowLoopback: allow}}
		client := s.newIdentityTransferHTTPClient()
		defer client.CloseIdleConnections()
		if client.Transport.(*http.Transport).Proxy != nil {
			t.Fatal("transfer client uses environment proxy")
		}
		// Bypass URL preflight: the transport must protect the actual connection too.
		res, err := client.Get(target.URL)
		if res != nil {
			res.Body.Close()
		}
		if allow && err != nil {
			t.Fatalf("explicit local development request failed: %v", err)
		}
		if !allow && (err == nil || hits.Load() != 0) {
			t.Fatal("production transfer reached loopback")
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("local development hits = %d", hits.Load())
	}
}

func TestTransferClientDoesNotFollowRedirects(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()
	s := &Server{cfg: config.Config{IdentityTransferAllowLoopback: true}}
	client := s.newIdentityTransferHTTPClient()
	defer client.CloseIdleConnections()
	req, _ := http.NewRequest(http.MethodGet, source.URL, nil)
	req.Header.Set("X-Glipz-Transfer-Token", "secret")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusFound || hits.Load() != 0 {
		t.Fatal("transfer redirect was followed")
	}
}

func TestIdentityBundleRejectsMalformedSaltAndNonce(t *testing.T) {
	valid := encryptedIdentitySecret{
		KDF:   "argon2id:v=19,m=65536,t=3,p=2",
		Salt:  base64.RawURLEncoding.EncodeToString(make([]byte, identityBundleSaltBytes)),
		Nonce: base64.RawURLEncoding.EncodeToString(make([]byte, identityBundleNonceBytes)),
	}
	for _, size := range []int{0, 1, 11, 13, 15, 17, 32} {
		for _, field := range []string{"salt", "nonce"} {
			enc := valid
			value := base64.RawURLEncoding.EncodeToString(make([]byte, size))
			if field == "salt" {
				enc.Salt = value
			} else {
				enc.Nonce = value
			}
			if _, err := decryptIdentityPrivateKey("correct horse battery staple", enc); err == nil {
				t.Fatalf("malformed %s length %d accepted", field, size)
			}
		}
	}
}
