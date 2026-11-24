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
	_, e := s.db.ExecContext(ctx, query("001_init.sql"))
	return e
}
func (s *Store) Initialize(ctx context.Context, initial []byte) ([]byte, error) {
	if !json.Valid(initial) {
		return nil, errors.New("invalid seed JSON")
	}
	if _, e := s.db.ExecContext(ctx, query("insert_state.sql"), initial); e != nil {
		return nil, e
	}
	return s.Load(ctx)
}
func (s *Store) Load(ctx context.Context) ([]byte, error) {
	var b []byte
	var version uint64
	e := s.db.QueryRowContext(ctx, query("select_state.sql")).Scan(&b, &version)
	if e == nil {
		s.version = version
	}
	return b, e
}
func (s *Store) Save(ctx context.Context, b []byte) error {
	if !json.Valid(b) {
		return errors.New("invalid state JSON")
	}
	result, e := s.db.ExecContext(ctx, query("update_state.sql"), b, s.version)
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
	s.version++
	return nil
}
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Store) Close() error                   { return s.db.Close() }
