package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EntityType represents the type of entity
type EntityType string

const (
	EntityTypePerson      EntityType = "person"
	EntityTypeBusiness    EntityType = "business"
	EntityTypeTrust       EntityType = "trust"
	EntityTypePartnership EntityType = "partnership"
	EntityTypeGovernment  EntityType = "government"
)

// Business subtypes
const (
	BusinessSubtypeFinancialInstitution = "financial_institution"
	BusinessSubtypeRetailer             = "retailer"
	BusinessSubtypeRestaurant           = "restaurant"
	BusinessSubtypeCafe                 = "cafe"
	BusinessSubtypeSupermarket          = "supermarket"
	BusinessSubtypeGrocery              = "grocery"
	BusinessSubtypeFoodAndBeverage      = "food_and_beverage"
	BusinessSubtypeService              = "service"
	BusinessSubtypeSoftware             = "software"
	BusinessSubtypeUtility              = "utility"
	BusinessSubtypeEntertainment        = "entertainment"
	BusinessSubtypeTransportation       = "transportation"
	BusinessSubtypeHealthcare           = "healthcare"
	BusinessSubtypeEducation            = "education"
	BusinessSubtypeFitness              = "fitness"
	BusinessSubtypeTravel               = "travel"
	BusinessSubtypeGovernment           = "government_service"
)

// Slugify converts a display name to a URL-friendly slug.
func Slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsOneTimeBusinessType returns true if the business subtype is typically not recurring
// (e.g., cafes, restaurants, supermarkets are usually one-time purchases)
func IsOneTimeBusinessType(subtype string) bool {
	switch subtype {
	case BusinessSubtypeCafe, BusinessSubtypeRestaurant, BusinessSubtypeFoodAndBeverage,
		BusinessSubtypeSupermarket, BusinessSubtypeGrocery, BusinessSubtypeRetailer:
		return true
	}
	return false
}

// TellerCategoryToSubtype maps Teller's transaction categories to business subtypes
// This uses the bank's categorization as a strong signal for entity type
func TellerCategoryToSubtype(tellerCategory string) string {
	switch strings.ToLower(tellerCategory) {
	// Food and drink
	case "dining", "restaurants", "fast_food", "coffee_shops", "bars":
		return BusinessSubtypeRestaurant
	case "cafe", "coffee":
		return BusinessSubtypeCafe
	case "groceries", "supermarkets", "grocery":
		return BusinessSubtypeSupermarket
	case "food_and_drink", "food":
		return BusinessSubtypeFoodAndBeverage

	// Services and subscriptions
	case "software", "saas", "digital_purchase", "digital_subscription":
		return BusinessSubtypeSoftware
	case "utilities", "electric", "gas", "water", "internet", "phone", "cable":
		return BusinessSubtypeUtility
	case "subscription", "streaming", "entertainment":
		return BusinessSubtypeEntertainment
	case "gym", "fitness", "sports":
		return BusinessSubtypeFitness

	// Other
	case "transportation", "taxi", "ride_share", "uber", "lyft", "parking", "fuel", "gas_stations":
		return BusinessSubtypeTransportation
	case "healthcare", "medical", "pharmacy", "health":
		return BusinessSubtypeHealthcare
	case "education", "school", "tuition", "books":
		return BusinessSubtypeEducation
	case "travel", "airlines", "hotels", "lodging":
		return BusinessSubtypeTravel
	case "bank", "financial", "atm", "bank_fees":
		return BusinessSubtypeFinancialInstitution
	case "government", "taxes", "government_services":
		return BusinessSubtypeGovernment

	// Retail
	case "shops", "shopping", "retail", "merchandise", "general_merchandise", "clothing",
		"electronics", "home", "home_improvement":
		return BusinessSubtypeRetailer

	default:
		return "" // Unknown - will need other detection methods
	}
}

// Person subtypes
const (
	PersonSubtypeIndividual     = "individual"
	PersonSubtypeSoleProprietor = "sole_proprietor"
)

// Trust subtypes
const (
	TrustSubtypeRevocable   = "revocable"
	TrustSubtypeIrrevocable = "irrevocable"
	TrustSubtypeCharitable  = "charitable"
)

// Partnership subtypes
const (
	PartnershipSubtypeGeneral       = "general"
	PartnershipSubtypeLimited       = "limited"
	PartnershipSubtypeLLC           = "llc"
	PartnershipSubtypeMarriedCouple = "married_couple"
)

// Government subtypes
const (
	GovernmentSubtypeFederal = "federal"
	GovernmentSubtypeState   = "state"
	GovernmentSubtypeLocal   = "local"
)

// Enrichment status constants
const (
	EnrichmentStatusPending      = "pending"
	EnrichmentStatusSearching    = "searching"     // Searching web for merchant
	EnrichmentStatusExtracting   = "extracting"    // Extracting company info
	EnrichmentStatusFetchingLogo = "fetching_logo" // Downloading logo
	EnrichmentStatusDone         = "done"
	EnrichmentStatusFailed       = "failed"
)

