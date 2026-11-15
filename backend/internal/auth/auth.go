// Package auth handles the browser session: a random device token kept in an
// HttpOnly cookie. There are no accounts or passwords. A device (one browser)
// holds the family members created on it, and one of them is active.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"
)

const (
	// CookieName is the session cookie that carries the device token.
	CookieName = "lv_device"
	// DeviceTTL is how long a device session lives, in the cookie and on the server.
	DeviceTTL = 30 * 24 * time.Hour

	tokenBytes = 32
)

// Session is who is making the request: the browser and its active member.
// FamilyID is always taken from here, never from the request.
type Session struct {
	DeviceID string
	MemberID string
	FamilyID string
}

// NewToken returns 32 random bytes encoded as URL-safe base64 (43 characters).
// It is used for device tokens and invite tokens.
func NewToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns the SHA-256 of a device token. Only this hash is stored.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// DeviceToken returns the token from the request's session cookie, or "".
func DeviceToken(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// SetDeviceCookie stores the device token in the browser. The cookie is
// Secure when forceSecure is set (production) or the request came over HTTPS,
// directly or through a proxy that sets X-Forwarded-Proto.
func SetDeviceCookie(w http.ResponseWriter, r *http.Request, token string, forceSecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(DeviceTTL / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   forceSecure || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
}

type sessionKey struct{}

// WithSession returns a copy of ctx that carries s.
func WithSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

// SessionFrom returns the session stored by WithSession.
func SessionFrom(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(Session)
	return s, ok
}
