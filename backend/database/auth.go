package database

import (
	"context"
	"database/sql"
	"errors"
)

var ErrCredentials = errors.New("invalid credentials")

type Identity struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

func (s *Store) Authenticate(ctx context.Context, login, password string) (Identity, error) {
	var u Identity
	var hash string
	err := s.db.QueryRowContext(ctx, query("authenticate_user.sql"), login, login).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrCredentials
	}
	if err != nil {
		return u, err
	}
	if !CheckPassword(hash, password) {
		return Identity{}, ErrCredentials
	}
	return u, nil
}
func (s *Store) Lookup(ctx context.Context, id string) (Identity, error) {
	var u Identity
	err := s.db.QueryRowContext(ctx, query("session_user.sql"), id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrCredentials
	}
	return u, err
}