// Entity represents any entity that can be involved in financial transactions
// This includes persons, businesses, trusts, partnerships, and government entities
type Entity struct {
	ID             uuid.UUID       `json:"id"`
	Type           EntityType      `json:"type"`
	Subtype        string          `json:"subtype,omitempty"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug,omitempty"`
	LogoURL        string          `json:"logo_url,omitempty"`
	Website        string          `json:"website,omitempty"`
	Description    string          `json:"description,omitempty"`
	ExternalID     string          `json:"external_id,omitempty"`
	ExternalSource string          `json:"external_source,omitempty"` // teller, manual
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	UserVerified   bool            `json:"user_verified"`

	// Enrichment workflow state tracking
	EnrichmentStatus      string           `json:"enrichment_status,omitempty"`       // pending, searching, extracting, fetching_logo, done, failed
	EnrichmentSteps       []EnrichmentStep `json:"enrichment_steps,omitempty"`        // Array of enrichment steps with timestamps
	EnrichmentError       string           `json:"enrichment_error,omitempty"`        // Error message if enrichment failed
	EnrichmentStartedAt   *time.Time       `json:"enrichment_started_at,omitempty"`   // When enrichment started
	EnrichmentCompletedAt *time.Time       `json:"enrichment_completed_at,omitempty"` // When enrichment completed

	// Pattern detection hints (learned from transaction patterns)
	// Entities can exhibit multiple patterns (e.g., Amazon: subscriptions, salary, purchases)
	PatternHints []EntityPatternHint `json:"pattern_hints,omitempty"`

	// Vector embedding for semantic similarity search
	Embedding          []float32  `json:"embedding,omitempty"`            // Vector embedding (typically 768 dimensions)
	EmbeddingModel     string     `json:"embedding_model,omitempty"`      // Model used to generate embedding
	EmbeddingUpdatedAt *time.Time `json:"embedding_updated_at,omitempty"` // When embedding was last updated

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EntityPatternHint represents a learned pattern for an entity
type EntityPatternHint struct {
	PatternType     string    `json:"pattern_type"`     // salary, recurring_bill, etc.
	Frequency       string    `json:"frequency"`        // weekly, monthly, annual
	Confidence      int       `json:"confidence"`       // 0-100 based on occurrences
	OccurrenceCount int       `json:"occurrence_count"` // How many transactions confirmed this
	LastUpdatedAt   time.Time `json:"last_updated_at"`  // When this hint was last updated
}

// EnrichmentStep represents a single step in the enrichment workflow
type EnrichmentStep struct {
	Step      string    `json:"step"`              // searching, extracting, fetching_logo, etc.
	Timestamp time.Time `json:"timestamp"`         // When this step occurred
	Details   string    `json:"details,omitempty"` // Optional details about the step
}

// IsPerson returns true if the entity is a person
func (e *Entity) IsPerson() bool {
	return e.Type == EntityTypePerson
}

// EntityStore handles database operations for entities
type EntityStore struct {
	pool *pgxpool.Pool
}

// NewEntityStore creates a new EntityStore
func NewEntityStore(pool *pgxpool.Pool) *EntityStore {
	return &EntityStore{pool: pool}
}

// Create creates a new entity
func (s *EntityStore) Create(ctx context.Context, e *Entity) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now

	// Default enrichment status
	if e.EnrichmentStatus == "" {
		e.EnrichmentStatus = EnrichmentStatusPending
	}

	// Marshal enrichment steps to JSON
	var enrichmentStepsJSON []byte
	if len(e.EnrichmentSteps) > 0 {
		var err error
		enrichmentStepsJSON, err = json.Marshal(e.EnrichmentSteps)
		if err != nil {
			return fmt.Errorf("failed to marshal enrichment steps: %w", err)
		}
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO entities (id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified,
			enrichment_status, enrichment_steps, enrichment_error,
			enrichment_started_at, enrichment_completed_at,
			created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`, e.ID, e.Type, NullString(e.Subtype), e.Name, NullString(e.Slug),
		NullString(e.LogoURL), NullString(e.Website), NullString(e.Description),
		NullString(e.ExternalID), NullString(e.ExternalSource), e.Metadata,
		e.UserVerified,
		NullString(e.EnrichmentStatus), enrichmentStepsJSON, NullString(e.EnrichmentError),
		e.EnrichmentStartedAt, e.EnrichmentCompletedAt,
		e.CreatedAt, e.UpdatedAt)

	return err
}

// Update updates an existing entity
func (s *EntityStore) Update(ctx context.Context, e *Entity) error {
	e.UpdatedAt = time.Now()

	// Marshal enrichment steps to JSON
	var enrichmentStepsJSON []byte
	if len(e.EnrichmentSteps) > 0 {
		var err error
		enrichmentStepsJSON, err = json.Marshal(e.EnrichmentSteps)
		if err != nil {
			return fmt.Errorf("failed to marshal enrichment steps: %w", err)
		}
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE entities SET
			type = $2, subtype = $3, name = $4, slug = $5, logo_url = $6, website = $7,
			description = $8, external_id = $9, external_source = $10, metadata = $11,
			user_verified = $12,
			enrichment_status = $13, enrichment_steps = $14, enrichment_error = $15,
			enrichment_started_at = $16, enrichment_completed_at = $17,
			updated_at = $18
		WHERE id = $1
	`, e.ID, e.Type, NullString(e.Subtype), e.Name, NullString(e.Slug),
		NullString(e.LogoURL), NullString(e.Website), NullString(e.Description),
		NullString(e.ExternalID), NullString(e.ExternalSource), e.Metadata,
		e.UserVerified,
		NullString(e.EnrichmentStatus), enrichmentStepsJSON, NullString(e.EnrichmentError),
		e.EnrichmentStartedAt, e.EnrichmentCompletedAt,
		e.UpdatedAt)

	return err
}

// UpdateEnrichmentStatus updates the enrichment status of an entity
func (s *EntityStore) UpdateEnrichmentStatus(ctx context.Context, entityID uuid.UUID, status string, errorMsg string) error {
	now := time.Now()
	var startedAt, completedAt *time.Time

	if status == EnrichmentStatusSearching || status == EnrichmentStatusExtracting || status == EnrichmentStatusFetchingLogo {
		// Check if we're starting enrichment
		var currentStatus string
		err := s.pool.QueryRow(ctx, `SELECT enrichment_status FROM entities WHERE id = $1`, entityID).Scan(&currentStatus)
		if err == nil && currentStatus == EnrichmentStatusPending {
			startedAt = &now
		}
	}

	if status == EnrichmentStatusDone || status == EnrichmentStatusFailed {
		completedAt = &now
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE entities SET
			enrichment_status = $2,
			enrichment_error = $3,
			enrichment_started_at = COALESCE($4, enrichment_started_at),
			enrichment_completed_at = $5,
			updated_at = $6
		WHERE id = $1
	`, entityID, status, NullString(errorMsg), startedAt, completedAt, now)

	return err
}

// AddEnrichmentStep adds a step to the enrichment workflow
func (s *EntityStore) AddEnrichmentStep(ctx context.Context, entityID uuid.UUID, step string, details string) error {
	now := time.Now()
	enrichmentStep := EnrichmentStep{
		Step:      step,
		Timestamp: now,
		Details:   details,
	}

	// Get current steps
	var currentStepsJSON []byte
	err := s.pool.QueryRow(ctx, `SELECT enrichment_steps FROM entities WHERE id = $1`, entityID).Scan(&currentStepsJSON)
	if err != nil {
		return err
	}

	var steps []EnrichmentStep
	if len(currentStepsJSON) > 0 {
		if err := json.Unmarshal(currentStepsJSON, &steps); err != nil {
			// If unmarshal fails, start fresh
			steps = []EnrichmentStep{}
		}
	}

	// Add new step
	steps = append(steps, enrichmentStep)

	// Marshal back to JSON
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return fmt.Errorf("failed to marshal enrichment steps: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE entities SET
			enrichment_steps = $2,
			updated_at = $3
		WHERE id = $1
	`, entityID, stepsJSON, now)

	return err
}

// scanBasicEntity populates an Entity from a Rows scan over the basic column set:
// id, type, subtype, name, slug, logo_url, website, description,
// external_id, external_source, metadata, user_verified, created_at, updated_at.
func scanBasicEntity(e *Entity, rows pgx.Rows) error {
	var subtype, slug, logoURL, website, description, externalID, externalSource sql.NullString
	if err := rows.Scan(&e.ID, &e.Type, &subtype, &e.Name, &slug, &logoURL, &website, &description,
		&externalID, &externalSource, &e.Metadata, &e.UserVerified, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return err
	}
	e.Subtype = subtype.String
	e.Slug = slug.String
	e.LogoURL = logoURL.String
	e.Website = website.String
	e.Description = description.String
	e.ExternalID = externalID.String
	e.ExternalSource = externalSource.String
	return nil
}

// scanFullEntity populates an Entity from a Row scan over the full column set including enrichment fields.
func scanFullEntity(ctx context.Context, row pgx.Row) (*Entity, error) {
	var e Entity
	var subtype, slug, logoURL, website, description, externalID, externalSource sql.NullString
	var enrichmentStatus, enrichmentError sql.NullString
	var enrichmentStepsJSON []byte
	var enrichmentStartedAt, enrichmentCompletedAt sql.NullTime

	if err := row.Scan(&e.ID, &e.Type, &subtype, &e.Name, &slug, &logoURL, &website, &description,
		&externalID, &externalSource, &e.Metadata, &e.UserVerified,
		&enrichmentStatus, &enrichmentStepsJSON, &enrichmentError,
		&enrichmentStartedAt, &enrichmentCompletedAt,
		&e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}

	e.Subtype = subtype.String
	e.Slug = slug.String
	e.LogoURL = logoURL.String
	e.Website = website.String
	e.Description = description.String
	e.ExternalID = externalID.String
	e.ExternalSource = externalSource.String
	e.EnrichmentStatus = enrichmentStatus.String
	e.EnrichmentError = enrichmentError.String
	if enrichmentStartedAt.Valid {
		e.EnrichmentStartedAt = &enrichmentStartedAt.Time
	}
	if enrichmentCompletedAt.Valid {
		e.EnrichmentCompletedAt = &enrichmentCompletedAt.Time
	}
	if len(enrichmentStepsJSON) > 0 {
		if err := json.Unmarshal(enrichmentStepsJSON, &e.EnrichmentSteps); err != nil {
			slog.WarnContext(ctx, "failed to unmarshal enrichment steps", "entity_id", e.ID, "err", err)
		}
	}

	return &e, nil
}

// GetByID returns an entity by ID
func (s *EntityStore) GetByID(ctx context.Context, id uuid.UUID) (*Entity, error) {
	return scanFullEntity(ctx, s.pool.QueryRow(ctx, `
		SELECT id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified,
			enrichment_status, enrichment_steps, enrichment_error,
			enrichment_started_at, enrichment_completed_at,
			created_at, updated_at
		FROM entities WHERE id = $1
	`, id))
}

// GetByExternalID returns an entity by external ID and source
func (s *EntityStore) GetByExternalID(ctx context.Context, externalSource, externalID string) (*Entity, error) {
	return scanFullEntity(ctx, s.pool.QueryRow(ctx, `
		SELECT id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified,
			enrichment_status, enrichment_steps, enrichment_error,
			enrichment_started_at, enrichment_completed_at,
			created_at, updated_at
		FROM entities WHERE external_source = $1 AND external_id = $2
	`, externalSource, externalID))
}

// GetByName returns an entity by name (case-insensitive)
func (s *EntityStore) GetByName(ctx context.Context, name string) (*Entity, error) {
	return scanFullEntity(ctx, s.pool.QueryRow(ctx, `
		SELECT id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified,
			enrichment_status, enrichment_steps, enrichment_error,
			enrichment_started_at, enrichment_completed_at,
			created_at, updated_at
		FROM entities WHERE LOWER(name) = LOWER($1)
	`, name))
}

// GetByNameAndType returns an entity by name and type
func (s *EntityStore) GetByNameAndType(ctx context.Context, name string, entityType EntityType) (*Entity, error) {
	return scanFullEntity(ctx, s.pool.QueryRow(ctx, `
		SELECT id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified,
			enrichment_status, enrichment_steps, enrichment_error,
			enrichment_started_at, enrichment_completed_at,
			created_at, updated_at
		FROM entities WHERE LOWER(name) = LOWER($1) AND type = $2
	`, name, entityType))
}

// List returns all entities with optional filtering
func (s *EntityStore) List(ctx context.Context, entityType *EntityType, subtype *string, search string, limit, offset int) ([]*Entity, int, error) {
	baseQuery := `FROM entities WHERE 1=1`
	args := []any{}
	argIdx := 1

	if entityType != nil {
		baseQuery += ` AND type = $` + strconv.Itoa(argIdx)
		args = append(args, *entityType)
		argIdx++
	}

	if subtype != nil && *subtype != "" {
		baseQuery += ` AND subtype = $` + strconv.Itoa(argIdx)
		args = append(args, *subtype)
		argIdx++
	}

	if search != "" {
		baseQuery += ` AND name ILIKE $` + strconv.Itoa(argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	// Get total count
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) `+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get entities
	query := `SELECT id, type, subtype, name, slug, logo_url, website, description,
		external_id, external_source, metadata, user_verified, created_at, updated_at ` +
		baseQuery + ` ORDER BY name`

	if limit > 0 {
		query += ` LIMIT $` + strconv.Itoa(argIdx)
		args = append(args, limit)
		argIdx++
	}

	if offset > 0 {
		query += ` OFFSET $` + strconv.Itoa(argIdx)
		args = append(args, offset)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entities []*Entity
	for rows.Next() {
		var e Entity
		if err := scanBasicEntity(&e, rows); err != nil {
			return nil, 0, err
		}
		entities = append(entities, &e)
	}

	return entities, total, rows.Err()
}

// Delete removes an entity by ID
func (s *EntityStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM entities WHERE id = $1`, id)
	return err
}

// Merge merges one entity into another, updating all references
// All transactions, relationships, and ownership records referencing fromID will be updated to reference toID
// After merging, the fromID entity is deleted
func (s *EntityStore) Merge(ctx context.Context, fromID, toID uuid.UUID) error {
	if fromID == toID {
		return fmt.Errorf("cannot merge entity into itself")
	}

	// Verify both entities exist
	fromEntity, err := s.GetByID(ctx, fromID)
	if err != nil {
		return fmt.Errorf("source entity not found: %w", err)
	}
	_, err = s.GetByID(ctx, toID)
	if err != nil {
		return fmt.Errorf("target entity not found: %w", err)
	}

	// Use a transaction to ensure atomicity
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Update transactions that reference fromID as entity_id
		_, err := tx.Exec(ctx, `
			UPDATE transactions SET entity_id = $1, updated_at = NOW()
			WHERE entity_id = $2
		`, toID, fromID)
		if err != nil {
			return fmt.Errorf("failed to update entity_id in transactions: %w", err)
		}

		// Update transactions that reference fromID as counterparty_entity_id
		_, err = tx.Exec(ctx, `
			UPDATE transactions SET counterparty_entity_id = $1, updated_at = NOW()
			WHERE counterparty_entity_id = $2
		`, toID, fromID)
		if err != nil {
			return fmt.Errorf("failed to update counterparty_entity_id in transactions: %w", err)
		}

		// Update transactions that reference fromID as intermediary_entity_id
		_, err = tx.Exec(ctx, `
			UPDATE transactions SET intermediary_entity_id = $1, updated_at = NOW()
			WHERE intermediary_entity_id = $2
		`, toID, fromID)
		if err != nil {
			return fmt.Errorf("failed to update intermediary_entity_id in transactions: %w", err)
		}

		// Update recurring_patterns that reference fromID
		_, err = tx.Exec(ctx, `
			UPDATE recurring_patterns SET entity_id = $1
			WHERE entity_id = $2
		`, toID, fromID)
		if err != nil {
			return fmt.Errorf("failed to update recurring_patterns: %w", err)
		}

		// Update entity_relationships
		// Update relationships where fromID is entity_a
		_, err = tx.Exec(ctx, `
			UPDATE entity_relationships SET entity_a_id = $1
			WHERE entity_a_id = $2 AND entity_b_id != $1
		`, toID, fromID)
		if err != nil {
			return fmt.Errorf("failed to update entity_relationships (entity_a): %w", err)
		}

		// Update relationships where fromID is entity_b
		_, err = tx.Exec(ctx, `
			UPDATE entity_relationships SET entity_b_id = $1
			WHERE entity_b_id = $2 AND entity_a_id != $1
		`, toID, fromID)
		if err != nil {
			return fmt.Errorf("failed to update entity_relationships (entity_b): %w", err)
		}

		// Delete relationships where both entities are the same (self-referential)
		_, err = tx.Exec(ctx, `
			DELETE FROM entity_relationships
			WHERE (entity_a_id = $1 AND entity_b_id = $2) OR (entity_a_id = $2 AND entity_b_id = $1)
		`, toID, fromID)
		if err != nil {
			return fmt.Errorf("failed to delete duplicate relationships: %w", err)
		}

		// Update account_entity_ownership
		_, err = tx.Exec(ctx, `
			UPDATE account_entity_ownership SET entity_id = $1
			WHERE entity_id = $2
		`, toID, fromID)
		if err != nil {
			return fmt.Errorf("failed to update account_entity_ownership: %w", err)
		}

		// If the target entity doesn't have certain fields but the source does, update them
		// (but only if target is not user-verified, to preserve user edits)
		_, err = tx.Exec(ctx, `
			UPDATE entities SET
				logo_url = CASE WHEN logo_url IS NULL OR logo_url = '' THEN $1 ELSE logo_url END,
				website = CASE WHEN website IS NULL OR website = '' THEN $2 ELSE website END,
				description = CASE WHEN description IS NULL OR description = '' THEN $3 ELSE description END,
				updated_at = NOW()
			WHERE id = $4 AND user_verified = false
		`, NullString(fromEntity.LogoURL), NullString(fromEntity.Website), NullString(fromEntity.Description), toID)
		if err != nil {
			return fmt.Errorf("failed to merge entity fields: %w", err)
		}

		// Finally, delete the source entity
		_, err = tx.Exec(ctx, `DELETE FROM entities WHERE id = $1`, fromID)
		if err != nil {
			return fmt.Errorf("failed to delete source entity: %w", err)
		}

		return nil
	})
}

