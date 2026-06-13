package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountOwner represents beneficial owner information from Teller Identity API
type AccountOwner struct {
	ID          uuid.UUID  `json:"id"`
	AccountID   uuid.UUID  `json:"account_id"`
	Name        string     `json:"name"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`

	// Primary address
	AddressStreet     string `json:"address_street,omitempty"`
	AddressCity       string `json:"address_city,omitempty"`
	AddressRegion     string `json:"address_region,omitempty"`
	AddressPostalCode string `json:"address_postal_code,omitempty"`
	AddressCountry    string `json:"address_country,omitempty"`

	// Primary contact info
	PhoneNumber string `json:"phone_number,omitempty"`
	Email       string `json:"email,omitempty"`

	// Additional data stored as JSON (extra addresses, phones, emails)
	AdditionalData json.RawMessage `json:"additional_data,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdditionalOwnerData holds extra addresses, phones, and emails
type AdditionalOwnerData struct {
	Addresses    []OwnerAddress `json:"addresses,omitempty"`
	PhoneNumbers []OwnerPhone   `json:"phone_numbers,omitempty"`
	Emails       []OwnerEmail   `json:"emails,omitempty"`
}

type OwnerAddress struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	Region     string `json:"region"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
	Type       string `json:"type"` // "primary", "secondary", etc.
}

type OwnerPhone struct {
	Number string `json:"number"`
	Type   string `json:"type"` // "mobile", "home", "work"
}

type OwnerEmail struct {
	Address string `json:"address"`
	Type    string `json:"type"` // "primary", "secondary"
}

type AccountOwnerStore struct {
	pool *pgxpool.Pool
}

func NewAccountOwnerStore(pool *pgxpool.Pool) *AccountOwnerStore {
	return &AccountOwnerStore{pool: pool}
}

func (s *AccountOwnerStore) GetByAccountID(ctx context.Context, accountID uuid.UUID) ([]*AccountOwner, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, account_id, name, date_of_birth,
			address_street, address_city, address_region, address_postal_code, address_country,
			phone_number, email, additional_data, created_at, updated_at
		FROM account_owners WHERE account_id = $1
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var owners []*AccountOwner
	for rows.Next() {
		var o AccountOwner
		var dob sql.NullTime
		var street, city, region, postal, country, phone, email sql.NullString
		var additionalData []byte

		if err := rows.Scan(&o.ID, &o.AccountID, &o.Name, &dob,
			&street, &city, &region, &postal, &country,
			&phone, &email, &additionalData, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}

		if dob.Valid {
			o.DateOfBirth = &dob.Time
		}
		o.AddressStreet = street.String
		o.AddressCity = city.String
		o.AddressRegion = region.String
		o.AddressPostalCode = postal.String
		o.AddressCountry = country.String
		o.PhoneNumber = phone.String
		o.Email = email.String
		o.AdditionalData = additionalData

		owners = append(owners, &o)
	}

	return owners, rows.Err()
}

// ReplaceForAccount atomically deletes all existing owners for an account and inserts the
// provided slice. Either all writes succeed or none do — callers never see a partially-updated state.
func (s *AccountOwnerStore) ReplaceForAccount(ctx context.Context, accountID uuid.UUID, owners []*AccountOwner) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM account_owners WHERE account_id = $1`, accountID); err != nil {
			return err
		}
		for _, owner := range owners {
			if owner.ID == uuid.Nil {
				owner.ID = uuid.New()
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO account_owners (id, account_id, name, date_of_birth,
					address_street, address_city, address_region, address_postal_code, address_country,
					phone_number, email, additional_data, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			`, owner.ID, owner.AccountID, owner.Name, owner.DateOfBirth,
				NullString(owner.AddressStreet), NullString(owner.AddressCity),
				NullString(owner.AddressRegion), NullString(owner.AddressPostalCode),
				NullString(owner.AddressCountry), NullString(owner.PhoneNumber),
				NullString(owner.Email), owner.AdditionalData, time.Now(), time.Now())
			if err != nil {
				return err
			}
		}
		return nil
	})
}
