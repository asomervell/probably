package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/asomervell/probably/internal/billing"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
)

// StripeWebhook handles Stripe webhook events
func (hdl *Handlers) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	// If billing is disabled, acknowledge but don't process
	if !hdl.cfg.BillingEnabled {
		slog.DebugContext(r.Context(), "Billing disabled, ignoring webhook")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","note":"billing_disabled"}`))
		return
	}

	// Read the raw body for signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to read body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify webhook signature
	if hdl.cfg.StripeWebhookSecret != "" {
		sigHeader := r.Header.Get("Stripe-Signature")
		event, err := webhook.ConstructEvent(body, sigHeader, hdl.cfg.StripeWebhookSecret)
		if err != nil {
			slog.ErrorContext(r.Context(), "Invalid signature", "error", err)
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
		// Use the verified event
		hdl.handleStripeEvent(r.Context(), &event)
	} else {
		// No webhook secret configured - parse directly (development only)
		var event stripe.Event
		if err := json.Unmarshal(body, &event); err != nil {
			slog.ErrorContext(r.Context(), "Failed to parse event", "error", err)
			http.Error(w, "Invalid event payload", http.StatusBadRequest)
			return
		}
		hdl.handleStripeEvent(r.Context(), &event)
	}

	// Always respond with 200 OK to acknowledge receipt
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// handleStripeEvent processes a Stripe webhook event
func (hdl *Handlers) handleStripeEvent(ctx context.Context, event *stripe.Event) {
	slog.InfoContext(ctx, "Received event", "type", event.Type, "id", event.ID)

	subscriptionStore := models.NewSubscriptionStore(hdl.db.Pool)
	entitySubscriptionStore := models.NewEntitySubscriptionStore(hdl.db.Pool)

	switch event.Type {
	case "checkout.session.completed":
		hdl.handleCheckoutSessionCompleted(ctx, event, subscriptionStore, entitySubscriptionStore)

	case "customer.subscription.updated":
		hdl.handleSubscriptionUpdated(ctx, event, subscriptionStore)

	case "customer.subscription.deleted":
		hdl.handleSubscriptionDeleted(ctx, event, subscriptionStore)

	case "invoice.payment_succeeded":
		hdl.handleInvoicePaymentSucceeded(ctx, event, subscriptionStore)

	case "invoice.payment_failed":
		hdl.handleInvoicePaymentFailed(ctx, event, subscriptionStore)

	case "customer.deleted":
		hdl.handleCustomerDeleted(ctx, event, subscriptionStore)

	default:
		slog.DebugContext(ctx, "Unhandled event type", "type", event.Type)
	}
}

// handleCheckoutSessionCompleted handles checkout.session.completed event
func (hdl *Handlers) handleCheckoutSessionCompleted(ctx context.Context, event *stripe.Event, subscriptionStore *models.SubscriptionStore, entitySubscriptionStore *models.EntitySubscriptionStore) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		slog.ErrorContext(ctx, "Failed to parse checkout.session.completed", "error", err)
		return
	}

	// Get subscription from session
	if session.Subscription == nil {
		slog.WarnContext(ctx, "Checkout session has no subscription")
		return
	}

	// Get user ID from metadata
	userIDStr, ok := session.Metadata["user_id"]
	if !ok {
		slog.WarnContext(ctx, "No user_id in checkout session metadata")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Invalid user_id in metadata", "error", err)
		return
	}

	// Get plan type from metadata
	planTypeStr, ok := session.Metadata["plan_type"]
	if !ok {
		slog.WarnContext(ctx, "No plan_type in metadata")
		return
	}
	planType := models.PlanType(planTypeStr)

	// Get entity IDs from metadata
	entityIDsStr := session.Metadata["entity_ids"]
	var entityIDs []uuid.UUID
	if entityIDsStr != "" {
		for _, eidStr := range strings.Split(entityIDsStr, ",") {
			if eid, err := uuid.Parse(strings.TrimSpace(eidStr)); err == nil {
				entityIDs = append(entityIDs, eid)
			}
		}
	}

	// Fetch subscription from Stripe
	stripeClient := billing.NewStripeClient(hdl.cfg)
	stripeSub, err := stripeClient.GetSubscription(ctx, session.Subscription.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get subscription from Stripe", "error", err)
		return
	}

	// Create or update subscription in database
	sub := &models.Subscription{
		UserID:               userID,
		StripeSubscriptionID: &stripeSub.ID,
		StripeCustomerID:     &stripeSub.Customer.ID,
		Status:               billing.ParseSubscriptionStatus(string(stripeSub.Status)),
		PlanType:             planType,
		CancelAtPeriodEnd:    stripeSub.CancelAtPeriodEnd,
	}

	if stripeSub.CurrentPeriodStart > 0 {
		periodStart := time.Unix(stripeSub.CurrentPeriodStart, 0)
		sub.CurrentPeriodStart = &periodStart
	}
	if stripeSub.CurrentPeriodEnd > 0 {
		periodEnd := time.Unix(stripeSub.CurrentPeriodEnd, 0)
		sub.CurrentPeriodEnd = &periodEnd
	}

	// Check if subscription already exists
	existing, err := subscriptionStore.GetByStripeSubscriptionID(ctx, stripeSub.ID)
	if err == nil && existing != nil {
		// Update existing
		sub.ID = existing.ID
		if err := subscriptionStore.Update(ctx, sub); err != nil {
			slog.ErrorContext(ctx, "Failed to update subscription", "error", err)
			return
		}
	} else {
		// Create new
		if err := subscriptionStore.Create(ctx, sub); err != nil {
			slog.ErrorContext(ctx, "Failed to create subscription", "error", err)
			return
		}
	}

	// Link entities to subscription
	for _, entityID := range entityIDs {
		es := &models.EntitySubscription{
			SubscriptionID: sub.ID,
			EntityID:       entityID,
		}
		if err := entitySubscriptionStore.Create(ctx, es); err != nil {
			slog.ErrorContext(ctx, "Failed to link entity to subscription", "entity_id", entityID, "error", err)
		}
	}

	slog.InfoContext(ctx, "Processed checkout.session.completed", "subscription_id", stripeSub.ID)
}