// Upsert creates or updates an entity by external ID
func (s *EntityStore) Upsert(ctx context.Context, e *Entity) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now

	_, err := s.pool.Exec(ctx, `
		INSERT INTO entities (id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (external_source, external_id) WHERE external_source IS NOT NULL AND external_id IS NOT NULL
		DO UPDATE SET
			name = CASE WHEN entities.user_verified THEN entities.name ELSE EXCLUDED.name END,
			slug = CASE WHEN entities.user_verified THEN entities.slug ELSE EXCLUDED.slug END,
			logo_url = CASE WHEN entities.user_verified THEN entities.logo_url ELSE EXCLUDED.logo_url END,
			website = CASE WHEN entities.user_verified THEN entities.website ELSE EXCLUDED.website END,
			description = CASE WHEN entities.user_verified THEN entities.description ELSE EXCLUDED.description END,
			subtype = CASE WHEN entities.user_verified THEN entities.subtype ELSE EXCLUDED.subtype END,
			updated_at = NOW()
	`, e.ID, e.Type, NullString(e.Subtype), e.Name, NullString(e.Slug),
		NullString(e.LogoURL), NullString(e.Website), NullString(e.Description),
		NullString(e.ExternalID), NullString(e.ExternalSource), e.Metadata,
		e.UserVerified, e.CreatedAt, e.UpdatedAt)

	if err == nil && e.ExternalID != "" && e.ExternalSource != "" {
		if scanErr := s.pool.QueryRow(ctx, `
			SELECT id FROM entities WHERE external_source = $1 AND external_id = $2
		`, e.ExternalSource, e.ExternalID).Scan(&e.ID); scanErr != nil {
			slog.WarnContext(ctx, "failed to refetch entity id after upsert",
				"external_source", e.ExternalSource,
				"external_id", e.ExternalID,
				"err", scanErr)
		}
	}

	return err
}

