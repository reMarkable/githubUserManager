package githuborg

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// appTransport is an http.RoundTripper that authenticates requests as a GitHub
// App installation. It mints a short-lived JWT signed with the app's private
// key, exchanges it for an installation access token, and caches that token
// until shortly before it expires.
//
// It replaces github.com/bradleyfalzon/ghinstallation with a stdlib-only
// implementation covering just what this service needs.
type appTransport struct {
	base           http.RoundTripper
	appID          int64
	installationID int64
	key            *rsa.PrivateKey

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

// newAppTransport parses the PEM-encoded RSA private key and returns a
// RoundTripper authenticating as the given app installation.
func newAppTransport(base http.RoundTripper, appID, installationID int64, pemKey []byte) (*appTransport, error) {
	if base == nil {
		base = http.DefaultTransport
	}
	key, err := parsePrivateKey(pemKey)
	if err != nil {
		return nil, err
	}
	return &appTransport{
		base:           base,
		appID:          appID,
		installationID: installationID,
		key:            key,
	}, nil
}

func parsePrivateKey(pemKey []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, fmt.Errorf("parsing private key: no PEM block found")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("parsing private key: not an RSA key")
	}
	return key, nil
}

func (t *appTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.installationToken(req.Context())
	if err != nil {
		return nil, err
	}
	// Clone so we don't mutate the caller's request.
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+token)
	clone.Header.Set("Accept", "application/vnd.github+json")
	return t.base.RoundTrip(clone)
}

// installationToken returns a cached installation token, refreshing it when it
// is missing or within a minute of expiry.
func (t *appTransport) installationToken(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.token != "" && time.Until(t.tokenExpiry) > time.Minute {
		return t.token, nil
	}

	jwt, err := t.appJWT()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", t.installationID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+jwt)
	httpReq.Header.Set("Accept", "application/vnd.github+json")

	resp, err := t.base.RoundTrip(httpReq)
	if err != nil {
		return "", fmt.Errorf("requesting installation token: %w", err)
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			slog.Warn("Failed to close response body", "err", err)
		}
	}()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("requesting installation token: unexpected status %s: %s", resp.Status, body)
	}

	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decoding installation token: %w", err)
	}

	t.token = out.Token
	t.tokenExpiry = out.ExpiresAt
	return t.token, nil
}

// appJWT builds and signs the short-lived RS256 JWT used to authenticate as the
// app itself when requesting installation tokens.
func (t *appTransport) appJWT() (string, error) {
	now := time.Now()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	// Backdate iat by 60s to tolerate clock drift; GitHub caps lifetime at 10m.
	claims := map[string]any{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": strconv.FormatInt(t.appID, 10),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	signingInput := encodeSegment(headerJSON) + "." + encodeSegment(claimsJSON)
	hashed := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, t.key, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("signing jwt: %w", err)
	}
	return signingInput + "." + encodeSegment(sig), nil
}

func encodeSegment(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
