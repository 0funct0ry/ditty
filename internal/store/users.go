package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// Role values (SPEC.md §9): operator may write when -w is set, viewer never
// writes even under -w.
const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
)

// ErrUserExists is returned by CreateUser when username is already taken.
var ErrUserExists = errors.New("store: user already exists")

// ErrUserNotFound is returned by SetPassword/SetDisabled when no such user
// exists.
var ErrUserNotFound = errors.New("store: user not found")

// User is a users row, minus password_hash — no exported accessor ever
// carries a hash, so no caller can accidentally log or serialize one
// (VerifyPassword is the only path that touches it, internally).
type User struct {
	ID          string
	Username    string
	Role        string
	Disabled    bool
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

func newULID() (string, error) {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(time.Now()), entropy)
	if err != nil {
		return "", fmt.Errorf("store: generate id: %w", err)
	}
	return id.String(), nil
}

// CreateUser hashes password with bcrypt (cost 12) and inserts a new user
// row. role must be RoleViewer or RoleOperator.
func (s *Store) CreateUser(username, password, role string) (User, error) {
	if role != RoleViewer && role != RoleOperator {
		return User{}, fmt.Errorf("store: role must be %q or %q, got %q", RoleViewer, RoleOperator, role)
	}
	id, err := newULID()
	if err != nil {
		return User{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("store: hash password: %w", err)
	}
	createdAt := time.Now().UTC()

	_, err = s.db.Exec(
		`INSERT INTO users (id, username, password_hash, role, disabled, created_at) VALUES (?, ?, ?, ?, 0, ?)`,
		id, username, hash, role, createdAt.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return User{}, ErrUserExists
		}
		return User{}, fmt.Errorf("store: create user: %w", err)
	}
	return User{ID: id, Username: username, Role: role, CreatedAt: createdAt}, nil
}

// GetUserByUsername looks up a user by username (case-insensitive, per the
// schema's COLLATE NOCASE), reporting found=false rather than a sentinel
// error when there is no such user.
func (s *Store) GetUserByUsername(username string) (User, bool, error) {
	row := s.db.QueryRow(
		`SELECT id, username, role, disabled, created_at, last_login_at FROM users WHERE username = ?`, username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("store: get user: %w", err)
	}
	return u, true, nil
}

// ListUsers returns every user, ordered by username.
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, role, disabled, created_at, last_login_at FROM users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("store: list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// SetPassword re-hashes and stores a new password for username.
func (s *Store) SetPassword(username, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("store: hash password: %w", err)
	}
	res, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE username = ?`, hash, username)
	if err != nil {
		return fmt.Errorf("store: set password: %w", err)
	}
	return requireRowAffected(res)
}

// SetDisabled enables or disables username's account; a disabled user can
// never authenticate.
func (s *Store) SetDisabled(username string, disabled bool) error {
	res, err := s.db.Exec(`UPDATE users SET disabled = ? WHERE username = ?`, boolToInt(disabled), username)
	if err != nil {
		return fmt.Errorf("store: set disabled: %w", err)
	}
	return requireRowAffected(res)
}

// TouchLastLogin records the current time as username's last_login_at.
func (s *Store) TouchLastLogin(username string) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE username = ?`,
		time.Now().UTC().Format(time.RFC3339), username)
	if err != nil {
		return fmt.Errorf("store: touch last login: %w", err)
	}
	return nil
}

// VerifyPassword is the only path that touches password_hash: it looks up
// username, checks password against the stored bcrypt hash, and reports
// (User{}, false, nil) for a wrong password, a disabled account, or no such
// user — indistinguishable to the caller, so a login handler can't leak
// which case applies.
func (s *Store) VerifyPassword(username, password string) (User, bool, error) {
	var hash string
	var disabled int
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, role, disabled, created_at, last_login_at FROM users WHERE username = ?`, username)
	var u User
	var createdAt string
	var lastLogin sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &hash, &u.Role, &disabled, &createdAt, &lastLogin); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, false, nil
		}
		return User{}, false, fmt.Errorf("store: verify password: %w", err)
	}
	if disabled != 0 {
		return User{}, false, nil
	}
	if !passwordMatches(hash, password) {
		return User{}, false, nil
	}
	u.Disabled = false
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastLogin.Valid {
		t, _ := time.Parse(time.RFC3339, lastLogin.String)
		u.LastLoginAt = &t
	}
	return u, true, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var u User
	var disabled int
	var createdAt string
	var lastLogin sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.Role, &disabled, &createdAt, &lastLogin); err != nil {
		return User{}, err
	}
	u.Disabled = disabled != 0
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastLogin.Valid {
		t, _ := time.Parse(time.RFC3339, lastLogin.String)
		u.LastLoginAt = &t
	}
	return u, nil
}

func requireRowAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: rows affected: %w", err)
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