// SearchByBM25 searches entities by name using BM25-style full-text search
func (s *EntityStore) SearchByBM25(ctx context.Context, query string, limit int) ([]*Entity, error) {
	// Simple implementation using ILIKE for now (PostgreSQL BM25 requires pg_trgm or full-text search setup)
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified, created_at, updated_at
		FROM entities 
		WHERE LOWER(name) LIKE '%' || LOWER($1) || '%'
		ORDER BY 
			CASE WHEN LOWER(name) = LOWER($1) THEN 0 ELSE 1 END,
			name
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*Entity
	for rows.Next() {
		var e Entity
		if err := scanBasicEntity(&e, rows); err != nil {
			return nil, err
		}
		entities = append(entities, &e)
	}

	return entities, rows.Err()
}

// CountOrphans returns the count of entities with no associated transactions
func (s *EntityStore) CountOrphans(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM entities
		WHERE NOT EXISTS (
			SELECT 1 FROM transactions t WHERE t.entity_id = entities.id
		)
		AND NOT EXISTS (
			SELECT 1 FROM transactions t WHERE t.counterparty_entity_id = entities.id
		)
		AND NOT EXISTS (
			SELECT 1 FROM transactions t WHERE t.intermediary_entity_id = entities.id
		)
	`).Scan(&count)
	return count, err
}

// PruneOrphans deletes entities with no associated transactions
func (s *EntityStore) PruneOrphans(ctx context.Context) (int, error) {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM entities
		WHERE NOT EXISTS (
			SELECT 1 FROM transactions t WHERE t.entity_id = entities.id
		)
		AND NOT EXISTS (
			SELECT 1 FROM transactions t WHERE t.counterparty_entity_id = entities.id
		)
		AND NOT EXISTS (
			SELECT 1 FROM transactions t WHERE t.intermediary_entity_id = entities.id
		)
	`)
	if err != nil {
		return 0, err
	}
	return int(result.RowsAffected()), nil
}

