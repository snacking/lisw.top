package main

import (
	"bufio"
	"encoding/json"
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
		r.AddCookie(&http.Cookie{Name: "demo_session", Value: token})
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestPermissionsAndConfigConflict(t *testing.T) {
	a := testApp()
	a.Sessions.Store("friend", "friend")
	a.Sessions.Store("admin", "admin")
	a.Sessions.Store("outsider", "outsider")
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
func TestNoValidationDemoLogin(t *testing.T) {
	a := testApp()
	w := request(a, "POST", "/api/register", `{}`, "")
	if w.Code != 200 || len(w.Result().Cookies()) != 1 {
		t.Fatal("demo registration should accept blank input")
	}
}
func TestServiceSpecificAdmin(t *testing.T) {
	a := testApp()
	a.Store.Services[0].Members = append(a.Store.Services[0].Members, Membership{"maintainer", "admin"})
	a.Sessions.Store("m", "maintainer")
	if request(a, "POST", "/api/services/hy2/stop", "{}", "m").Code != 200 {
		t.Fatal("service admin denied")
	}
	if request(a, "POST", "/api/services/vless/stop", "{}", "m").Code != 403 {
		t.Fatal("cross service access")
	}
}

func TestSSERefreshAfterMutation(t *testing.T) {
	a := testApp()
	a.Sessions.Store("friend", "friend")
	a.Sessions.Store("admin", "admin")
	srv := httptest.NewServer(a)
	defer srv.Close()
	r, _ := http.NewRequest("GET", srv.URL+"/api/events", nil)
	r.AddCookie(&http.Cookie{Name: "demo_session", Value: "friend"})
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
