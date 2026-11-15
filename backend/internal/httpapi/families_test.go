package httpapi_test

// These tests run the real router against a real PostgreSQL (see testdb).
// Each client has its own cookie jar, so one client is one browser.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"littlevillage/internal/auth"
	"littlevillage/internal/httpapi"
	"littlevillage/internal/store"
	"littlevillage/internal/testdb"
)

type familyView struct {
	Family  store.FamilyRef `json:"family"`
	Members []store.Member  `json:"members"`
	Invite  struct {
		Token string `json:"token"`
		Path  string `json:"path"`
	} `json:"invite"`
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newServer(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	srv := httptest.NewServer(httpapi.NewRouter(store.New(pool), httpapi.Options{StaticDir: t.TempDir()}))
	t.Cleanup(srv.Close)
	return srv, pool
}

// browser is an HTTP client with its own cookie jar.
type browser struct {
	t    *testing.T
	base string
	http *http.Client
}

func newBrowser(t *testing.T, srv *httptest.Server) *browser {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &browser{t: t, base: srv.URL, http: &http.Client{Jar: jar}}
}

// call sends body as JSON, decodes the JSON reply into out (if not nil) and
// returns the HTTP status.
func (b *browser) call(method, path string, body, out any) int {
	b.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			b.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, b.base+path, reader)
	if err != nil {
		b.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		b.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			b.t.Fatalf("%s %s: decode reply (status %d): %v", method, path, resp.StatusCode, err)
		}
	}
	return resp.StatusCode
}

func (b *browser) createFamily(family, name, relationship string) store.Me {
	b.t.Helper()
	var me store.Me
	body := map[string]string{"family_name": family, "member_name": name, "relationship": relationship}
	if status := b.call(http.MethodPost, "/api/families", body, &me); status != http.StatusCreated {
		b.t.Fatalf("create family: status %d", status)
	}
	return me
}

func (b *browser) join(token, name, relationship string) store.Me {
	b.t.Helper()
	var me store.Me
	body := map[string]string{"token": token, "member_name": name, "relationship": relationship}
	if status := b.call(http.MethodPost, "/api/families/join", body, &me); status != http.StatusCreated {
		b.t.Fatalf("join family: status %d", status)
	}
	return me
}

func (b *browser) me() store.Me {
	b.t.Helper()
	var me store.Me
	if status := b.call(http.MethodGet, "/api/me", nil, &me); status != http.StatusOK {
		b.t.Fatalf("get me: status %d", status)
	}
	return me
}

func (b *browser) family() familyView {
	b.t.Helper()
	var fam familyView
	if status := b.call(http.MethodGet, "/api/family", nil, &fam); status != http.StatusOK {
		b.t.Fatalf("get family: status %d", status)
	}
	return fam
}