// UpdatePatternHint updates the pattern hints for an entity based on detected patterns
// Entities can have multiple patterns (e.g., Amazon: subscriptions AND salary)
// This upserts into the pattern_hints JSONB array
func (s *EntityStore) UpdatePatternHint(ctx context.Context, entityID uuid.UUID, patternType, frequency string, confidence int) error {
	now := time.Now()

	// Read current hints
	var hintsJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(pattern_hints, '[]'::jsonb) FROM entities WHERE id = $1
	`, entityID).Scan(&hintsJSON)
	if err != nil {
		return fmt.Errorf("failed to read pattern hints: %w", err)
	}

	var hints []EntityPatternHint
	if err := json.Unmarshal(hintsJSON, &hints); err != nil {
		hints = []EntityPatternHint{} // Start fresh if corrupted
	}

	// Default frequency if empty
	if frequency == "" {
		frequency = "monthly"
	}

	// Find existing hint for this pattern type
	found := false
	for i := range hints {
		if hints[i].PatternType == patternType {
			// Update existing - increment count, keep max confidence
			hints[i].OccurrenceCount++
			if confidence > hints[i].Confidence {
				hints[i].Confidence = confidence
			}
			hints[i].Frequency = frequency
			hints[i].LastUpdatedAt = now
			found = true
			break
		}
	}

	// Add new hint if not found
	if !found {
		hints = append(hints, EntityPatternHint{
			PatternType:     patternType,
			Frequency:       frequency,
			Confidence:      confidence,
			OccurrenceCount: 1,
			LastUpdatedAt:   now,
		})
	}

	// Write back
	updatedJSON, err := json.Marshal(hints)
	if err != nil {
		return fmt.Errorf("failed to marshal pattern hints: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE entities SET pattern_hints = $2, updated_at = $3 WHERE id = $1
	`, entityID, updatedJSON, now)
	return err
}

