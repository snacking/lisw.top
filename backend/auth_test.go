package main

import (
	"context"
	"encoding/json"
	"lisw.top/panel/database"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeAuth struct{ disabled bool }

func (f *fakeAuth) Authenticate(_ context.Context, name, password string) (database.Identity, error) {
	if name != "snacking" || password != "test-password" || f.disabled {
		return database.Identity{}, database.ErrCredentials
	}
	return database.Identity{ID: "admin", Username: "snacking", Role: "admin"}, nil
}
func (f *fakeAuth) Lookup(_ context.Context, id string) (database.Identity, error) {
	if f.disabled {
		return database.Identity{}, database.ErrCredentials
	}
	return database.Identity{ID: id}, nil
}
func TestLoginValidation(t *testing.T) {
	a := testApp()
	a.Auth = &fakeAuth{}
	for _, body := range []string{`{}`, `{"role":"admin"}`, `{"username":"snacking","password":"wrong","role":"admin"}`, `{"username":"unknown","password":"test-password"}`} {
		w := request(a, "POST", "/api/login", body, "")
		if w.Code != 401 || len(w.Result().Cookies()) != 0 {
			t.Fatalf("invalid login accepted: %d", w.Code)
		}
	}
	w := request(a, "POST", "/api/login", `{"username":"snacking","password":"test-password","role":"user"}`, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["role"] != "admin" {
		t.Fatal("role not read from database")
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe session cookie")
	}
	if request(a, "GET", "/api/snapshot", "", cookie.Value).Code != 200 {
		t.Fatal("valid session rejected")
	}
	request(a, "POST", "/api/logout", "{}", cookie.Value)
	if request(a, "GET", "/api/snapshot", "", cookie.Value).Code != 401 {
		t.Fatal("logout did not invalidate session")
	}
}
func TestExpiredAndLegacySession(t *testing.T) {
	a := testApp()
	a.Sessions.Store("expired", Session{"admin", time.Now().Add(-time.Second)})
	if request(a, "GET", "/api/snapshot", "", "expired").Code != 401 {
		t.Fatal("expired session accepted")
	}
	a.Sessions.Store("legacy", "admin")
	if request(a, "GET", "/api/snapshot", "", "legacy").Code != 401 {
		t.Fatal("legacy session accepted")
	}
	if request(a, "POST", "/api/register", `{"role":"admin"}`, "").Code != 403 {
		t.Fatal("registration bypass")
	}
}
func TestLoginOriginAndRateLimit(t *testing.T) {
	a := testApp()
	a.Auth = &fakeAuth{}
	r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross origin login accepted")
	}
	for i := 0; i < 5; i++ {
		request(a, "POST", "/api/login", `{}`, "")
	}
	if request(a, "POST", "/api/login", `{}`, "").Code != 429 {
		t.Fatal("missing rate limit")
	}
}
func TestDisabledAccountSession(t *testing.T) {
	a := testApp()
	a.Auth = &fakeAuth{disabled: true}
	a.Sessions.Store("old", Session{"admin", time.Now().Add(time.Hour)})
	if request(a, "GET", "/api/snapshot", "", "old").Code != 401 {
		t.Fatal("disabled user accepted")
	}
}

func TestProxySecurity(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_IP", "172.29.44.2")
	r := httptest.NewRequest("POST", "/api/login", nil)
	r.Header.Set("X-Real-IP", "203.0.113.9")
	r.RemoteAddr = "198.51.100.1:1234"
	if clientIP(r) != "198.51.100.1" {
		t.Fatal("untrusted forwarding accepted")
	}
	r.RemoteAddr = "172.29.44.2:1234"
	if clientIP(r) != "203.0.113.9" {
		t.Fatal("trusted forwarding ignored")
	}
	r.Header.Set("X-Real-IP", "invalid")
	if clientIP(r) != "172.29.44.2" {
		t.Fatal("invalid address accepted")
	}
	t.Setenv("COOKIE_SECURE", "true")
	a := testApp()
	a.Auth = &fakeAuth{}
	w := request(a, "POST", "/api/login", `{"username":"snacking","password":"test-password"}`, "")
	if w.Code != 200 || !w.Result().Cookies()[0].Secure {
		t.Fatal("HTTPS proxy cookie lacks Secure")
	}
}
