package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Membership struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}
type Endpoint struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Address string       `json:"address"`
	Region  string       `json:"region"`
	Members []Membership `json:"members"`
}
type Runtime struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
}
type Service struct {
	ID         string            `json:"id"`
	EndpointID string            `json:"endpointId"`
	RuntimeID  string            `json:"runtimeId"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	Version    string            `json:"version"`
	Revision   int               `json:"revision"`
	UpdatedAt  string            `json:"updatedAt"`
	Config     map[string]string `json:"config"`
	Members    []Membership      `json:"members"`
	Role       string            `json:"role"`
}
type Event struct {
	ID      int    `json:"id"`
	Service string `json:"service"`
	Action  string `json:"action"`
	At      string `json:"at"`
}

// Ports for durable storage, cache and registration mail. This build injects only memory adapters.
type Repository interface {
	Snapshot() ([]Endpoint, []Runtime, []Service, []Event)
}
type Cache interface {
	Set(context.Context, string, string) error
}
type Mailer interface {
	Send(context.Context, string, string) error
}
type MockDependencies struct{}

func (MockDependencies) Set(context.Context, string, string) error  { return nil }
func (MockDependencies) Send(context.Context, string, string) error { return nil }

type ServiceDriver interface {
	Validate(map[string]string) error
	Action(string) error
}
type MockDriver struct{ Kind string }

func (d MockDriver) Validate(c map[string]string) error {
	if c["host"] == "" || c["credential"] == "" {
		return fmt.Errorf("入口和凭据不能为空")
	}
	n, e := strconv.Atoi(c["port"])
	if e != nil || n < 1 || n > 65535 {
		return fmt.Errorf("端口需为 1–65535")
	}
	return nil
}
func (MockDriver) Action(string) error { return nil }

type Memory struct {
	sync.Mutex
	Endpoints   []Endpoint
	Runtimes    []Runtime
	Services    []Service
	Events      []Event
	Subscribers map[chan struct{}]bool
}

func (m *Memory) Snapshot() ([]Endpoint, []Runtime, []Service, []Event) {
	return m.Endpoints, m.Runtimes, m.Services, m.Events
}
func seed() *Memory {
	now := time.Now().UTC().Format(time.RFC3339)
	members := []Membership{{"admin", "admin"}, {"friend", "user"}}
	return &Memory{Endpoints: []Endpoint{{"racknerd", "RackNerd · US West", "198.12.74.38", "美国 · 洛杉矶", members}}, Runtimes: []Runtime{{"vps", "RackNerd VPS", "本机部署"}}, Services: []Service{{"hy2", "racknerd", "vps", "hysteria2", "Hysteria 2", "running", "v2 · mock", 1, now, map[string]string{"host": "198.12.74.38", "port": "443", "credential": "demo-hy2-friends-only", "sni": "example.com"}, members, ""}, {"vless", "racknerd", "vps", "vless", "VLESS · Reality", "running", "26.6.27 · mock", 1, now, map[string]string{"host": "198.12.74.38", "port": "8443", "credential": "00000000-0000-4000-8000-000000000001", "sni": "example.com", "publicKey": "DEMO_PUBLIC_KEY", "shortId": "0123456789abcdef", "flow": "xtls-rprx-vision"}, members, ""}}, Events: []Event{{1, "系统", "演示环境已就绪", now}}, Subscribers: map[chan struct{}]bool{}}
}

type App struct {
	Store    *Memory
	Drivers  map[string]ServiceDriver
	Cache    Cache
	Mail     Mailer
	Sessions sync.Map
}

func role(ms []Membership, id string) string {
	for _, m := range ms {
		if m.UserID == id {
			return m.Role
		}
	}
	return ""
}
func (a *App) serviceRole(s Service, id string) string {
	for _, e := range a.Store.Endpoints {
		if e.ID == s.EndpointID && role(e.Members, id) == "admin" {
			return "admin"
		}
	}
	return role(s.Members, id)
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"error": msg})
}
func (a *App) user(r *http.Request) string {
	c, e := r.Cookie("demo_session")
	if e != nil {
		return ""
	}
	v, ok := a.Sessions.Load(c.Value)
	if !ok {
		return ""
	}
	return v.(string)
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/api/health" {
		jsonOut(w, 200, map[string]string{"status": "ok", "mode": "mock"})
		return
	}
	if r.URL.Path == "/api/login" || r.URL.Path == "/api/register" {
		if r.Method != "POST" {
			fail(w, 405, "method")
			return
		}
		var body struct {
			Role  string `json:"role"`
			Email string `json:"email"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body) != nil {
			fail(w, 400, "无效 JSON")
			return
		}
		id := "friend"
		if body.Role == "admin" {
			id = "admin"
		}
		b := make([]byte, 32)
		if _, e := rand.Read(b); e != nil {
			fail(w, 500, "session")
			return
		}
		token := hex.EncodeToString(b)
		a.Sessions.Store(token, id)
		http.SetCookie(w, &http.Cookie{Name: "demo_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
		if r.URL.Path == "/api/register" {
			a.Mail.Send(r.Context(), body.Email, "mock welcome")
		}
		jsonOut(w, 200, map[string]string{"userId": id, "role": body.Role})
		return
	}
	id := a.user(r)
	if id == "" {
		fail(w, 401, "请先进入演示空间")
		return
	}
	if r.Method != "GET" {
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
			fail(w, 403, "origin")
			return
		}
	}
	if r.URL.Path == "/api/logout" && r.Method == "POST" {
		c, _ := r.Cookie("demo_session")
		a.Sessions.Delete(c.Value)
		http.SetCookie(w, &http.Cookie{Name: "demo_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		jsonOut(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.URL.Path == "/api/events" {
		if r.Method != "GET" {
			fail(w, 405, "method")
			return
		}
		f, ok := w.(http.Flusher)
		if !ok {
			fail(w, 500, "stream")
			return
		}
		ch := make(chan struct{}, 1)
		a.Store.Lock()
		a.Store.Subscribers[ch] = true
		a.Store.Unlock()
		defer func() { a.Store.Lock(); delete(a.Store.Subscribers, ch); a.Store.Unlock() }()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Accel-Buffering", "no")
		fmt.Fprint(w, "data: refresh\n\n")
		f.Flush()
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ch:
				fmt.Fprint(w, "data: refresh\n\n")
				f.Flush()
			case <-t.C:
				fmt.Fprint(w, ": heartbeat\n\n")
				f.Flush()
			}
		}
	}
	a.Store.Lock()
	defer a.Store.Unlock()
	if r.URL.Path == "/api/snapshot" && r.Method == "GET" {
		endpoints, runtimes, services, events := a.Store.Snapshot()
		es := []Endpoint{}
		ss := []Service{}
		rs := []Runtime{}
		ev := []Event{}
		visible := map[string]bool{}
		runtimeIDs := map[string]bool{}
		for _, s := range services {
			rr := a.serviceRole(s, id)
			if rr != "" {
				s.Role = rr
				ss = append(ss, s)
				visible[s.Name] = true
				runtimeIDs[s.RuntimeID] = true
			}
		}
		for _, e := range endpoints {
			if role(e.Members, id) != "" {
				es = append(es, e)
			}
		}
		for _, v := range runtimes {
			if runtimeIDs[v.ID] {
				rs = append(rs, v)
			}
		}
		for _, v := range events {
			if visible[v.Service] || v.Service == "系统" {
				ev = append(ev, v)
			}
		}
		jsonOut(w, 200, map[string]any{"endpoints": es, "runtimes": rs, "services": ss, "events": ev, "userId": id, "mode": "mock"})
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "services" && r.Method == "POST" {
		for i := range a.Store.Services {
			s := &a.Store.Services[i]
			if s.ID != parts[2] {
				continue
			}
			if a.serviceRole(*s, id) != "admin" {
				fail(w, 403, "仅服务管理员可以操作")
				return
			}
			action := parts[3]
			d := a.Drivers[s.Kind]
			if d == nil {
				fail(w, 422, "服务驱动尚未实现")
				return
			}
			switch action {
			case "start", "stop":
				if e := d.Action(action); e != nil {
					fail(w, 502, e.Error())
					return
				}
				s.Status = "running"
				if action == "stop" {
					s.Status = "stopped"
				}
			case "config":
				var b struct {
					Config   map[string]string `json:"config"`
					Revision int               `json:"revision"`
				}
				if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&b) != nil {
					fail(w, 400, "无效配置")
					return
				}
				if b.Revision != s.Revision {
					fail(w, 409, "配置已更新，请重新打开编辑器")
					return
				}
				if e := d.Validate(b.Config); e != nil {
					fail(w, 422, e.Error())
					return
				}
				s.Config = b.Config
			case "upgrade":
				s.Version = "mock-latest"
			default:
				fail(w, 404, "未知操作")
				return
			}
			s.Revision++
			s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			a.Cache.Set(r.Context(), s.ID, strconv.Itoa(s.Revision))
			a.Store.Events = append([]Event{{len(a.Store.Events) + 1, s.Name, action, s.UpdatedAt}}, a.Store.Events...)
			for c := range a.Store.Subscribers {
				select {
				case c <- struct{}{}:
				default:
				}
			}
			jsonOut(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	fail(w, 404, "未找到接口")
}
func main() {
	a := &App{Store: seed(), Drivers: map[string]ServiceDriver{"hysteria2": MockDriver{"hysteria2"}, "vless": MockDriver{"vless"}}, Cache: MockDependencies{}, Mail: MockDependencies{}}
	mux := http.NewServeMux()
	mux.Handle("/api/", a)
	dist := os.Getenv("STATIC_DIR")
	if dist == "" {
		dist = "../frontend/dist"
	}
	mux.Handle("/", http.FileServer(http.Dir(dist)))
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8081"
	}
	log.Printf("lisw mock listening on %s", addr)
	log.Fatal((&http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}).ListenAndServe())
}