// UpdateEmbedding updates the embedding for an entity
func (s *EntityStore) UpdateEmbedding(ctx context.Context, entityID uuid.UUID, embedding []float32, model string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE entities SET
			embedding = $2,
			embedding_model = $3,
			embedding_updated_at = $4,
			updated_at = $5
		WHERE id = $1
	`, entityID, embedding, model, now, now)
	return err
}

func (s *EntityStore) getEmbedding(ctx context.Context, entityID uuid.UUID) ([]float32, string, error) {
	var embedding []float32
	var model sql.NullString
	err := s.pool.QueryRow(ctx, `
		SELECT embedding, embedding_model FROM entities WHERE id = $1
	`, entityID).Scan(&embedding, &model)
	if err != nil {
		return nil, "", err
	}
	return embedding, model.String, nil
}

// EntitySimilarityResult represents a similarity search result
type EntitySimilarityResult struct {
	Entity     *Entity
	Similarity float32
}

// FindSimilarEntities finds entities most similar to the given embedding using cosine similarity
// Returns entities sorted by similarity (highest first), limited to 'limit' results
// Only considers entities that have embeddings and similarity >= minSimilarity
func (s *EntityStore) FindSimilarEntities(ctx context.Context, embedding []float32, limit int, minSimilarity float32) ([]*EntitySimilarityResult, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding cannot be empty")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified,
			embedding, embedding_model, embedding_updated_at,
			created_at, updated_at,
			cosine_similarity(embedding, $1) as similarity
		FROM entities 
		WHERE embedding IS NOT NULL
			AND cosine_similarity(embedding, $1) >= $2
		ORDER BY similarity DESC
		LIMIT $3
	`, embedding, minSimilarity, limit)
	if err != nil {
		return nil, fmt.Errorf("similarity search failed: %w", err)
	}
	defer rows.Close()

	var results []*EntitySimilarityResult
	for rows.Next() {
		r, err := scanEntitySimilarityRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// FindSimilarToEntity finds entities similar to a given entity
// Excludes the entity itself from results
func (s *EntityStore) FindSimilarToEntity(ctx context.Context, entityID uuid.UUID, limit int, minSimilarity float32) ([]*EntitySimilarityResult, error) {
	// First get the entity's embedding
	embedding, _, err := s.getEmbedding(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity embedding: %w", err)
	}
	if len(embedding) == 0 {
		return nil, fmt.Errorf("entity has no embedding")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified,
			embedding, embedding_model, embedding_updated_at,
			created_at, updated_at,
			cosine_similarity(embedding, $1) as similarity
		FROM entities 
		WHERE embedding IS NOT NULL
			AND id != $4
			AND cosine_similarity(embedding, $1) >= $2
		ORDER BY similarity DESC
		LIMIT $3
	`, embedding, minSimilarity, limit, entityID)
	if err != nil {
		return nil, fmt.Errorf("similarity search failed: %w", err)
	}
	defer rows.Close()

	var results []*EntitySimilarityResult
	for rows.Next() {
		r, err := scanEntitySimilarityRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// GetEntitiesWithoutEmbedding returns entities that don't have an embedding yet
// Useful for batch embedding generation
func (s *EntityStore) GetEntitiesWithoutEmbedding(ctx context.Context, limit int) ([]*Entity, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, subtype, name, slug, logo_url, website, description,
			external_id, external_source, metadata, user_verified,
			created_at, updated_at
		FROM entities 
		WHERE embedding IS NULL
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*Entity
	for rows.Next() {
		var e Entity
		if err := scanBasicEntity(&e, rows); err != nil {
			return nil, err
		}
		entities = append(entities, &e)
	}

	return entities, rows.Err()
}

// CountEntitiesWithEmbedding returns the count of entities with/without embeddings
func (s *EntityStore) CountEntitiesWithEmbedding(ctx context.Context) (withEmbedding, withoutEmbedding int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE embedding IS NOT NULL) as with_embedding,
			COUNT(*) FILTER (WHERE embedding IS NULL) as without_embedding
		FROM entities
	`).Scan(&withEmbedding, &withoutEmbedding)
	return
}

// EntityRelationship represents a relationship between two entities
type EntityRelationship struct {
	ID               uuid.UUID  `json:"id"`
	LedgerID         uuid.UUID  `json:"ledger_id"`
	EntityAID        uuid.UUID  `json:"entity_a_id"`
	EntityBID        uuid.UUID  `json:"entity_b_id"`
	RelationshipType string     `json:"relationship_type"` // spouse, partner, family, trustee, beneficiary, employer, self
	ValidFrom        *time.Time `json:"valid_from,omitempty"`
	ValidTo          *time.Time `json:"valid_to,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`

	// Loaded separately
	EntityA *Entity `json:"entity_a,omitempty"`
	EntityB *Entity `json:"entity_b,omitempty"`
}

// Relationship type constants
const (
	RelationshipTypeSpouse      = "spouse"
	RelationshipTypePartner     = "partner"
	RelationshipTypeFamily      = "family"
	RelationshipTypeTrustee     = "trustee"
	RelationshipTypeBeneficiary = "beneficiary"
	RelationshipTypeEmployer    = "employer"
	RelationshipTypeSelf        = "self"
)

// EntityRelationshipStore handles database operations for entity relationships
type EntityRelationshipStore struct {
	pool *pgxpool.Pool
}

// NewEntityRelationshipStore creates a new EntityRelationshipStore
func NewEntityRelationshipStore(pool *pgxpool.Pool) *EntityRelationshipStore {
	return &EntityRelationshipStore{pool: pool}
}

// Create creates a new entity relationship
func (s *EntityRelationshipStore) Create(ctx context.Context, r *EntityRelationship) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	r.CreatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO entity_relationships (id, ledger_id, entity_a_id, entity_b_id, relationship_type, valid_from, valid_to, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, r.ID, r.LedgerID, r.EntityAID, r.EntityBID, r.RelationshipType, r.ValidFrom, r.ValidTo, r.CreatedAt)

	return err
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}

