package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
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
func writeRelations(ctx context.Context, tx *sql.Tx, state relationState, version uint64, previous *relationState) error {
	batch := func(name string, rows [][]any) error {
		if len(rows) == 0 {
			return nil
		}
		q := query(name)
		start := strings.Index(q, " VALUES (")
		if start < 0 {
			return errors.New("invalid insert template")
		}
		start += len(" VALUES ")
		end := strings.Index(q[start:], ")") + start + 1
		values := make([]string, len(rows))
		args := []any{}
		for i, row := range rows {
			values[i] = q[start:end]
			args = append(args, row...)
		}
		_, err := tx.ExecContext(ctx, q[:start]+strings.Join(values, ",")+q[end:], args...)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}
	oldEndpoints := map[string]endpointRecord{}
	oldRuntimes := map[string]runtimeRecord{}
	oldServices := map[string]serviceRecord{}
	knownUsers := map[string]bool{}
	if previous != nil {
		for _, e := range previous.Endpoints {
			oldEndpoints[e.ID] = e
			for _, m := range e.Members {
				knownUsers[m.UserID] = true
			}
		}
		for _, r := range previous.Runtimes {
			oldRuntimes[r.ID] = r
		}
		for _, s := range previous.Services {
			oldServices[s.ID] = s
			for _, m := range s.Members {
				knownUsers[m.UserID] = true
			}
		}
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
	ids := []string{}
	for id := range users {
		if !knownUsers[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	rows := [][]any{}
	for _, id := range ids {
		rows = append(rows, []any{id, id, id})
	}
	if err := batch("upsert_user.sql", rows); err != nil {
		return err
	}
	rows = [][]any{}
	for _, e := range state.Endpoints {
		if old, ok := oldEndpoints[e.ID]; ok && reflect.DeepEqual(old, e) {
			continue
		}
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
		rows = append(rows, []any{e.ID, e.Name, e.Address, addressType(e.Address), e.Region, provider, owner})
	}
	if err := batch("upsert_endpoint.sql", rows); err != nil {
		return err
	}
	rows = [][]any{}
	for _, r := range state.Runtimes {
		if old, ok := oldRuntimes[r.ID]; ok && reflect.DeepEqual(old, r) {
			continue
		}
		var address any
		for _, service := range state.Services {
			if service.RuntimeID == r.ID && r.ID == "vps" {
				for _, e := range state.Endpoints {
					if e.ID == service.EndpointID {
						address = e.Address
					}
				}
			}
		}
		rows = append(rows, []any{r.ID, r.Name, r.Location, address})
	}
	if err := batch("upsert_runtime.sql", rows); err != nil {
		return err
	}
	rows = [][]any{}
	for _, s := range state.Services {
		if old, ok := oldServices[s.ID]; ok && reflect.DeepEqual(old, s) {
			continue
		}
		config, e := json.Marshal(s.Config)
		if e != nil {
			return e
		}
		at, _ := time.Parse(time.RFC3339, s.UpdatedAt)
		rows = append(rows, []any{s.ID, s.EndpointID, s.RuntimeID, s.Kind, s.Name, s.Status, s.Version, s.Revision, config, at.UTC()})
	}
	if err := batch("upsert_service.sql", rows); err != nil {
		return err
	}
	endpointRows := [][]any{}
	serviceRows := [][]any{}
	if previous == nil {
		for _, table := range []string{"lisw_service_members", "lisw_endpoint_members"} {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
				return err
			}
		}
	}
	for _, e := range state.Endpoints {
		if old, ok := oldEndpoints[e.ID]; ok && reflect.DeepEqual(old.Members, e.Members) {
			continue
		}
		if previous != nil {
			if _, err := tx.ExecContext(ctx, "DELETE FROM lisw_endpoint_members WHERE endpoint_id=?", e.ID); err != nil {
				return err
			}
		}
		for _, m := range e.Members {
			endpointRows = append(endpointRows, []any{e.ID, m.UserID, m.Role})
		}
	}
	for _, s := range state.Services {
		if old, ok := oldServices[s.ID]; ok && reflect.DeepEqual(old.Members, s.Members) {
			continue
		}
		if previous != nil {
			if _, err := tx.ExecContext(ctx, "DELETE FROM lisw_service_members WHERE service_id=?", s.ID); err != nil {
				return err
			}
		}
		for _, m := range s.Members {
			serviceRows = append(serviceRows, []any{s.ID, m.UserID, m.Role})
		}
	}
	// Complete legacy backfill prunes stale rows. Normal saves prune only actual removals.
	prune := func(table string, ids []string, oldIDs map[string]bool) error {
		current := map[string]bool{}
		for _, id := range ids {
			current[id] = true
		}
		required := previous == nil
		for id := range oldIDs {
			if !current[id] {
				required = true
			}
		}
		if !required {
			return nil
		}
		q := "DELETE FROM " + table
		args := []any{}
		if len(ids) > 0 {
			q += " WHERE id NOT IN (" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")"
			for _, id := range ids {
				args = append(args, id)
			}
		}
		_, err := tx.ExecContext(ctx, q, args...)
		return err
	}
	serviceIDs := []string{}
	oldIDs := map[string]bool{}
	for _, s := range state.Services {
		serviceIDs = append(serviceIDs, s.ID)
	}
	for id := range oldServices {
		oldIDs[id] = true
	}
	if err := prune("lisw_services", serviceIDs, oldIDs); err != nil {
		return err
	}
	endpointIDs := []string{}
	oldIDs = map[string]bool{}
	for _, e := range state.Endpoints {
		endpointIDs = append(endpointIDs, e.ID)
	}
	for id := range oldEndpoints {
		oldIDs[id] = true
	}
	if err := prune("lisw_endpoints", endpointIDs, oldIDs); err != nil {
		return err
	}
	runtimeIDs := []string{}
	oldIDs = map[string]bool{}
	for _, r := range state.Runtimes {
		runtimeIDs = append(runtimeIDs, r.ID)
	}
	for id := range oldRuntimes {
		oldIDs[id] = true
	}
	if err := prune("lisw_runtimes", runtimeIDs, oldIDs); err != nil {
		return err
	}
	if err := batch("insert_endpoint_member.sql", endpointRows); err != nil {
		return err
	}
	if err := batch("insert_service_member.sql", serviceRows); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, query("upsert_relational_sync.sql"), version)
	return err
}
