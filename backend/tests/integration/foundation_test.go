package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"journey-safety/backend/internal/config"
	"journey-safety/backend/internal/platform/database"
	"journey-safety/backend/internal/server"
)

var db *pgxpool.Pool
var dsn string
var app *httptest.Server
var logs lockedBuffer
var cfg config.Config

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(v []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(v)
}
func (b *lockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		fmt.Println("SKIP integration: set TEST_DATABASE_URL or run ./dev integration")
		os.Exit(0)
	}
	u, err := url.Parse(base)
	if err != nil || !strings.HasSuffix(u.Path, "_test") {
		fmt.Fprintln(os.Stderr, "TEST_DATABASE_URL must point to a dedicated database ending in _test")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot connect to test database")
		os.Exit(1)
	}
	name := "phase1_" + strings.ReplaceAll(uuid.NewString(), "-", "") + "_test"
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()); err != nil {
		fmt.Fprintln(os.Stderr, "cannot create isolated test database")
		os.Exit(1)
	}
	u.Path = "/" + name
	dsn = u.String()
	exitCode := runSuite(m)
	cleanup, release := context.WithTimeout(context.Background(), 10*time.Second)
	defer release()
	if _, err = admin.Exec(cleanup, `DROP DATABASE `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`); err != nil {
		fmt.Fprintln(os.Stderr, "failed to remove isolated test database")
		exitCode = 1
	}
	admin.Close(cleanup)
	os.Exit(exitCode)
}
func runSuite(m *testing.M) int {
	ctx := context.Background()
	if err := database.Migrate(ctx, dsn, "up"); err != nil {
		fmt.Fprintln(os.Stderr, "test migrations failed:", err)
		return 1
	}
	var err error
	db, err = database.Open(ctx, dsn, 10)
	if err != nil {
		fmt.Fprintln(os.Stderr, "test pool failed")
		return 1
	}
	defer db.Close()
	cfg = config.Config{Environment: "test", AllowedOrigins: []string{"http://localhost:5173"}, SessionTTL: time.Hour, InvitationTTL: time.Hour, AuthRatePerMinute: 1000}
	r, err := server.Router(cfg, db, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "test router failed")
		return 1
	}
	app = httptest.NewServer(r)
	defer app.Close()
	return m.Run()
}

type actor struct {
	client                 *http.Client
	csrf, email, id, token string
}
type response struct {
	status  int
	body    []byte
	header  http.Header
	cookies []*http.Cookie
}

func (a actor) request(method, path string, payload any) response {
	var body io.Reader
	if payload != nil {
		v, _ := json.Marshal(payload)
		body = bytes.NewReader(v)
	}
	req, _ := http.NewRequest(method, app.URL+path, body)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Content-Type", "application/json")
	if a.csrf != "" {
		req.Header.Set("X-CSRF-Token", a.csrf)
	}
	r, err := a.client.Do(req)
	if err != nil {
		return response{status: 0, body: []byte(err.Error())}
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return response{r.StatusCode, b, r.Header, r.Cookies()}
}
func expect(t *testing.T, r response, status int) {
	t.Helper()
	if r.status != status {
		var e struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		json.Unmarshal(r.body, &e)
		t.Fatalf("status %d, want %d; error=%s", r.status, status, e.Error.Code)
	}
}
func data[T any](t *testing.T, r response) T {
	t.Helper()
	var envelope struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(r.body, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}
func newActor(t *testing.T) actor {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	a := actor{client: &http.Client{Jar: jar, Timeout: 10 * time.Second}, email: uuid.NewString() + "@example.com"}
	r := a.request("POST", "/api/v1/auth/register", map[string]string{"email": a.email, "password": "test-password-long-123", "display_name": "Người dùng thử", "device_name": "integration"})
	expect(t, r, 201)
	v := data[struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		CSRF string `json:"csrf_token"`
	}](t, r)
	a.id = v.User.ID
	a.csrf = v.CSRF
	for _, c := range r.cookies {
		if c.Name == "journey_session" {
			a.token = c.Value
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
				t.Fatal("unsafe session cookie")
			}
		}
	}
	return a
}
func invite(t *testing.T, owner actor, email string) (string, string) {
	t.Helper()
	r := owner.request("POST", "/api/v1/trusted-contacts/invitations", map[string]string{"email": email})
	expect(t, r, 201)
	v := data[struct {
		Invitation struct {
			ID string `json:"id"`
		} `json:"invitation"`
		Token string `json:"invitation_token"`
	}](t, r)
	return v.Invitation.ID, v.Token
}

func TestMigrationRoundTripAndExtensions(t *testing.T) {
	ctx := context.Background()
	if !database.Ready(ctx, db) {
		t.Fatal("database not ready")
	}
	if err := database.Migrate(ctx, dsn, "down"); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, dsn, "down"); err != nil {
		t.Fatal(err)
	}
	if database.Ready(ctx, db) {
		t.Fatal("readiness remained true after rollback")
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM pg_extension WHERE extname IN ('postgis','timescaledb')`).Scan(&n); err != nil || n != 2 {
		t.Fatal("rollback dropped shared extensions")
	}
	if err := database.Migrate(ctx, dsn, "up"); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, dsn, "up"); err != nil {
		t.Fatal("up was not repeatable")
	}
	var meters float64
	if err := db.QueryRow(ctx, `SELECT ST_Distance(ST_SetSRID(ST_Point(106.7,10.7),4326)::geography,ST_SetSRID(ST_Point(106.7,10.701),4326)::geography)`).Scan(&meters); err != nil || meters < 100 || meters > 120 {
		t.Fatal("PostGIS geography distance unavailable")
	}
	// Verify hypertables are genuinely usable, without shipping Phase 3 schema.
	_, err := db.Exec(ctx, `CREATE TABLE extension_probe(recorded_at timestamptz NOT NULL, value int); SELECT create_hypertable('extension_probe','recorded_at'); INSERT INTO extension_probe VALUES(now(),1); DROP TABLE extension_probe`)
	if err != nil {
		t.Fatal("TimescaleDB hypertable probe failed:", err)
	}
}

