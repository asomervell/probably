package auth

import (
	"context"
	"errors"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrLedgerNotFound   = errors.New("ledger not found")
)

// PermissionChecker provides permission checking utilities
type PermissionChecker struct {
	permissions *models.PermissionStore
	entities    *models.EntityStore
}

// NewPermissionChecker creates a new PermissionChecker
func NewPermissionChecker(permissions *models.PermissionStore, entities *models.EntityStore) *PermissionChecker {
	return &PermissionChecker{
		permissions: permissions,
		entities:    entities,
	}
}

// CheckLedgerPermission checks if a user has the required permission level on a ledger
// Returns the actual permission level if access is granted, or an error if denied
func (pc *PermissionChecker) CheckLedgerPermission(ctx context.Context, userID, ledgerID uuid.UUID, requiredLevel models.PermissionLevel) (models.PermissionLevel, error) {
	// Get all entities that own this ledger
	ledgerEntities, err := pc.permissions.GetLedgerEntities(ctx, ledgerID)
	if err != nil {
		return "", err
	}

	if len(ledgerEntities) == 0 {
		return "", ErrLedgerNotFound
	}

	// Check user's permissions on each entity that owns the ledger
	// Return the highest permission level found
	var highestLevel models.PermissionLevel
	found := false

	for _, el := range ledgerEntities {
		perm, err := pc.permissions.GetUserEntityPermission(ctx, userID, el.EntityID)
		if err != nil {
			// User doesn't have permission on this entity, continue
			continue
		}

		// Permissions inherit from entity to ledger
		entityLevel := perm.PermissionLevel

		// Track the highest permission level
		if !found || isHigherPermission(entityLevel, highestLevel) {
			highestLevel = entityLevel
			found = true
		}
	}

	if !found {
		return "", ErrPermissionDenied
	}

	// Check if the highest permission level meets the requirement
	if !hasPermission(highestLevel, requiredLevel) {
		return "", ErrPermissionDenied
	}

	return highestLevel, nil
}

// GetUserLedgerPermissions returns all ledgers a user can access with their permission levels
func (pc *PermissionChecker) GetUserLedgerPermissions(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]models.PermissionLevel, error) {
	// Get all user's entity permissions
	userPerms, err := pc.permissions.GetUserEntityPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]models.PermissionLevel)

	// For each entity the user has permission on, get all its ledgers
	for _, perm := range userPerms {
		ledgerEntities, err := pc.permissions.GetEntityLedgers(ctx, perm.EntityID)
		if err != nil {
			continue
		}

		// Permissions inherit from entity to ledger
		for _, el := range ledgerEntities {
			// If user has multiple paths to the same ledger, keep the highest permission
			currentLevel, exists := result[el.LedgerID]
			if !exists || isHigherPermission(perm.PermissionLevel, currentLevel) {
				result[el.LedgerID] = perm.PermissionLevel
			}
		}
	}

	return result, nil
}

// hasPermission checks if the user's permission level meets the required level
// Permission hierarchy: owner > edit > view
func hasPermission(userLevel, requiredLevel models.PermissionLevel) bool {
	if userLevel == models.PermissionLevelOwner {
		return true // Owner has all permissions
	}
	if userLevel == models.PermissionLevelEdit {
		return requiredLevel == models.PermissionLevelEdit || requiredLevel == models.PermissionLevelView
	}
	if userLevel == models.PermissionLevelView {
		return requiredLevel == models.PermissionLevelView
	}
	return false
}

// isHigherPermission checks if level1 is higher than level2
func isHigherPermission(level1, level2 models.PermissionLevel) bool {
	if level1 == models.PermissionLevelOwner {
		return level2 != models.PermissionLevelOwner
	}
	if level1 == models.PermissionLevelEdit {
		return level2 == models.PermissionLevelView
	}
	return false // view is the lowest
}