func scanEntitySimilarityRow(rows pgx.Rows) (*EntitySimilarityResult, error) {
	var e Entity
	var subtype, slug, logoURL, website, description, externalID, externalSource sql.NullString
	var embeddingModel sql.NullString
	var embeddingUpdatedAt sql.NullTime
	var similarity float32

	if err := rows.Scan(&e.ID, &e.Type, &subtype, &e.Name, &slug, &logoURL, &website, &description,
		&externalID, &externalSource, &e.Metadata, &e.UserVerified,
		&e.Embedding, &embeddingModel, &embeddingUpdatedAt,
		&e.CreatedAt, &e.UpdatedAt, &similarity); err != nil {
		return nil, err
	}

	e.Subtype = subtype.String
	e.Slug = slug.String
	e.LogoURL = logoURL.String
	e.Website = website.String
	e.Description = description.String
	e.ExternalID = externalID.String
	e.ExternalSource = externalSource.String
	e.EmbeddingModel = embeddingModel.String
	if embeddingUpdatedAt.Valid {
		e.EmbeddingUpdatedAt = &embeddingUpdatedAt.Time
	}

	return &EntitySimilarityResult{Entity: &e, Similarity: similarity}, nil
}

func scanEntityRelationshipRow(rows pgx.Rows) (*EntityRelationship, error) {
	var r EntityRelationship
	var validFrom, validTo sql.NullTime
	if err := rows.Scan(&r.ID, &r.LedgerID, &r.EntityAID, &r.EntityBID, &r.RelationshipType, &validFrom, &validTo, &r.CreatedAt); err != nil {
		return nil, err
	}
	r.ValidFrom = nullTimePtr(validFrom)
	r.ValidTo = nullTimePtr(validTo)
	return &r, nil
}

