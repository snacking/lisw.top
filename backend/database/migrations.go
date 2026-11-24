package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// MySQL DDL auto-commits. One DDL statement per immutable file; record only
// after success. A dedicated connection owns the lock until all migrations finish.
func (s *Store) Migrate(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK('lisw_panel_schema',60)").Scan(&locked); err != nil || locked != 1 {
		return errors.New("cannot acquire schema migration lock")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('lisw_panel_schema')")
	var exists int
	if err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='lisw_schema_migrations'").Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		if _, err = conn.ExecContext(ctx, "CREATE TABLE lisw_schema_migrations (version VARCHAR(128) PRIMARY KEY, checksum CHAR(64) NOT NULL, adopted BOOLEAN NOT NULL DEFAULT FALSE, applied_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB"); err != nil {
			return err
		}
	}
	entries, err := queries.ReadDir("sql/ddl")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		raw, _ := queries.ReadFile("sql/ddl/" + entry.Name())
		sum := sha256.Sum256(raw)
		digest := hex.EncodeToString(sum[:])
		var applied string
		err = conn.QueryRowContext(ctx, "SELECT checksum FROM lisw_schema_migrations WHERE version=?", entry.Name()).Scan(&applied)
		if err == nil {
			if applied != digest {
				return fmt.Errorf("applied DDL checksum mismatch: %s", entry.Name())
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		// Adopt the pre-ledger tables from this project's earlier releases without re-executing DDL.
		fields := strings.Fields(string(raw))
		if len(fields) < 6 || strings.Join(fields[:5], " ") != "CREATE TABLE IF NOT EXISTS" {
			return errors.New("migration must contain one CREATE TABLE IF NOT EXISTS statement")
		}
		table := fields[5]
		if err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?", table).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			if _, err = conn.ExecContext(ctx, string(raw)); err != nil {
				return fmt.Errorf("DDL %s: %w", entry.Name(), err)
			}
		}
		if _, err = conn.ExecContext(ctx, "INSERT INTO lisw_schema_migrations(version,checksum,adopted) VALUES(?,?,?)", entry.Name(), digest, exists != 0); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) CheckSchema(ctx context.Context) error {
	entries, err := queries.ReadDir("sql/ddl")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		raw, _ := queries.ReadFile("sql/ddl/" + entry.Name())
		sum := sha256.Sum256(raw)
		var got string
		if err = s.db.QueryRowContext(ctx, "SELECT checksum FROM lisw_schema_migrations WHERE version=?", entry.Name()).Scan(&got); err != nil {
			return fmt.Errorf("schema not ready; run panel --migrate: %w", err)
		}
		if got != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("DDL checksum mismatch: %s", entry.Name())
		}
	}
	return nil
}
