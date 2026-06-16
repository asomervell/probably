package sync

import "fmt"

// uniquePlaidAccountDisplayName returns a display name for a Plaid account that
// does not collide with any existing name, as reported by nameExists.
//
// Candidates are tried in order:
//  1. baseName (defaulting to "Account" when empty)
//  2. baseName + " (...lastFour)"
//  3. baseName + " · " + last 8 chars of plaidID
//  4. baseName + " #2", "#3", … up to #50
func uniquePlaidAccountDisplayName(baseName, lastFour, plaidID string, nameExists func(string) (bool, error)) (string, error) {
	if baseName == "" {
		baseName = "Account"
	}

	candidates := []string{baseName}
	if lastFour != "" {
		candidates = append(candidates, fmt.Sprintf("%s (...%s)", baseName, lastFour))
	}
	tail := plaidID
	if len(tail) > 8 {
		tail = tail[len(tail)-8:]
	}
	if tail != "" {
		candidates = append(candidates, fmt.Sprintf("%s · %s", baseName, tail))
	}
	for i := 2; i <= 50; i++ {
		candidates = append(candidates, fmt.Sprintf("%s #%d", baseName, i))
	}

	for _, name := range candidates {
		taken, err := nameExists(name)
		if err != nil {
			return "", err
		}
		if !taken {
			return name, nil
		}
	}
	return "", fmt.Errorf("could not find unique display name for Plaid account after 50 attempts")
}
