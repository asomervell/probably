package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus string

const (
	SubscriptionStatusActive            SubscriptionStatus = "active"
	SubscriptionStatusCanceled          SubscriptionStatus = "canceled"
	SubscriptionStatusPastDue           SubscriptionStatus = "past_due"
	SubscriptionStatusTrialing          SubscriptionStatus = "trialing"
	SubscriptionStatusIncomplete        SubscriptionStatus = "incomplete"
	SubscriptionStatusIncompleteExpired SubscriptionStatus = "incomplete_expired"
	SubscriptionStatusUnpaid            SubscriptionStatus = "unpaid"
	SubscriptionStatusPaused            SubscriptionStatus = "paused"
)

// PlanType represents the type of subscription plan
type PlanType string

const (
	PlanTypeMonthly PlanType = "monthly"
	PlanTypeAnnual  PlanType = "annual"
	PlanTypeBundle  PlanType = "bundle"
)

// Subscription represents a user's subscription
type Subscription struct {
	ID                   uuid.UUID          `json:"id"`
	UserID               uuid.UUID          `json:"user_id"`
	StripeSubscriptionID *string            `json:"stripe_subscription_id,omitempty"`
	StripeCustomerID     *string            `json:"stripe_customer_id,omitempty"`
	Status               SubscriptionStatus `json:"status"`
	PlanType             PlanType           `json:"plan_type"`
	CurrentPeriodStart   *time.Time         `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     *time.Time         `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd    bool               `json:"cancel_at_period_end"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
}

// EntitySubscription represents the relationship between a subscription and an entity
type EntitySubscription struct {
	ID             uuid.UUID `json:"id"`
	SubscriptionID uuid.UUID `json:"subscription_id"`
	EntityID       uuid.UUID `json:"entity_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// scanSubscription scans a row into sub, handling nullable stripe and period fields.
func scanSubscription(row rowScanner, sub *Subscription) error {
	var stripeSubID, stripeCustID sql.NullString
	var periodStart, periodEnd sql.NullTime
	if err := row.Scan(&sub.ID, &sub.UserID, &stripeSubID, &stripeCustID, &sub.Status, &sub.PlanType,
		&periodStart, &periodEnd, &sub.CancelAtPeriodEnd, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
		return err
	}
	sub.StripeSubscriptionID = nullStringPtr(stripeSubID)
	sub.StripeCustomerID = nullStringPtr(stripeCustID)
	sub.CurrentPeriodStart = nullTimePtr(periodStart)
	sub.CurrentPeriodEnd = nullTimePtr(periodEnd)
	return nil
}

// SubscriptionStore handles database operations for subscriptions
type SubscriptionStore struct {
	pool *pgxpool.Pool
}

// NewSubscriptionStore creates a new SubscriptionStore
func NewSubscriptionStore(pool *pgxpool.Pool) *SubscriptionStore {
	return &SubscriptionStore{pool: pool}
}

// Create creates a new subscription
func (s *SubscriptionStore) Create(ctx context.Context, sub *Subscription) error {
	if sub.ID == uuid.Nil {
		sub.ID = uuid.New()
	}
	now := time.Now()
	sub.CreatedAt = now
	sub.UpdatedAt = now

	stripeSubID := toNullString(sub.StripeSubscriptionID)
	stripeCustID := toNullString(sub.StripeCustomerID)
	periodStart := toNullTime(sub.CurrentPeriodStart)
	periodEnd := toNullTime(sub.CurrentPeriodEnd)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO subscriptions (id, user_id, stripe_subscription_id, stripe_customer_id, status, plan_type, current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, sub.ID, sub.UserID, stripeSubID, stripeCustID, sub.Status, sub.PlanType, periodStart, periodEnd, sub.CancelAtPeriodEnd, sub.CreatedAt, sub.UpdatedAt)

	return err
}

// Update updates a subscription
func (s *SubscriptionStore) Update(ctx context.Context, sub *Subscription) error {
	sub.UpdatedAt = time.Now()

	stripeSubID := toNullString(sub.StripeSubscriptionID)
	stripeCustID := toNullString(sub.StripeCustomerID)
	periodStart := toNullTime(sub.CurrentPeriodStart)
	periodEnd := toNullTime(sub.CurrentPeriodEnd)

	_, err := s.pool.Exec(ctx, `
		UPDATE subscriptions 
		SET stripe_subscription_id = $2, stripe_customer_id = $3, status = $4, plan_type = $5, 
		    current_period_start = $6, current_period_end = $7, cancel_at_period_end = $8, updated_at = $9
		WHERE id = $1
	`, sub.ID, stripeSubID, stripeCustID, sub.Status, sub.PlanType, periodStart, periodEnd, sub.CancelAtPeriodEnd, sub.UpdatedAt)

	return err
}

// GetByID gets a subscription by ID
func (s *SubscriptionStore) GetByID(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	var sub Subscription
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, stripe_subscription_id, stripe_customer_id, status, plan_type,
		       current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at
		FROM subscriptions WHERE id = $1
	`, id)
	if err := scanSubscription(row, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetByStripeSubscriptionID gets a subscription by Stripe subscription ID
func (s *SubscriptionStore) GetByStripeSubscriptionID(ctx context.Context, stripeID string) (*Subscription, error) {
	var sub Subscription
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, stripe_subscription_id, stripe_customer_id, status, plan_type,
		       current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at
		FROM subscriptions WHERE stripe_subscription_id = $1
	`, stripeID)
	if err := scanSubscription(row, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetByStripeCustomerID gets all subscriptions for a Stripe customer ID
func (s *SubscriptionStore) GetByStripeCustomerID(ctx context.Context, stripeCustomerID string) ([]*Subscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, stripe_subscription_id, stripe_customer_id, status, plan_type, 
		       current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at
		FROM subscriptions WHERE stripe_customer_id = $1
		ORDER BY created_at DESC
	`, stripeCustomerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		var sub Subscription
		if err := scanSubscription(rows, &sub); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}

	return subs, rows.Err()
}

// GetByUserID gets all subscriptions for a user
func (s *SubscriptionStore) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*Subscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, stripe_subscription_id, stripe_customer_id, status, plan_type, 
		       current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at
		FROM subscriptions WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		var sub Subscription
		if err := scanSubscription(rows, &sub); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}

	return subs, rows.Err()
}

// GetActiveSubscriptions gets all active subscriptions for a user
func (s *SubscriptionStore) GetActiveSubscriptions(ctx context.Context, userID uuid.UUID) ([]*Subscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, stripe_subscription_id, stripe_customer_id, status, plan_type, 
		       current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at
		FROM subscriptions 
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		var sub Subscription
		if err := scanSubscription(rows, &sub); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}

	return subs, rows.Err()
}

// UpdateStatus updates the status of a subscription
func (s *SubscriptionStore) UpdateStatus(ctx context.Context, id uuid.UUID, status SubscriptionStatus) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE subscriptions SET status = $2, updated_at = $3 WHERE id = $1
	`, id, status, time.Now())
	return err
}

// Delete deletes a subscription
func (s *SubscriptionStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM subscriptions WHERE id = $1
	`, id)
	return err
}

// HasEverHadTrial checks if a user has ever had a subscription with trialing status
func (s *SubscriptionStore) HasEverHadTrial(ctx context.Context, userID uuid.UUID) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM subscriptions 
		WHERE user_id = $1 AND status = 'trialing'
	`, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasActiveSubscriptionOrTrial checks if a user has an active subscription or trial
func (s *SubscriptionStore) HasActiveSubscriptionOrTrial(ctx context.Context, userID uuid.UUID) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM subscriptions 
		WHERE user_id = $1 AND status IN ('active', 'trialing')
	`, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetActiveSubscriptionOrTrial gets the current active subscription or trial for a user
func (s *SubscriptionStore) GetActiveSubscriptionOrTrial(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	var sub Subscription
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, stripe_subscription_id, stripe_customer_id, status, plan_type,
		       current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at
		FROM subscriptions
		WHERE user_id = $1 AND status IN ('active', 'trialing')
		ORDER BY created_at DESC
		LIMIT 1
	`, userID)
	if err := scanSubscription(row, &sub); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

// EntitySubscriptionStore handles database operations for entity subscriptions
type EntitySubscriptionStore struct {
	pool *pgxpool.Pool
}

// NewEntitySubscriptionStore creates a new EntitySubscriptionStore
func NewEntitySubscriptionStore(pool *pgxpool.Pool) *EntitySubscriptionStore {
	return &EntitySubscriptionStore{pool: pool}
}

// Create creates a new entity subscription link
func (s *EntitySubscriptionStore) Create(ctx context.Context, es *EntitySubscription) error {
	if es.ID == uuid.Nil {
		es.ID = uuid.New()
	}
	es.CreatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO entity_subscriptions (id, subscription_id, entity_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (subscription_id, entity_id) DO NOTHING
	`, es.ID, es.SubscriptionID, es.EntityID, es.CreatedAt)

	return err
}

// GetBySubscriptionID gets all entities for a subscription
func (s *EntitySubscriptionStore) GetBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID) ([]*EntitySubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, subscription_id, entity_id, created_at
		FROM entity_subscriptions
		WHERE subscription_id = $1
		ORDER BY created_at ASC
	`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*EntitySubscription
	for rows.Next() {
		var es EntitySubscription
		if err := rows.Scan(&es.ID, &es.SubscriptionID, &es.EntityID, &es.CreatedAt); err != nil {
			return nil, err
		}
		entities = append(entities, &es)
	}

	return entities, rows.Err()
}

// GetByEntityID gets all subscriptions for an entity
func (s *EntitySubscriptionStore) GetByEntityID(ctx context.Context, entityID uuid.UUID) ([]*EntitySubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, subscription_id, entity_id, created_at
		FROM entity_subscriptions
		WHERE entity_id = $1
		ORDER BY created_at ASC
	`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []*EntitySubscription
	for rows.Next() {
		var es EntitySubscription
		if err := rows.Scan(&es.ID, &es.SubscriptionID, &es.EntityID, &es.CreatedAt); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, &es)
	}

	return subscriptions, rows.Err()
}

// Delete removes an entity subscription link
func (s *EntitySubscriptionStore) Delete(ctx context.Context, subscriptionID, entityID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM entity_subscriptions
		WHERE subscription_id = $1 AND entity_id = $2
	`, subscriptionID, entityID)
	return err
}

// DeleteBySubscriptionID removes all entity links for a subscription
func (s *EntitySubscriptionStore) DeleteBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM entity_subscriptions
		WHERE subscription_id = $1
	`, subscriptionID)
	return err
}