// handleSubscriptionUpdated handles customer.subscription.updated event
func (hdl *Handlers) handleSubscriptionUpdated(ctx context.Context, event *stripe.Event, subscriptionStore *models.SubscriptionStore) {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		slog.ErrorContext(ctx, "Failed to parse subscription.updated", "error", err)
		return
	}

	slog.InfoContext(ctx, "Processing subscription.updated",
		"id", stripeSub.ID, "status", stripeSub.Status, "cancel_at_period_end", stripeSub.CancelAtPeriodEnd)

	// Get existing subscription
	sub, err := subscriptionStore.GetByStripeSubscriptionID(ctx, stripeSub.ID)
	if err != nil {
		slog.WarnContext(ctx, "Subscription not found", "subscription_id", stripeSub.ID)
		return
	}

	// Update status and period
	newStatus := billing.ParseSubscriptionStatus(string(stripeSub.Status))
	slog.InfoContext(ctx, "Updating subscription",
		"subscription_id", stripeSub.ID, "old_status", sub.Status, "new_status", newStatus)

	sub.Status = newStatus
	sub.CancelAtPeriodEnd = stripeSub.CancelAtPeriodEnd

	if stripeSub.CurrentPeriodStart > 0 {
		periodStart := time.Unix(stripeSub.CurrentPeriodStart, 0)
		sub.CurrentPeriodStart = &periodStart
	}
	if stripeSub.CurrentPeriodEnd > 0 {
		periodEnd := time.Unix(stripeSub.CurrentPeriodEnd, 0)
		sub.CurrentPeriodEnd = &periodEnd
	}

	if err := subscriptionStore.Update(ctx, sub); err != nil {
		slog.ErrorContext(ctx, "Failed to update subscription", "error", err)
		return
	}

	slog.InfoContext(ctx, "Successfully updated subscription", "subscription_id", stripeSub.ID, "status", sub.Status)
}

// handleSubscriptionDeleted handles customer.subscription.deleted event
func (hdl *Handlers) handleSubscriptionDeleted(ctx context.Context, event *stripe.Event, subscriptionStore *models.SubscriptionStore) {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		slog.ErrorContext(ctx, "Failed to parse subscription.deleted", "error", err)
		return
	}

	// Get existing subscription
	sub, err := subscriptionStore.GetByStripeSubscriptionID(ctx, stripeSub.ID)
	if err != nil {
		slog.WarnContext(ctx, "Subscription not found", "subscription_id", stripeSub.ID)
		return
	}

	// Update status to canceled
	sub.Status = models.SubscriptionStatusCanceled
	if err := subscriptionStore.Update(ctx, sub); err != nil {
		slog.ErrorContext(ctx, "Failed to cancel subscription", "error", err)
		return
	}

	slog.InfoContext(ctx, "Canceled subscription", "subscription_id", stripeSub.ID)
}

