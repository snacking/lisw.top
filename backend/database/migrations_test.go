package database

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMySQLMigrations(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable MySQL database")
	}
	db, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	s := &Store{db: db}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Migrate(ctx) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if e = s.CheckSchema(ctx); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM lisw_schema_migrations").Scan(&count); e != nil || count != 8 {
		t.Fatalf("migration count %d: %v", count, e)
	}
	if e = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM lisw_users").Scan(&count); e != nil || count != 0 {
		t.Fatal("DDL seeded user data")
	}
	var before, after string
	db.QueryRowContext(ctx, "SELECT CAST(MAX(applied_at) AS CHAR) FROM lisw_schema_migrations").Scan(&before)
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	db.QueryRowContext(ctx, "SELECT CAST(MAX(applied_at) AS CHAR) FROM lisw_schema_migrations").Scan(&after)
	if before != after {
		t.Fatal("migration replayed")
	}
	if _, e = db.ExecContext(ctx, "UPDATE lisw_schema_migrations SET checksum=REPEAT('0',64) WHERE version='001_init.sql'"); e != nil {
		t.Fatal(e)
	}
	if s.CheckSchema(ctx) == nil || s.Migrate(ctx) == nil {
		t.Fatal("accepted modified applied DDL")
	}
}
