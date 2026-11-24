package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testApp() *App {
	return &App{Store: seed(), Drivers: map[string]ServiceDriver{"hysteria2": MockDriver{}, "vless": MockDriver{}}, Cache: MockDependencies{}, Mail: MockDependencies{}}
}
func request(a *App, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.AddCookie(&http.Cookie{Name: "panel_session", Value: token})
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestPermissionsAndConfigConflict(t *testing.T) {
	a := testApp()
	a.Sessions.Store("friend", Session{"friend", time.Now().Add(time.Hour)})
	a.Sessions.Store("admin", Session{"admin", time.Now().Add(time.Hour)})
	a.Sessions.Store("outsider", Session{"outsider", time.Now().Add(time.Hour)})
	for _, c := range []struct {
		method, path, body, token string
		status                    int
	}{{"GET", "/api/snapshot", "", "", 401}, {"POST", "/api/services/hy2/stop", "{}", "friend", 403}, {"POST", "/api/services/hy2/stop", "{}", "outsider", 403}, {"POST", "/api/services/hy2/stop", "{}", "admin", 200}, {"POST", "/api/services/hy2/config", `{"revision":1,"config":{"host":"demo","port":"443","credential":"demo"}}`, "admin", 409}, {"POST", "/api/services/hy2/config", `{"revision":2,"config":{"host":"demo","port":"0","credential":"demo"}}`, "admin", 422}, {"POST", "/api/services/hy2/config", `{"revision":2,"config":{"host":"demo","port":"443","credential":"new-demo"}}`, "admin", 200}} {
		if w := request(a, c.method, c.path, c.body, c.token); w.Code != c.status {
			t.Fatalf("%s %s: got %d want %d: %s", c.method, c.path, w.Code, c.status, w.Body.String())
		}
	}
	w := request(a, "GET", "/api/snapshot", "", "friend")
	if !strings.Contains(w.Body.String(), "new-demo") {
		t.Fatal("friend did not see updated credential")
	}
	w = request(a, "GET", "/api/snapshot", "", "outsider")
	var v struct {
		Services []Service `json:"services"`
	}
	json.Unmarshal(w.Body.Bytes(), &v)
	if len(v.Services) != 0 {
		t.Fatal("leaked services")
	}
}
func TestServiceSpecificAdmin(t *testing.T) {
	a := testApp()
	a.Store.Services[0].Members = append(a.Store.Services[0].Members, Membership{"maintainer", "admin"})
	a.Sessions.Store("m", Session{"maintainer", time.Now().Add(time.Hour)})
	if request(a, "POST", "/api/services/hy2/stop", "{}", "m").Code != 200 {
		t.Fatal("service admin denied")
	}
	if request(a, "POST", "/api/services/vless/stop", "{}", "m").Code != 403 {
		t.Fatal("cross service access")
	}
}

func TestSSERefreshAfterMutation(t *testing.T) {
	a := testApp()
	a.Sessions.Store("friend", Session{"friend", time.Now().Add(time.Hour)})
	a.Sessions.Store("admin", Session{"admin", time.Now().Add(time.Hour)})
	srv := httptest.NewServer(a)
	defer srv.Close()
	r, _ := http.NewRequest("GET", srv.URL+"/api/events", nil)
	r.AddCookie(&http.Cookie{Name: "panel_session", Value: "friend"})
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	if err != nil || line != "data: refresh\n" {
		t.Fatalf("initial SSE: %q %v", line, err)
	}
	reader.ReadString('\n')
	if request(a, "POST", "/api/services/hy2/stop", "{}", "admin").Code != 200 {
		t.Fatal("mutation failed")
	}
	line, err = reader.ReadString('\n')
	if err != nil || line != "data: refresh\n" {
		t.Fatalf("update SSE: %q %v", line, err)
	}
}

type fakePersistence struct {
	raw  []byte
	fail bool
}

func (f *fakePersistence) Save(_ context.Context, b []byte) error {
	if f.fail {
		return errors.New("database offline")
	}
	f.raw = b
	return nil
}
func (f *fakePersistence) Load(context.Context) ([]byte, error) { return f.raw, nil }
func TestDatabaseFailureRollback(t *testing.T) {
	a := testApp()
	initial := a.Store.encode()
	a.Persist = &fakePersistence{raw: initial, fail: true}
	a.Sessions.Store("a", Session{"admin", time.Now().Add(time.Hour)})
	if w := request(a, "POST", "/api/services/hy2/stop", "{}", "a"); w.Code != 503 {
		t.Fatalf("got %d", w.Code)
	}
	if string(a.Store.encode()) != string(initial) {
		t.Fatal("failed write changed memory")
	}
}
func TestCrossInstanceRefresh(t *testing.T) {
	a := testApp()
	other := seed()
	other.Services[0].Revision = 7
	other.Services[0].Config["credential"] = "demo-other-instance"
	a.Persist = &fakePersistence{raw: other.encode()}
	ch := make(chan struct{}, 1)
	a.Store.Subscribers[ch] = true
	if e := a.syncDatabase(context.Background()); e != nil {
		t.Fatal(e)
	}
	if a.Store.Services[0].Revision != 7 {
		t.Fatal("state not refreshed")
	}
	select {
	case <-ch:
	default:
		t.Fatal("no SSE refresh")
	}
}
