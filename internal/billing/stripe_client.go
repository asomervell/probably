package billing

import (
	"context"
	"fmt"
	"strings"

	"log/slog"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	portalsession "github.com/stripe/stripe-go/v76/billingportal/session"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
)

// StripeClient wraps Stripe API operations
type StripeClient struct {
	cfg *config.Config
}

// NewStripeClient creates a new Stripe client
func NewStripeClient(cfg *config.Config) *StripeClient {
	if cfg.StripeSecretKey != "" {
		stripe.Key = cfg.StripeSecretKey
	}
	return &StripeClient{cfg: cfg}
}

// FindCustomerByEmail searches for an existing Stripe customer by email
func (c *StripeClient) FindCustomerByEmail(ctx context.Context, email string) (string, error) {
	params := &stripe.CustomerListParams{}
	params.Filters.AddFilter("email", "", email)
	params.Limit = stripe.Int64(1)

	customers := customer.List(params)
	if customers.Next() {
		return customers.Customer().ID, nil
	}
	if err := customers.Err(); err != nil {
		return "", fmt.Errorf("failed to search for customer: %w", err)
	}
	return "", nil // Not found
}

// GetCustomer retrieves a customer from Stripe by ID
// Returns an error if the customer doesn't exist or has been deleted
func (c *StripeClient) GetCustomer(ctx context.Context, customerID string) (*stripe.Customer, error) {
	cust, err := customer.Get(customerID, nil)
	if err != nil {
		// Log the actual error type for debugging
		if stripeErr, ok := err.(*stripe.Error); ok {
			slog.ErrorContext(ctx, "GetCustomer failed",
				"customer_id", customerID,
				"error_type", string(stripeErr.Type),
				"error_code", string(stripeErr.Code),
				"error_msg", stripeErr.Msg)
		} else {
			slog.ErrorContext(ctx, "GetCustomer failed (non-stripe error)",
				"customer_id", customerID,
				"error", err)
		}
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	// Check if customer was soft-deleted in Stripe
	// Stripe keeps deleted customers but marks them with Deleted=true
	if cust.Deleted {
		slog.WarnContext(ctx, "Customer is deleted", "customer_id", customerID)
		return nil, fmt.Errorf("customer %s has been deleted", customerID)
	}

	slog.DebugContext(ctx, "GetCustomer success",
		"customer_id", cust.ID,
		"email", cust.Email)
	return cust, nil
}

// GetOrCreateCustomer gets an existing Stripe customer by email or creates a new one
func (c *StripeClient) GetOrCreateCustomer(ctx context.Context, user *models.User) (string, error) {
	// First, try to find existing customer by email
	existingID, err := c.FindCustomerByEmail(ctx, user.Email)
	if err != nil {
		return "", err
	}
	if existingID != "" {
		return existingID, nil
	}

	// No existing customer found, create a new one
	params := &stripe.CustomerParams{
		Email: stripe.String(user.Email),
		Metadata: map[string]string{
			"user_id": user.ID.String(),
		},
	}

	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	return cust.ID, nil
}

// CreateCheckoutSession creates a Stripe Checkout session for subscription
func (c *StripeClient) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, customerID string, entityIDs []uuid.UUID, planType models.PlanType, successURL, cancelURL string) (string, error) {
	return c.CreateCheckoutSessionWithTrial(ctx, userID, customerID, entityIDs, planType, successURL, cancelURL, 0)
}

// CreateCheckoutSessionWithTrial creates a Stripe Checkout session for subscription with optional trial period
func (c *StripeClient) CreateCheckoutSessionWithTrial(ctx context.Context, userID uuid.UUID, customerID string, entityIDs []uuid.UUID, planType models.PlanType, successURL, cancelURL string, trialDays int64) (string, error) {
	var priceID string
	switch planType {
	case models.PlanTypeMonthly:
		priceID = c.cfg.StripePriceMonthly
	case models.PlanTypeAnnual:
		priceID = c.cfg.StripePriceAnnual
	case models.PlanTypeBundle:
		priceID = c.cfg.StripePriceBundle
	default:
		return "", fmt.Errorf("invalid plan type: %s", planType)
	}

	if priceID == "" {
		return "", fmt.Errorf("price ID not configured for plan type: %s", planType)
	}

	// Build metadata with entity IDs
	metadata := map[string]string{
		"user_id":    userID.String(),
		"plan_type":  string(planType),
		"entity_ids": "",
	}
	if len(entityIDs) > 0 {
		strs := make([]string, len(entityIDs))
		for i, eid := range entityIDs {
			strs[i] = eid.String()
		}
		metadata["entity_ids"] = strings.Join(strs, ",")
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Metadata:   metadata,
	}

	// Add trial period if specified
	if trialDays > 0 {
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(trialDays),
		}
	}

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create checkout session: %w", err)
	}

	return sess.URL, nil
}

// CreatePortalSession creates a Stripe Customer Portal session
func (c *StripeClient) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}

	portalSession, err := portalsession.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create portal session: %w", err)
	}

	return portalSession.URL, nil
}

// GetCheckoutSession retrieves a checkout session from Stripe
func (c *StripeClient) GetCheckoutSession(ctx context.Context, sessionID string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("subscription")
	sess, err := session.Get(sessionID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkout session: %w", err)
	}
	return sess, nil
}

// GetSubscription retrieves a subscription from Stripe
func (c *StripeClient) GetSubscription(ctx context.Context, stripeSubscriptionID string) (*stripe.Subscription, error) {
	s, err := subscription.Get(stripeSubscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return s, nil
}

// ParseSubscriptionStatus converts Stripe status to our model status
func ParseSubscriptionStatus(stripeStatus string) models.SubscriptionStatus {
	switch stripeStatus {
	case "active":
		return models.SubscriptionStatusActive
	case "canceled":
		return models.SubscriptionStatusCanceled
	case "past_due":
		return models.SubscriptionStatusPastDue
	case "trialing":
		return models.SubscriptionStatusTrialing
	case "incomplete":
		return models.SubscriptionStatusIncomplete
	case "incomplete_expired":
		return models.SubscriptionStatusIncompleteExpired
	case "unpaid":
		return models.SubscriptionStatusUnpaid
	case "paused":
		return models.SubscriptionStatusPaused
	default:
		return models.SubscriptionStatusCanceled
	}
}

