package models

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID       uuid.UUID `json:"id"`
	Email    string    `json:"email"`
	Password string    `json:"-"`

	// Authboss fields
	Confirmed          bool       `json:"confirmed"`
	ConfirmSelector    string     `json:"-"`
	ConfirmVerifier    string     `json:"-"`
	RecoverSelector    string     `json:"-"`
	RecoverVerifier    string     `json:"-"`
	RecoverTokenExpiry *time.Time `json:"-"`
	Locked             bool       `json:"locked"`
	LockReason         string     `json:"-"`
	AttemptCount       int        `json:"-"`
	LastAttempt        *time.Time `json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Authboss interface implementations

func (u *User) GetPID() string {
	return u.Email
}

func (u *User) PutPID(pid string) {
	u.Email = pid
}

func (u *User) GetPassword() string {
	return u.Password
}

func (u *User) PutPassword(password string) {
	u.Password = password
}

func (u *User) GetEmail() string {
	return u.Email
}

func (u *User) PutEmail(email string) {
	u.Email = email
}

func (u *User) GetConfirmed() bool {
	return u.Confirmed
}

func (u *User) PutConfirmed(confirmed bool) {
	u.Confirmed = confirmed
}

func (u *User) GetConfirmSelector() string {
	return u.ConfirmSelector
}

func (u *User) PutConfirmSelector(selector string) {
	u.ConfirmSelector = selector
}

func (u *User) GetConfirmVerifier() string {
	return u.ConfirmVerifier
}

func (u *User) PutConfirmVerifier(verifier string) {
	u.ConfirmVerifier = verifier
}

func (u *User) GetRecoverSelector() string {
	return u.RecoverSelector
}

func (u *User) PutRecoverSelector(selector string) {
	u.RecoverSelector = selector
}

func (u *User) GetRecoverVerifier() string {
	return u.RecoverVerifier
}

func (u *User) PutRecoverVerifier(verifier string) {
	u.RecoverVerifier = verifier
}

func (u *User) GetRecoverExpiry() time.Time {
	if u.RecoverTokenExpiry == nil {
		return time.Time{}
	}
	return *u.RecoverTokenExpiry
}

func (u *User) PutRecoverExpiry(expiry time.Time) {
	u.RecoverTokenExpiry = &expiry
}

func (u *User) GetLocked() time.Time {
	if u.Locked {
		return time.Now().Add(time.Hour * 24 * 365 * 100) // Far future
	}
	return time.Time{}
}

func (u *User) PutLocked(locked time.Time) {
	u.Locked = !locked.IsZero()
}

func (u *User) GetAttemptCount() int {
	return u.AttemptCount
}

func (u *User) PutAttemptCount(count int) {
	u.AttemptCount = count
}

func (u *User) GetLastAttempt() time.Time {
	if u.LastAttempt == nil {
		return time.Time{}
	}
	return *u.LastAttempt
}

func (u *User) PutLastAttempt(attempt time.Time) {
	u.LastAttempt = &attempt
}

// UserStore handles user persistence
type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

func (s *UserStore) Create(ctx context.Context, user *User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, email, password, confirmed, confirm_selector, confirm_verifier,
			recover_selector, recover_verifier, recover_token_expiry, locked, lock_reason,
			attempt_count, last_attempt, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, user.ID, user.Email, user.Password, user.Confirmed, NullString(user.ConfirmSelector),
		NullString(user.ConfirmVerifier), NullString(user.RecoverSelector), NullString(user.RecoverVerifier),
		user.RecoverTokenExpiry, user.Locked, NullString(user.LockReason), user.AttemptCount,
		user.LastAttempt, time.Now(), time.Now())

	return err
}

func (s *UserStore) Update(ctx context.Context, user *User) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET
			email = $2, password = $3, confirmed = $4, confirm_selector = $5, confirm_verifier = $6,
			recover_selector = $7, recover_verifier = $8, recover_token_expiry = $9, locked = $10,
			lock_reason = $11, attempt_count = $12, last_attempt = $13, updated_at = $14
		WHERE id = $1
	`, user.ID, user.Email, user.Password, user.Confirmed, NullString(user.ConfirmSelector),
		NullString(user.ConfirmVerifier), NullString(user.RecoverSelector), NullString(user.RecoverVerifier),
		user.RecoverTokenExpiry, user.Locked, NullString(user.LockReason), user.AttemptCount,
		user.LastAttempt, time.Now())

	return err
}

func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.scanUser(s.pool.QueryRow(ctx, `
		SELECT id, email, password, confirmed, confirm_selector, confirm_verifier,
			recover_selector, recover_verifier, recover_token_expiry, locked, lock_reason,
			attempt_count, last_attempt, created_at, updated_at
		FROM users WHERE id = $1
	`, id))
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	return s.scanUser(s.pool.QueryRow(ctx, `
		SELECT id, email, password, confirmed, confirm_selector, confirm_verifier,
			recover_selector, recover_verifier, recover_token_expiry, locked, lock_reason,
			attempt_count, last_attempt, created_at, updated_at
		FROM users WHERE email = $1
	`, email))
}

func (s *UserStore) GetByConfirmSelector(ctx context.Context, selector string) (*User, error) {
	return s.scanUser(s.pool.QueryRow(ctx, `
		SELECT id, email, password, confirmed, confirm_selector, confirm_verifier,
			recover_selector, recover_verifier, recover_token_expiry, locked, lock_reason,
			attempt_count, last_attempt, created_at, updated_at
		FROM users WHERE confirm_selector = $1
	`, selector))
}

func (s *UserStore) GetByRecoverSelector(ctx context.Context, selector string) (*User, error) {
	return s.scanUser(s.pool.QueryRow(ctx, `
		SELECT id, email, password, confirmed, confirm_selector, confirm_verifier,
			recover_selector, recover_verifier, recover_token_expiry, locked, lock_reason,
			attempt_count, last_attempt, created_at, updated_at
		FROM users WHERE recover_selector = $1
	`, selector))
}

func (s *UserStore) Delete(ctx context.Context, id uuid.UUID) error {
	// Cascade delete will handle ledgers, accounts, transactions, etc.
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (s *UserStore) scanUser(row interface{ Scan(dest ...any) error }) (*User, error) {
	var u User
	var confirmSelector, confirmVerifier, recoverSelector, recoverVerifier, lockReason sql.NullString

	err := row.Scan(
		&u.ID, &u.Email, &u.Password, &u.Confirmed, &confirmSelector, &confirmVerifier,
		&recoverSelector, &recoverVerifier, &u.RecoverTokenExpiry, &u.Locked, &lockReason,
		&u.AttemptCount, &u.LastAttempt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	u.ConfirmSelector = confirmSelector.String
	u.ConfirmVerifier = confirmVerifier.String
	u.RecoverSelector = recoverSelector.String
	u.RecoverVerifier = recoverVerifier.String
	u.LockReason = lockReason.String

	return &u, nil
}

