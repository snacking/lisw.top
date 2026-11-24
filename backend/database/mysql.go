// Package database owns MySQL configuration, verified TLS, migrations and DML.
package database

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	mysql "github.com/go-sql-driver/mysql"
	"net/url"
	"os"
	"strings"
	"time"
)

//go:embed sql/*.sql
var queries embed.FS
var ErrConflict = errors.New("database state changed")

type Config struct {
	URL    string `json:"url"`
	CAFile string `json:"caFile"`
}
type Store struct {
	db      *sql.DB
	version uint64
}

func ConfigFromEnvironment() (Config, error) {
	c := Config{URL: os.Getenv("DATABASE_URL"), CAFile: os.Getenv("DATABASE_CA_FILE")}
	if file := os.Getenv("DATABASE_CONFIG_FILE"); file != "" {
		b, e := os.ReadFile(file)
		if e != nil {
			return c, errors.New("cannot read database config file")
		}
		if e = json.Unmarshal(b, &c); e != nil {
			return c, errors.New("invalid database config JSON")
		}
	}
	return c, nil
}
func driverConfig(c Config) (*mysql.Config, error) {
	u, e := url.Parse(c.URL)
	if e != nil || u.Scheme != "mysql" || u.User == nil || u.Hostname() == "" || u.Port() == "" || strings.Trim(u.Path, "/") == "" {
		return nil, errors.New("invalid MySQL connection URL")
	}
	if c.CAFile == "" {
		return nil, errors.New("database CA file is required")
	}
	pem, e := os.ReadFile(c.CAFile)
	if e != nil {
		return nil, errors.New("cannot read database CA")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("invalid database CA certificate")
	}
	sum := sha256.Sum256(append(pem, []byte(u.Hostname())...))
	name := fmt.Sprintf("lisw-%x", sum[:8])
	if e = mysql.RegisterTLSConfig(name, &tls.Config{RootCAs: roots, ServerName: u.Hostname(), MinVersion: tls.VersionTLS12}); e != nil {
		return nil, errors.New("cannot configure database TLS")
	}
	cfg := mysql.NewConfig()
	cfg.User = u.User.Username()
	cfg.Passwd, _ = u.User.Password()
	cfg.Net = "tcp"
	cfg.Addr = u.Host
	cfg.DBName = strings.TrimPrefix(u.Path, "/")
	cfg.TLSConfig = name
	cfg.ParseTime = true
	cfg.Timeout = 10 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.Collation = "utf8mb4_unicode_ci"
	return cfg, nil
}
func Open(ctx context.Context, c Config) (*Store, error) {
	cfg, e := driverConfig(c)
	if e != nil {
		return nil, e
	}
	connector, e := mysql.NewConnector(cfg)
	if e != nil {
		return nil, errors.New("cannot create MySQL connector")
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(3 * time.Minute)
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, fmt.Errorf("database TLS connection failed: %w", e)
	}
	return &Store{db: db}, nil
}
func query(name string) string {
	b, e := queries.ReadFile("sql/" + name)
	if e != nil {
		panic(e)
	}
	return string(b)
}
func (s *Store) Migrate(ctx context.Context) error {
	for _, name := range []string{"001_init.sql", "002_users.sql", "003_endpoints.sql", "004_runtimes.sql", "005_services.sql", "006_endpoint_members.sql", "007_service_members.sql", "008_relational_sync.sql", "seed_owner.sql"} {
		if _, e := s.db.ExecContext(ctx, query(name)); e != nil {
			return fmt.Errorf("migration %s: %w", name, e)
		}
	}
	return nil
}
func (s *Store) Initialize(ctx context.Context, initial []byte) ([]byte, error) {
	if _, e := decodeRelations(initial); e != nil {
		return nil, e
	}
	if _, e := s.db.ExecContext(ctx, query("insert_state.sql"), initial); e != nil {
		return nil, e
	}
	return s.Load(ctx)
}
func (s *Store) Load(ctx context.Context) ([]byte, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	var b []byte
	var version uint64
	if e = tx.QueryRowContext(ctx, query("select_state_for_update.sql")).Scan(&b, &version); e != nil {
		return nil, e
	}
	var synced uint64
	e = tx.QueryRowContext(ctx, query("select_relational_sync.sql")).Scan(&synced)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	if synced != version {
		state, e := decodeRelations(b)
		if e != nil {
			return nil, e
		}
		if e = writeRelations(ctx, tx, state, version); e != nil {
			return nil, e
		}
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	s.version = version
	return b, nil
}
func (s *Store) Save(ctx context.Context, b []byte) error {
	state, e := decodeRelations(b)
	if e != nil {
		return e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	result, e := tx.ExecContext(ctx, query("update_state.sql"), b, s.version)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrConflict
	}
	if e = writeRelations(ctx, tx, state, s.version+1); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	s.version++
	return nil
}
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Store) Close() error                   { return s.db.Close() }