func TestAuthenticationSessionsAndCSRF(t *testing.T) {
	a := newActor(t)
	expect(t, a.request("GET", "/api/v1/users/me", nil), 200)
	r := a.request("POST", "/api/v1/auth/register", map[string]string{"email": strings.ToUpper(a.email), "password": "test-password-long-123", "display_name": "Khác"})
	expect(t, r, 409)
	without := a
	without.csrf = ""
	expect(t, without.request("PATCH", "/api/v1/users/me", map[string]string{"display_name": "Đổi tên"}), 403)
	wrong := a
	wrong.csrf = "wrong"
	expect(t, wrong.request("PATCH", "/api/v1/users/me", map[string]string{"display_name": "Đổi tên"}), 403)
	r = a.request("PATCH", "/api/v1/users/me", map[string]string{"display_name": "Tên mới tiếng Việt"})
	expect(t, r, 200)
	if data[struct {
		Name string `json:"display_name"`
	}](t, r).Name != "Tên mới tiếng Việt" {
		t.Fatal("profile not updated")
	}
	jar, _ := cookiejar.New(nil)
	b := actor{client: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
	for _, email := range []string{a.email, "missing-" + a.email} {
		r = b.request("POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "wrong-password-long"})
		expect(t, r, 401)
	}
	r = b.request("POST", "/api/v1/auth/login", map[string]string{"email": a.email, "password": "test-password-long-123", "device_name": "second"})
	expect(t, r, 200)
	b.csrf = data[struct {
		Token string `json:"csrf_token"`
	}](t, r).Token
	r = a.request("GET", "/api/v1/auth/sessions", nil)
	expect(t, r, 200)
	if len(data[[]any](t, r)) != 2 {
		t.Fatal("session management failed")
	}
	expect(t, a.request("POST", "/api/v1/auth/logout", map[string]any{}), 204)
	expect(t, a.request("GET", "/api/v1/users/me", nil), 401)
	expect(t, b.request("GET", "/api/v1/users/me", nil), 200)
	expect(t, b.request("DELETE", "/api/v1/auth/sessions", nil), 204)
	expect(t, b.request("GET", "/api/v1/users/me", nil), 401)
}

func TestSessionExpiryAndCrossUserRevocation(t *testing.T) {
	a, b := newActor(t), newActor(t)
	sessions := data[[]struct {
		ID string `json:"id"`
	}](t, a.request("GET", "/api/v1/auth/sessions", nil))
	id := sessions[0].ID
	expect(t, b.request("DELETE", "/api/v1/auth/sessions/"+id, nil), 404)
	expect(t, a.request("GET", "/api/v1/users/me", nil), 200)
	_, err := db.Exec(context.Background(), `UPDATE user_sessions SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, a.request("GET", "/api/v1/auth/session", nil), 401)
	sessions = data[[]struct {
		ID string `json:"id"`
	}](t, b.request("GET", "/api/v1/auth/sessions", nil))
	expect(t, b.request("DELETE", "/api/v1/auth/sessions/"+sessions[0].ID, nil), 204)
	expect(t, b.request("GET", "/api/v1/auth/session", nil), 401)
}

func TestTrustedContactConsentAndRevocation(t *testing.T) {
	owner, recipient, outsider := newActor(t), newActor(t), newActor(t)
	expect(t, owner.request("POST", "/api/v1/trusted-contacts/invitations", map[string]string{"email": owner.email}), 400)
	id, token := invite(t, owner, recipient.email)
	path := "/api/v1/trusted-contacts/invitations/" + id
	if len(data[[]any](t, owner.request("GET", "/api/v1/trusted-contacts", nil))) != 0 {
		t.Fatal("unconfirmed contact granted")
	}
	r := recipient.request("GET", "/api/v1/trusted-contacts/invitations", nil)
	expect(t, r, 200)
	if strings.Contains(string(r.body), token) || strings.Contains(string(r.body), "token_hash") {
		t.Fatal("invitation leaked token")
	}
	expect(t, outsider.request("POST", path+"/accept", map[string]string{"token": token}), 404)
	expect(t, recipient.request("POST", path+"/accept", map[string]string{"token": strings.Repeat("x", 43)}), 404)
	expect(t, outsider.request("DELETE", path, nil), 404)
	r = recipient.request("POST", path+"/accept", map[string]string{"token": token})
	expect(t, r, 200)
	contact := data[struct {
		ID string `json:"id"`
	}](t, r)
	editPath := "/api/v1/trusted-contacts/" + contact.ID
	expect(t, recipient.request("PATCH", editPath, map[string]string{"label": "Không được đổi"}), 404)
	expect(t, outsider.request("PATCH", editPath, map[string]string{"label": "Không được đổi"}), 404)
	r = owner.request("PATCH", editPath, map[string]string{"label": "Mẹ"})
	expect(t, r, 200)
	if data[struct {
		Label string `json:"label"`
	}](t, r).Label != "Mẹ" {
		t.Fatal("contact label was not updated")
	}
	expect(t, recipient.request("POST", path+"/accept", map[string]string{"token": token}), 200)
	if len(data[[]any](t, owner.request("GET", "/api/v1/trusted-contacts", nil))) != 1 {
		t.Fatal("accepted contact missing")
	}
	if len(data[[]any](t, recipient.request("GET", "/api/v1/trusted-contacts", nil))) != 0 {
		t.Fatal("trust incorrectly became reciprocal")
	}
	if len(data[[]any](t, recipient.request("GET", "/api/v1/trusted-contacts/trusted-by", nil))) != 1 {
		t.Fatal("recipient cannot inspect relationship")
	}
	expect(t, outsider.request("DELETE", "/api/v1/trusted-contacts/"+contact.ID, nil), 404)
	expect(t, recipient.request("DELETE", "/api/v1/trusted-contacts/"+contact.ID, nil), 204)
	expect(t, recipient.request("POST", path+"/accept", map[string]string{"token": token}), 409)
	if len(data[[]any](t, owner.request("GET", "/api/v1/trusted-contacts", nil))) != 0 {
		t.Fatal("revoked relationship still visible")
	}
	newID, newToken := invite(t, owner, recipient.email)
	r = recipient.request("POST", "/api/v1/trusted-contacts/invitations/"+newID+"/accept", map[string]string{"token": newToken})
	expect(t, r, 200)
	if data[struct {
		ID string `json:"id"`
	}](t, r).ID != contact.ID {
		t.Fatal("reinvitation duplicated contact")
	}
	expect(t, owner.request("DELETE", "/api/v1/trusted-contacts/"+contact.ID, nil), 204)
	for _, secret := range []string{owner.email, token, newToken, owner.token, "test-password-long-123"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("sensitive data in application logs")
		}
	}
}

func TestConcurrentInvitationLimit(t *testing.T) {
	owner := newActor(t)
	var wg sync.WaitGroup
	results := make(chan int, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := owner.request("POST", "/api/v1/trusted-contacts/invitations", map[string]string{"email": uuid.NewString() + "@example.com"})
			results <- r.status
		}()
	}
	wg.Wait()
	close(results)
	created, limited := 0, 0
	for s := range results {
		switch s {
		case 201:
			created++
		case 409:
			limited++
		default:
			t.Fatalf("unexpected concurrent response %d", s)
		}
	}
	if created != 3 || limited != 9 {
		t.Fatalf("limit race: created=%d limited=%d", created, limited)
	}
}

func TestConcurrentAcceptCancel(t *testing.T) {
	for i := 0; i < 5; i++ {
		owner, recipient := newActor(t), newActor(t)
		id, token := invite(t, owner, recipient.email)
		path := "/api/v1/trusted-contacts/invitations/" + id
		start := make(chan struct{})
		results := make(chan int, 2)
		go func() {
			<-start
			results <- recipient.request("POST", path+"/accept", map[string]string{"token": token}).status
		}()
		go func() { <-start; results <- owner.request("DELETE", path, nil).status }()
		close(start)
		a, b := <-results, <-results
		if !((a == 200 && b == 409) || (a == 409 && b == 200) || (a == 204 && b == 409) || (a == 409 && b == 204)) {
			t.Fatalf("inconsistent race: %d %d", a, b)
		}
	}
}

func TestExpiryDeclineCancelAndReinvite(t *testing.T) {
	owner, recipient := newActor(t), newActor(t)
	id, token := invite(t, owner, recipient.email)
	path := "/api/v1/trusted-contacts/invitations/" + id
	expect(t, owner.request("POST", "/api/v1/trusted-contacts/invitations", map[string]string{"email": recipient.email}), 409)
	_, err := db.Exec(context.Background(), `UPDATE trusted_contact_invitations SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	r := recipient.request("GET", "/api/v1/trusted-contacts/invitations", nil)
	if !strings.Contains(string(r.body), "EXPIRED") {
		t.Fatal("expiry not reflected without worker")
	}
	expect(t, recipient.request("POST", path+"/accept", map[string]string{"token": token}), 410)
	id, _ = invite(t, owner, recipient.email)
	path = "/api/v1/trusted-contacts/invitations/" + id
	expect(t, owner.request("POST", path+"/decline", map[string]any{}), 404)
	expect(t, recipient.request("POST", path+"/decline", map[string]any{}), 204)
	id, token = invite(t, owner, recipient.email)
	path = "/api/v1/trusted-contacts/invitations/" + id
	expect(t, owner.request("DELETE", path, nil), 204)
	expect(t, recipient.request("POST", path+"/accept", map[string]string{"token": token}), 409)
}

func TestHTTPBoundariesAndRateLimit(t *testing.T) {
	a := newActor(t)
	for _, body := range []string{`{"display_name":"x","unknown":true}`, `{"display_name":"x"} {}`, `not-json`, strings.Repeat(" ", 33<<10)} {
		req, _ := http.NewRequest("PATCH", app.URL+"/api/v1/users/me", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", a.csrf)
		res, err := a.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 400 && res.StatusCode != 413 {
			t.Fatalf("invalid JSON accepted: %d", res.StatusCode)
		}
	}
	req, _ := http.NewRequest("PATCH", app.URL+"/api/v1/users/me", strings.NewReader(`{"display_name":"x"}`))
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", a.csrf)
	res, err := a.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("untrusted origin accepted")
	}
	req, _ = http.NewRequest("POST", app.URL+"/api/v1/auth/login", strings.NewReader(`email=x`))
	req.Header.Set("Content-Type", "text/plain")
	res, err = a.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 415 {
		t.Fatal("simple cross-site payload accepted")
	}
	limited := cfg
	limited.AuthRatePerMinute = 2
	r, err := server.Router(limited, db, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", uuid.NewString())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if i == 2 && w.Code != 429 {
			t.Fatal("limiter trusted spoofed forwarded address")
		}
	}
}

func TestCredentialsStoredOnlyAsHashes(t *testing.T) {
	a := newActor(t)
	var password string
	var hash []byte
	if err := db.QueryRow(context.Background(), `SELECT password_hash FROM users WHERE id=$1`, a.id).Scan(&password); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(password, "$argon2id$") || strings.Contains(password, "test-password-long-123") {
		t.Fatal("password stored in plaintext")
	}
	if err := db.QueryRow(context.Background(), `SELECT token_hash FROM user_sessions WHERE user_id=$1`, a.id).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if len(hash) != 32 || string(hash) == a.token {
		t.Fatal("session stored in plaintext")
	}
}

func TestProductionRoleGuard(t *testing.T) {
	ctx := context.Background()
	if database.SafeApplicationRole(ctx, db) {
		t.Fatal("test administrator accepted as production app role")
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	role := "probe_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{role}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE ROLE "+quoted+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		conn.Exec(ctx, "RESET ROLE")
		if _, err := conn.Exec(ctx, "DROP ROLE "+quoted); err != nil {
			t.Error(err)
		}
	}()
	if _, err = conn.Exec(ctx, "SET ROLE "+quoted); err != nil {
		t.Fatal(err)
	}
	if !database.SafeApplicationRole(ctx, conn) {
		t.Fatal("non-admin role rejected")
	}
}