func (b *browser) switchTo(memberID string) (int, apiError) {
	b.t.Helper()
	var e apiError
	status := b.call(http.MethodPost, "/api/session/switch", map[string]string{"member_id": memberID}, &e)
	return status, e
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func memberNames(ms []store.Member) []string {
	names := make([]string, len(ms))
	for i, m := range ms {
		names[i] = m.Name
	}
	return names
}

func TestFamilyMemberCannotSeeAnotherFamily(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	alice := newBrowser(t, srv)
	bob := newBrowser(t, srv)

	meA := alice.createFamily("Family A", "Alice", "Mom")
	meB := bob.createFamily("Family B", "Bob", "Dad")

	famA := alice.family()
	if famA.Family.ID != meA.Family.ID || famA.Family.Name != "Family A" {
		t.Fatalf("Alice got family %+v, want Family A", famA.Family)
	}
	if got := memberNames(famA.Members); len(got) != 1 || got[0] != "Alice" {
		t.Fatalf("Family A members = %v, want [Alice]", got)
	}
	if famB := bob.family(); famA.Invite.Token == famB.Invite.Token {
		t.Fatal("two families share one invite token")
	}

	// The family always comes from the session: a family_id in the request is ignored.
	var fam familyView
	alice.call(http.MethodGet, "/api/family?family_id="+meB.Family.ID, nil, &fam)
	if fam.Family.ID != meA.Family.ID {
		t.Fatalf("family_id query parameter changed the family to %s", fam.Family.ID)
	}

	// Alice cannot make Bob active in her browser.
	if status, e := alice.switchTo(meB.Member.ID); status != http.StatusForbidden || e.Error.Code != "forbidden" {
		t.Fatalf("switch to other family's member: status %d code %q, want 403 forbidden", status, e.Error.Code)
	}
	if got := alice.family().Family.ID; got != meA.Family.ID {
		t.Fatalf("after failed switch Alice sees family %s", got)
	}
}

func TestSwitchOnlyToMembersOfThisBrowser(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	home := newBrowser(t, srv)

	mom := home.createFamily("The Smiths", "Anna", "Mom")
	token := home.family().Invite.Token

	// The same browser adds Grandma and she becomes active.
	gran := home.join(token, "Mary", "Grandma")
	if gran.Member.Name != "Mary" || gran.Family.ID != mom.Family.ID || len(gran.DeviceMembers) != 2 {
		t.Fatalf("after join: %+v", gran)
	}

	// Switch back to Mom; it persists.
	if status, _ := home.switchTo(mom.Member.ID); status != http.StatusOK {
		t.Fatalf("switch to own member: status %d", status)
	}
	if got := home.me().Member.ID; got != mom.Member.ID {
		t.Fatalf("active member = %s, want Mom %s", got, mom.Member.ID)
	}

	// Dad joins the same family from another browser.
	dad := newBrowser(t, srv).join(token, "Tom", "Dad")
	if got := len(home.family().Members); got != 3 {
		t.Fatalf("family has %d members, want 3", got)
	}

	// Same family is not enough: Dad was not added in this browser.
	for _, id := range []string{dad.Member.ID, "00000000-0000-0000-0000-000000000000"} {
		if status, e := home.switchTo(id); status != http.StatusForbidden || e.Error.Code != "forbidden" {
			t.Fatalf("switch to %s: status %d code %q, want 403 forbidden", id, status, e.Error.Code)
		}
	}
	if status, e := home.switchTo("not-a-uuid"); status != http.StatusBadRequest || e.Error.Code != "validation_error" {
		t.Fatalf("switch to malformed id: status %d code %q, want 400 validation_error", status, e.Error.Code)
	}
	if got := home.me().Member.ID; got != mom.Member.ID {
		t.Fatalf("failed switches changed the active member to %s", got)
	}
}

func TestInviteLookupAndUnknownToken(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	owner := newBrowser(t, srv)
	owner.createFamily("The Lees", "Grace", "Mom")
	token := owner.family().Invite.Token

	var inv store.Invite
	if status := newBrowser(t, srv).call(http.MethodGet, "/api/invites/"+token, nil, &inv); status != http.StatusOK {
		t.Fatalf("get invite: status %d", status)
	}
	if inv.FamilyName != "The Lees" || inv.InvitedBy != "Grace" {
		t.Fatalf("invite = %+v", inv)
	}

	stranger := newBrowser(t, srv)
	var e apiError
	status := stranger.call(http.MethodPost, "/api/families/join",
		map[string]string{"token": "no-such-token", "member_name": "Eve", "relationship": "Other"}, &e)
	if status != http.StatusNotFound || e.Error.Code != "invite_not_found" {
		t.Fatalf("join with unknown token: status %d code %q, want 404 invite_not_found", status, e.Error.Code)
	}
	if status := stranger.call(http.MethodGet, "/api/invites/no-such-token", nil, &e); status != http.StatusNotFound {
		t.Fatalf("get unknown invite: status %d, want 404", status)
	}
	if status := stranger.call(http.MethodGet, "/api/me", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("failed join left a session: /api/me status %d", status)
	}
}

func TestNameValidation(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	token := func() string {
		b := newBrowser(t, srv)
		b.createFamily("Validation", "Owner", "Mom")
		return b.family().Invite.Token
	}()

	cases := []struct {
		name         string
		path         string
		body         map[string]string
		wantStatus   int
		wantStoredAs string
	}{
		{"empty name", "/api/families", map[string]string{"family_name": "F", "member_name": "", "relationship": "Mom"}, 400, ""},
		{"spaces only", "/api/families", map[string]string{"family_name": "F", "member_name": "   ", "relationship": "Mom"}, 400, ""},
		{"41 characters", "/api/families", map[string]string{"family_name": "F", "member_name": strings.Repeat("a", 41), "relationship": "Mom"}, 400, ""},
		{"empty family name", "/api/families", map[string]string{"family_name": " ", "member_name": "Ann", "relationship": "Mom"}, 400, ""},
		{"41-character family name", "/api/families", map[string]string{"family_name": strings.Repeat("f", 41), "member_name": "Ann", "relationship": "Mom"}, 400, ""},
		{"unknown relationship", "/api/families", map[string]string{"family_name": "F", "member_name": "Ann", "relationship": "Aunt"}, 400, ""},
		{"relationship is case-sensitive", "/api/families", map[string]string{"family_name": "F", "member_name": "Ann", "relationship": "mom"}, 400, ""},
		{"join: 41 characters", "/api/families/join", map[string]string{"token": token, "member_name": strings.Repeat("b", 41), "relationship": "Dad"}, 400, ""},
		{"join: spaces only", "/api/families/join", map[string]string{"token": token, "member_name": "\t ", "relationship": "Dad"}, 400, ""},
		{"40 characters", "/api/families", map[string]string{"family_name": "F", "member_name": strings.Repeat("a", 40), "relationship": "Mom"}, 201, strings.Repeat("a", 40)},
		{"40 non-ASCII characters", "/api/families", map[string]string{"family_name": "F", "member_name": strings.Repeat("я", 40), "relationship": "Mom"}, 201, strings.Repeat("я", 40)},
		{"trimmed", "/api/families", map[string]string{"family_name": "  F  ", "member_name": "  Anna  ", "relationship": "Mom"}, 201, "Anna"},
		{"join: trimmed", "/api/families/join", map[string]string{"token": token, "member_name": " Joe ", "relationship": "Grandpa"}, 201, "Joe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBrowser(t, srv)
			if tc.wantStatus != http.StatusCreated {
				var e apiError
				status := b.call(http.MethodPost, tc.path, tc.body, &e)
				if status != tc.wantStatus || e.Error.Code != "validation_error" || e.Error.Message == "" {
					t.Fatalf("status %d error %+v, want %d validation_error", status, e.Error, tc.wantStatus)
				}
				if status := b.call(http.MethodGet, "/api/me", nil, nil); status != http.StatusUnauthorized {
					t.Fatalf("invalid request created a session: /api/me status %d", status)
				}
				return
			}
			var me store.Me
			if status := b.call(http.MethodPost, tc.path, tc.body, &me); status != tc.wantStatus {
				t.Fatalf("status %d, want %d", status, tc.wantStatus)
			}
			if me.Member.Name != tc.wantStoredAs {
				t.Fatalf("stored name %q, want %q", me.Member.Name, tc.wantStoredAs)
			}
		})
	}

	var e apiError
	if status := newBrowser(t, srv).call(http.MethodPost, "/api/families", "not an object", &e); status != 400 || e.Error.Code != "invalid_json" {
		t.Fatalf("non-object body: status %d code %q, want 400 invalid_json", status, e.Error.Code)
	}
}

