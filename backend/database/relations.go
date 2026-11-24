package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

type memberRecord struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}
type endpointRecord struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Address string         `json:"address"`
	Region  string         `json:"region"`
	Members []memberRecord `json:"members"`
}
type runtimeRecord struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
}
type serviceRecord struct {
	ID         string            `json:"id"`
	EndpointID string            `json:"endpointId"`
	RuntimeID  string            `json:"runtimeId"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	Version    string            `json:"version"`
	Revision   uint64            `json:"revision"`
	UpdatedAt  string            `json:"updatedAt"`
	Config     map[string]string `json:"config"`
	Members    []memberRecord    `json:"members"`
}
type relationState struct {
	Endpoints []endpointRecord
	Runtimes  []runtimeRecord
	Services  []serviceRecord
}

func decodeRelations(raw []byte) (relationState, error) {
	var s relationState
	if e := json.Unmarshal(raw, &s); e != nil {
		return s, errors.New("invalid workspace JSON")
	}
	endpoints := map[string]bool{}
	runtimes := map[string]bool{}
	services := map[string]bool{}
	addresses := map[string]bool{}
	validMembers := func(ms []memberRecord) error {
		seen := map[string]bool{}
		for _, m := range ms {
			if m.UserID == "" || len(m.UserID) > 64 || (m.Role != "admin" && m.Role != "user") || seen[m.UserID] {
				return errors.New("invalid or duplicate membership")
			}
			seen[m.UserID] = true
		}
		return nil
	}
	for _, e := range s.Endpoints {
		if e.ID == "" || len(e.ID) > 64 || endpoints[e.ID] || e.Address == "" || addresses[strings.ToLower(e.Address)] {
			return s, errors.New("invalid or duplicate endpoint")
		}
		if err := validMembers(e.Members); err != nil {
			return s, err
		}
		endpoints[e.ID] = true
		addresses[strings.ToLower(e.Address)] = true
	}
	for _, r := range s.Runtimes {
		if r.ID == "" || len(r.ID) > 64 || runtimes[r.ID] {
			return s, errors.New("invalid or duplicate runtime")
		}
		runtimes[r.ID] = true
	}
	for _, v := range s.Services {
		if v.ID == "" || len(v.ID) > 64 || services[v.ID] || !endpoints[v.EndpointID] || !runtimes[v.RuntimeID] || v.Revision < 1 || v.Kind == "" {
			return s, errors.New("invalid service reference or revision")
		}
		if _, err := time.Parse(time.RFC3339, v.UpdatedAt); err != nil {
			return s, errors.New("invalid service timestamp")
		}
		if err := validMembers(v.Members); err != nil {
			return s, err
		}
		services[v.ID] = true
	}
	return s, nil
}
func addressType(address string) string {
	ip := net.ParseIP(address)
	if ip == nil {
		return "hostname"
	}
	if ip.To4() != nil {
		return "ipv4"
	}
	return "ipv6"
}

// Snapshot and relational records are written in one transaction. The snapshot remains
// the compatible API read model and retains the event history during migration.
func writeRelations(ctx context.Context, tx *sql.Tx, state relationState, version uint64) error {
	exec := func(name string, args ...any) error {
		_, e := tx.ExecContext(ctx, query(name), args...)
		if e != nil {
			return fmt.Errorf("%s: %w", name, e)
		}
		return nil
	}
	users := map[string]bool{}
	for _, e := range state.Endpoints {
		for _, m := range e.Members {
			users[m.UserID] = true
		}
	}
	for _, s := range state.Services {
		for _, m := range s.Members {
			users[m.UserID] = true
		}
	}
	ids := make([]string, 0, len(users))
	for id := range users {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := exec("upsert_user.sql", id, id, id); err != nil {
			return err
		}
	}
	for _, e := range state.Endpoints {
		owner := "admin"
		for _, m := range e.Members {
			if m.Role == "admin" {
				owner = m.UserID
				break
			}
		}
		provider := ""
		if e.ID == "racknerd" {
			provider = "RackNerd"
		}
		if err := exec("upsert_endpoint.sql", e.ID, e.Name, e.Address, addressType(e.Address), e.Region, provider, owner); err != nil {
			return err
		}
	}
	for _, r := range state.Runtimes {
		var address any
		for _, s := range state.Services {
			if s.RuntimeID == r.ID && r.ID == "vps" {
				for _, e := range state.Endpoints {
					if e.ID == s.EndpointID {
						address = e.Address
					}
				}
			}
		}
		if err := exec("upsert_runtime.sql", r.ID, r.Name, r.Location, address); err != nil {
			return err
		}
	}
	for _, s := range state.Services {
		config, e := json.Marshal(s.Config)
		if e != nil {
			return e
		}
		at, _ := time.Parse(time.RFC3339, s.UpdatedAt)
		if err := exec("upsert_service.sql", s.ID, s.EndpointID, s.RuntimeID, s.Kind, s.Name, s.Status, s.Version, s.Revision, config, at.UTC()); err != nil {
			return err
		}
	}
	if _, e := tx.ExecContext(ctx, "DELETE FROM lisw_service_members"); e != nil {
		return e
	}
	if _, e := tx.ExecContext(ctx, "DELETE FROM lisw_endpoint_members"); e != nil {
		return e
	}
	// Remove records omitted by the complete workspace snapshot; preserve user identities.
	deleteMissing := func(table string, ids []string) error {
		q := "DELETE FROM " + table
		args := []any{}
		if len(ids) > 0 {
			q += " WHERE id NOT IN (" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")"
			for _, id := range ids {
				args = append(args, id)
			}
		}
		_, e := tx.ExecContext(ctx, q, args...)
		return e
	}
	serviceIDs := []string{}
	for _, s := range state.Services {
		serviceIDs = append(serviceIDs, s.ID)
	}
	if e := deleteMissing("lisw_services", serviceIDs); e != nil {
		return e
	}
	endpointIDs := []string{}
	for _, e := range state.Endpoints {
		endpointIDs = append(endpointIDs, e.ID)
	}
	if e := deleteMissing("lisw_endpoints", endpointIDs); e != nil {
		return e
	}
	runtimeIDs := []string{}
	for _, r := range state.Runtimes {
		runtimeIDs = append(runtimeIDs, r.ID)
	}
	if e := deleteMissing("lisw_runtimes", runtimeIDs); e != nil {
		return e
	}
	for _, e := range state.Endpoints {
		for _, m := range e.Members {
			if err := exec("insert_endpoint_member.sql", e.ID, m.UserID, m.Role); err != nil {
				return err
			}
		}
	}
	for _, s := range state.Services {
		for _, m := range s.Members {
			if err := exec("insert_service_member.sql", s.ID, m.UserID, m.Role); err != nil {
				return err
			}
		}
	}
	return exec("upsert_relational_sync.sql", version)
}
