package auth

import (
	"context"
	"errors"

	"github.com/aarondl/authboss/v3"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Storer implements authboss.ServerStorer and related interfaces
type Storer struct {
	users *models.UserStore
}

func NewStorer(users *models.UserStore) *Storer {
	return &Storer{users: users}
}

// ServerStorer interface
func (s *Storer) Load(ctx context.Context, key string) (authboss.User, error) {
	user, err := s.users.GetByEmail(ctx, key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authboss.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *Storer) Save(ctx context.Context, user authboss.User) error {
	u := user.(*models.User)
	return s.users.Update(ctx, u)
}

// CreatingServerStorer interface
func (s *Storer) New(ctx context.Context) authboss.User {
	return &models.User{
		ID:        uuid.New(),
		Confirmed: true, // Auto-confirm for now
	}
}

func (s *Storer) Create(ctx context.Context, user authboss.User) error {
	u := user.(*models.User)
	return s.users.Create(ctx, u)
}

// ConfirmingServerStorer interface
func (s *Storer) LoadByConfirmSelector(ctx context.Context, selector string) (authboss.ConfirmableUser, error) {
	user, err := s.users.GetByConfirmSelector(ctx, selector)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authboss.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// RecoveringServerStorer interface
func (s *Storer) LoadByRecoverSelector(ctx context.Context, selector string) (authboss.RecoverableUser, error) {
	user, err := s.users.GetByRecoverSelector(ctx, selector)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authboss.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}