// handleInvoicePaymentSucceeded handles invoice.payment_succeeded event
func (hdl *Handlers) handleInvoicePaymentSucceeded(ctx context.Context, event *stripe.Event, subscriptionStore *models.SubscriptionStore) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		slog.ErrorContext(ctx, "Failed to parse invoice.payment_succeeded", "error", err)
		return
	}

	if invoice.Subscription == nil {
		return
	}

	// Get subscription and update period
	sub, err := subscriptionStore.GetByStripeSubscriptionID(ctx, invoice.Subscription.ID)
	if err != nil {
		slog.WarnContext(ctx, "Subscription not found for invoice", "subscription_id", invoice.Subscription.ID)
		return
	}

	// Fetch latest subscription data from Stripe
	stripeClient := billing.NewStripeClient(hdl.cfg)
	stripeSub, err := stripeClient.GetSubscription(ctx, invoice.Subscription.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get subscription from Stripe", "error", err)
		return
	}

	// Update period dates
	if stripeSub.CurrentPeriodStart > 0 {
		periodStart := time.Unix(stripeSub.CurrentPeriodStart, 0)
		sub.CurrentPeriodStart = &periodStart
	}
	if stripeSub.CurrentPeriodEnd > 0 {
		periodEnd := time.Unix(stripeSub.CurrentPeriodEnd, 0)
		sub.CurrentPeriodEnd = &periodEnd
	}

	if err := subscriptionStore.Update(ctx, sub); err != nil {
		slog.ErrorContext(ctx, "Failed to update subscription period", "error", err)
		return
	}

	slog.InfoContext(ctx, "Updated subscription period", "subscription_id", invoice.Subscription.ID)
}

// handleInvoicePaymentFailed handles invoice.payment_failed event
func (hdl *Handlers) handleInvoicePaymentFailed(ctx context.Context, event *stripe.Event, subscriptionStore *models.SubscriptionStore) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		slog.ErrorContext(ctx, "Failed to parse invoice.payment_failed", "error", err)
		return
	}

	if invoice.Subscription == nil {
		return
	}

	// Get subscription and update status
	sub, err := subscriptionStore.GetByStripeSubscriptionID(ctx, invoice.Subscription.ID)
	if err != nil {
		slog.WarnContext(ctx, "Subscription not found for invoice", "subscription_id", invoice.Subscription.ID)
		return
	}

	// Update to past_due if not already canceled
	if sub.Status != models.SubscriptionStatusCanceled {
		sub.Status = models.SubscriptionStatusPastDue
		if err := subscriptionStore.Update(ctx, sub); err != nil {
			slog.ErrorContext(ctx, "Failed to update subscription status", "error", err)
			return
		}
	}

	slog.WarnContext(ctx, "Marked subscription as past_due", "subscription_id", invoice.Subscription.ID)
}

// handleCustomerDeleted handles customer.deleted event
// When a customer is deleted in Stripe, all their subscriptions are also deleted
// We need to mark all subscriptions for this customer as canceled
func (hdl *Handlers) handleCustomerDeleted(ctx context.Context, event *stripe.Event, subscriptionStore *models.SubscriptionStore) {
	var stripeCustomer stripe.Customer
	if err := json.Unmarshal(event.Data.Raw, &stripeCustomer); err != nil {
		slog.ErrorContext(ctx, "Failed to parse customer.deleted", "error", err)
		return
	}

	slog.InfoContext(ctx, "Processing customer.deleted", "customer_id", stripeCustomer.ID)

	// Find all subscriptions for this customer
	subscriptions, err := subscriptionStore.GetByStripeCustomerID(ctx, stripeCustomer.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to find subscriptions for customer", "customer_id", stripeCustomer.ID, "error", err)
		return
	}

	// Mark all subscriptions as canceled
	for _, sub := range subscriptions {
		if sub.Status != models.SubscriptionStatusCanceled {
			sub.Status = models.SubscriptionStatusCanceled
			if err := subscriptionStore.Update(ctx, sub); err != nil {
				slog.ErrorContext(ctx, "Failed to cancel subscription", "subscription_id", sub.ID, "error", err)
				continue
			}
			slog.DebugContext(ctx, "Canceled subscription due to customer deletion", "subscription_id", sub.ID)
		}
	}

	slog.InfoContext(ctx, "Processed customer.deleted", "subscriptions_canceled", len(subscriptions))
}
