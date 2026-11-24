package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"lisw.top/panel/database"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Authenticator interface {
	Authenticate(context.Context, string, string) (database.Identity, error)
	Lookup(context.Context, string) (database.Identity, error)
}
type Session struct {
	UserID  string
	Expires time.Time
}
type loginAttempt struct {
	Count int
	Until time.Time
}
type LoginLimiter struct {
	sync.Mutex
	Attempts map[string]loginAttempt
}

func (l *LoginLimiter) Allow(key string) bool {
	l.Lock()
	defer l.Unlock()
	now := time.Now()
	if l.Attempts == nil {
		l.Attempts = map[string]loginAttempt{}
	}
	for k, a := range l.Attempts {
		if now.After(a.Until) {
			delete(l.Attempts, k)
		}
	}
	a := l.Attempts[key]
	if a.Count >= 5 || len(l.Attempts) >= 5000 {
		return false
	}
	if a.Count == 0 {
		a.Until = now.Add(time.Minute)
	}
	a.Count++
	l.Attempts[key] = a
	return true
}
func validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || origin == "http://"+r.Host || origin == "https://"+r.Host
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method")
		return
	}
	if !validOrigin(r) {
		fail(w, 403, "origin")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.LoginLimiter.Allow(ip) {
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "尝试次数过多，请稍后再试")
		return
	}
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body) != nil {
		fail(w, 400, "无效 JSON")
		return
	}
	name := strings.TrimSpace(body.Username)
	if name == "" {
		name = strings.TrimSpace(body.Email)
	}
	if name == "" || body.Password == "" || len(name) > 254 || len(body.Password) > 1024 {
		fail(w, 401, "账号或密码错误")
		return
	}
	if a.Auth == nil {
		fail(w, 503, "登录服务不可用")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	u, err := a.Auth.Authenticate(ctx, name, body.Password)
	if errors.Is(err, database.ErrCredentials) {
		fail(w, 401, "账号或密码错误")
		return
	}
	if err != nil {
		fail(w, 503, "登录服务暂时不可用")
		return
	}
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		fail(w, 500, "session")
		return
	}
	token := hex.EncodeToString(b)
	if old, e := r.Cookie("panel_session"); e == nil {
		a.Sessions.Delete(old.Value)
	}
	a.Sessions.Range(func(k, v any) bool {
		session, ok := v.(Session)
		if !ok || time.Now().After(session.Expires) {
			a.Sessions.Delete(k)
		}
		return true
	})
	expiry := time.Now().Add(8 * time.Hour)
	a.Sessions.Store(token, Session{u.ID, expiry})
	http.SetCookie(w, &http.Cookie{Name: "panel_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, Expires: expiry, MaxAge: 8 * 60 * 60})
	jsonOut(w, 200, map[string]any{"userId": u.ID, "role": u.Role, "user": u})
}
