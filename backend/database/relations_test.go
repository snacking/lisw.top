package database

import (
	"encoding/json"
	"testing"
)

func fixtureState() relationState {
	return relationState{Endpoints: []endpointRecord{{ID: "racknerd", Name: "VPS", Address: "198.12.74.38", Members: []memberRecord{{"admin", "admin"}}}}, Runtimes: []runtimeRecord{{ID: "vps", Name: "VPS"}}, Services: []serviceRecord{{ID: "hy2", EndpointID: "racknerd", RuntimeID: "vps", Kind: "hysteria2", Revision: 1, UpdatedAt: "2026-11-24T12:00:00Z", Members: []memberRecord{{"admin", "admin"}}}}}
}
func TestDecodeRelations(t *testing.T) {
	b, _ := json.Marshal(fixtureState())
	if _, e := decodeRelations(b); e != nil {
		t.Fatal(e)
	}
}
func TestRejectBrokenRelations(t *testing.T) {
	cases := []func(*relationState){func(s *relationState) { s.Services[0].EndpointID = "missing" }, func(s *relationState) { s.Services[0].RuntimeID = "missing" }, func(s *relationState) { s.Services[0].Revision = 0 }, func(s *relationState) { s.Endpoints[0].Members[0].Role = "owner" }, func(s *relationState) { s.Endpoints = append(s.Endpoints, s.Endpoints[0]) }, func(s *relationState) { s.Services[0].UpdatedAt = "invalid" }, func(s *relationState) {
		s.Endpoints[0].Members = append(s.Endpoints[0].Members, s.Endpoints[0].Members[0])
	}}
	for _, mutate := range cases {
		s := fixtureState()
		mutate(&s)
		b, _ := json.Marshal(s)
		if _, e := decodeRelations(b); e == nil {
			t.Fatal("accepted invalid relations")
		}
	}
}
func TestAddressType(t *testing.T) {
	for input, want := range map[string]string{"198.12.74.38": "ipv4", "2001:db8::1": "ipv6", "lisw.top": "hostname"} {
		if got := addressType(input); got != want {
			t.Fatalf("%s: %s", input, got)
		}
	}
}