// GetByID returns an entity relationship by ID
func (s *EntityRelationshipStore) GetByID(ctx context.Context, id uuid.UUID) (*EntityRelationship, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, entity_a_id, entity_b_id, relationship_type, valid_from, valid_to, created_at
		FROM entity_relationships WHERE id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, pgx.ErrNoRows
	}
	return scanEntityRelationshipRow(rows)
}

// GetByLedgerID returns all entity relationships for a ledger
func (s *EntityRelationshipStore) GetByLedgerID(ctx context.Context, ledgerID uuid.UUID) ([]*EntityRelationship, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, entity_a_id, entity_b_id, relationship_type, valid_from, valid_to, created_at
		FROM entity_relationships WHERE ledger_id = $1
		ORDER BY created_at DESC
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []*EntityRelationship
	for rows.Next() {
		r, err := scanEntityRelationshipRow(rows)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, r)
	}

	return relationships, rows.Err()
}

// GetByEntityID returns all relationships involving an entity
func (s *EntityRelationshipStore) GetByEntityID(ctx context.Context, entityID uuid.UUID) ([]*EntityRelationship, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, entity_a_id, entity_b_id, relationship_type, valid_from, valid_to, created_at
		FROM entity_relationships WHERE entity_a_id = $1 OR entity_b_id = $1
		ORDER BY created_at DESC
	`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []*EntityRelationship
	for rows.Next() {
		r, err := scanEntityRelationshipRow(rows)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, r)
	}

	return relationships, rows.Err()
}

// Delete removes an entity relationship
func (s *EntityRelationshipStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM entity_relationships WHERE id = $1`, id)
	return err
}