func TestPrivateEndpointsNeedSession(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/me"},
		{http.MethodGet, "/api/family"},
		{http.MethodPost, "/api/session/switch"},
	}
	forged := newBrowser(t, srv)
	forged.http.Jar.SetCookies(mustURL(t, srv.URL), []*http.Cookie{{Name: auth.CookieName, Value: "forged-token"}})

	for _, b := range []*browser{newBrowser(t, srv), forged} {
		for _, ep := range endpoints {
			var e apiError
			status := b.call(ep.method, ep.path, map[string]string{"member_id": "00000000-0000-0000-0000-000000000000"}, &e)
			if status != http.StatusUnauthorized || e.Error.Code != "unauthenticated" {
				t.Errorf("%s %s: status %d code %q, want 401 unauthenticated", ep.method, ep.path, status, e.Error.Code)
			}
		}
	}
}

func TestFamilyMemberLimit(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	owner := newBrowser(t, srv)
	owner.createFamily("Big Family", "Owner", "Mom")
	token := owner.family().Invite.Token

	for i := 2; i <= store.MaxMembers; i++ {
		newBrowser(t, srv).join(token, "Relative", "Other")
	}
	var e apiError
	status := newBrowser(t, srv).call(http.MethodPost, "/api/families/join",
		map[string]string{"token": token, "member_name": "One too many", "relationship": "Other"}, &e)
	if status != http.StatusConflict || e.Error.Code != "family_full" {
		t.Fatalf("11th member: status %d code %q, want 409 family_full", status, e.Error.Code)
	}
	if got := len(owner.family().Members); got != store.MaxMembers {
		t.Fatalf("family has %d members, want %d", got, store.MaxMembers)
	}
}

func TestDeviceCookie(t *testing.T) {
	t.Parallel()
	srv, pool := newServer(t)
	body := `{"family_name":"Cookie","member_name":"Ann","relationship":"Mom"}`

	for _, viaHTTPS := range []bool{false, true} {
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/families", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if viaHTTPS {
			req.Header.Set("X-Forwarded-Proto", "https")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		var c *http.Cookie
		for _, rc := range resp.Cookies() {
			if rc.Name == auth.CookieName {
				c = rc
			}
		}
		if c == nil {
			t.Fatalf("no %s cookie in reply", auth.CookieName)
		}
		if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 30*24*60*60 {
			t.Fatalf("cookie attributes: %+v", c)
		}
		if c.Secure != viaHTTPS {
			t.Fatalf("Secure = %v for X-Forwarded-Proto https = %v", c.Secure, viaHTTPS)
		}
		if len(c.Value) < 43 {
			t.Fatalf("token %q is shorter than 32 bytes of base64", c.Value)
		}

		// Only the SHA-256 of the token is stored.
		var n int
		if err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM devices WHERE token_hash = $1`, auth.HashToken(c.Value),
		).Scan(&n); err != nil || n != 1 {
			t.Fatalf("devices with the token's hash: %d (err %v), want 1", n, err)
		}
	}
}
