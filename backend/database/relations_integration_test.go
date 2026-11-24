package database

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLiveRelationalMigration(t *testing.T) {
	if os.Getenv("LISW_DB_INTEGRATION") != "1" {
		t.Skip("requires explicit live database opt-in")
	}
	cfg, e := ConfigFromEnvironment()
	if e != nil || cfg.URL == "" {
		t.Fatal("database configuration required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	s, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	raw, e := s.Load(ctx)
	if e != nil {
		t.Fatal(e)
	}
	state, e := decodeRelations(raw)
	if e != nil {
		t.Fatal(e)
	}
	var username, globalRole, github string
	var demo bool
	if e = s.db.QueryRowContext(ctx, "SELECT username,platform_role,github_username,is_demo FROM lisw_users WHERE id='admin'").Scan(&username, &globalRole, &github, &demo); e != nil {
		t.Fatal(e)
	}
	if username != "snacking" || github != "snacking" || globalRole != "admin" || demo {
		t.Fatal("owner not correctly seeded")
	}
	var address, owner, provider string
	if e = s.db.QueryRowContext(ctx, "SELECT address,owner_user_id,provider FROM lisw_endpoints WHERE id='racknerd'").Scan(&address, &owner, &provider); e != nil {
		t.Fatal(e)
	}
	if address != "198.12.74.38" || owner != "admin" || provider != "RackNerd" {
		t.Fatal("incorrect VPS endpoint")
	}
	for _, service := range state.Services {
		var cfgRaw []byte
		var revision uint64
		var role string
		if e = s.db.QueryRowContext(ctx, "SELECT config,revision FROM lisw_services WHERE id=?", service.ID).Scan(&cfgRaw, &revision); e != nil {
			t.Fatal(e)
		}
		var config map[string]string
		json.Unmarshal(cfgRaw, &config)
		if !reflect.DeepEqual(config, service.Config) || revision != service.Revision {
			t.Fatal("service configuration not preserved")
		}
		if e = s.db.QueryRowContext(ctx, "SELECT role FROM lisw_service_members WHERE service_id=? AND user_id='admin'", service.ID).Scan(&role); e != nil || role != "admin" {
			t.Fatal("owner service membership missing")
		}
	}
	beforeVersion := s.version
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Load(ctx); e != nil {
		t.Fatal(e)
	}
	if s.version != beforeVersion {
		t.Fatal("idempotent migration changed revision")
	}
	// A relational constraint failure must roll back both snapshot and table writes.
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	var before []byte
	var version uint64
	if e = tx.QueryRowContext(ctx, query("select_state_for_update.sql")).Scan(&before, &version); e != nil {
		t.Fatal(e)
	}
	bad, e := decodeRelations(before)
	if e != nil {
		t.Fatal(e)
	}
	if len(bad.Services) == 0 {
		t.Fatal("service fixture missing")
	}
	bad.Services[0].Name = strings.Repeat("x", 129)
	if _, e = tx.ExecContext(ctx, query("update_state.sql"), before, version); e != nil {
		t.Fatal(e)
	}
	if e = writeRelations(ctx, tx, bad, version+1, nil); e == nil {
		t.Fatal("expected relational write failure")
	}
	if e = tx.Rollback(); e != nil {
		t.Fatal(e)
	}
	after, e := s.Load(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if string(after) != string(before) || s.version != version {
		t.Fatal("failed relational transaction changed snapshot")
	}
	t.Log("Owner, VPS, preserved services, memberships, idempotence and transaction rollback verified")
}
