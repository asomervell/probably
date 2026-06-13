package models

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategorizationRule struct {
	ID           uuid.UUID `json:"id"`
	LedgerID     uuid.UUID `json:"ledger_id"`
	Name         string    `json:"name"`
	Prompt       string    `json:"prompt"`                  // LLM instruction for this rule
	Examples     string    `json:"examples,omitempty"`      // Optional transaction examples
	MatchPattern string    `json:"match_pattern,omitempty"` // Legacy/fallback pattern
	IsRegex      bool      `json:"is_regex"`                // Legacy
	TagID        uuid.UUID `json:"tag_id"`
	Priority     int       `json:"priority"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Loaded separately
	TagName  string `json:"tag_name,omitempty"`
	TagColor string `json:"tag_color,omitempty"`
}

// Match checks if the given description matches this rule's pattern.
// Returns true if the description matches (case-insensitive).
func (r *CategorizationRule) Match(description string) bool {
	if r.MatchPattern == "" {
		return false
	}

	if r.IsRegex {
		// Compile regex with case-insensitive flag
		re, err := regexp.Compile("(?i)" + r.MatchPattern)
		if err != nil {
			return false
		}
		return re.MatchString(description)
	}

	// Simple case-insensitive substring match
	return strings.Contains(strings.ToLower(description), strings.ToLower(r.MatchPattern))
}

type RuleStore struct {
	pool *pgxpool.Pool
}

func NewRuleStore(pool *pgxpool.Pool) *RuleStore {
	return &RuleStore{pool: pool}
}

func (s *RuleStore) Create(ctx context.Context, rule *CategorizationRule) error {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO categorization_rules (id, ledger_id, name, prompt, examples, match_pattern, is_regex, tag_id, priority, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, rule.ID, rule.LedgerID, rule.Name, rule.Prompt, NullString(rule.Examples), NullString(rule.MatchPattern), rule.IsRegex, rule.TagID, rule.Priority, true, time.Now(), time.Now())

	return err
}

func (s *RuleStore) Update(ctx context.Context, rule *CategorizationRule) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE categorization_rules SET
			name = $2, prompt = $3, examples = $4, match_pattern = $5, is_regex = $6, tag_id = $7, priority = $8, is_active = $9, updated_at = $10
		WHERE id = $1
	`, rule.ID, rule.Name, rule.Prompt, NullString(rule.Examples), NullString(rule.MatchPattern), rule.IsRegex, rule.TagID, rule.Priority, rule.IsActive, time.Now())

	return err
}

func (s *RuleStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM categorization_rules WHERE id = $1`, id)
	return err
}

const ruleSelectColumns = `
	SELECT r.id, r.ledger_id, r.name, r.prompt, r.examples, r.match_pattern, r.is_regex, r.tag_id, r.priority, r.is_active, r.created_at, r.updated_at,
		t.name, t.color
	FROM categorization_rules r
	JOIN tags t ON r.tag_id = t.id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRule(row rowScanner) (*CategorizationRule, error) {
	var r CategorizationRule
	var prompt, examples, matchPattern sql.NullString
	if err := row.Scan(&r.ID, &r.LedgerID, &r.Name, &prompt, &examples, &matchPattern, &r.IsRegex, &r.TagID, &r.Priority, &r.IsActive,
		&r.CreatedAt, &r.UpdatedAt, &r.TagName, &r.TagColor); err != nil {
		return nil, err
	}
	r.Prompt = prompt.String
	r.Examples = examples.String
	r.MatchPattern = matchPattern.String
	return &r, nil
}

func (s *RuleStore) GetByID(ctx context.Context, id uuid.UUID) (*CategorizationRule, error) {
	row := s.pool.QueryRow(ctx, ruleSelectColumns+` WHERE r.id = $1`, id)
	return scanRule(row)
}

func (s *RuleStore) GetByLedgerID(ctx context.Context, ledgerID uuid.UUID) ([]*CategorizationRule, error) {
	rows, err := s.pool.Query(ctx, ruleSelectColumns+` WHERE r.ledger_id = $1 ORDER BY r.priority DESC, r.name`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*CategorizationRule
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// GetActiveRules returns active rules sorted by priority (highest first)
func (s *RuleStore) GetActiveRules(ctx context.Context, ledgerID uuid.UUID) ([]*CategorizationRule, error) {
	rows, err := s.pool.Query(ctx, ruleSelectColumns+` WHERE r.ledger_id = $1 AND r.is_active = true ORDER BY r.priority DESC`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*CategorizationRule
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}
