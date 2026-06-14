package sync

import "fmt"

// uniquePlaidAccountDisplayName returns a display name for a Plaid account that
// does not collide with any existing name. It tries candidates in order:
//  1. baseName (defaulting to "Account" when empty)
//  2. baseName + " (...lastFour)"
//  3. baseName + " · " + last 8 chars of plaidID
//  4. baseName + " #2", "#3", … up to 50
func uniquePlaidAccountDisplayName(baseName, lastFour, plaidID string, nameExists func(string) (bool, error)) (string, error) {
	if baseName == "" {
		baseName = "Account"
	}

	candidates := []string{
		baseName,
		fmt.Sprintf("%s (...%s)", baseName, lastFour),
		baseName + " · " + tail(plaidID, 8),
	}
	for _, c := range candidates {
		ok, err := nameExists(c)
		if err != nil {
			return "", err
		}
		if !ok {
			return c, nil
		}
	}

	for i := 2; i <= 50; i++ {
		c := fmt.Sprintf("%s #%d", baseName, i)
		ok, err := nameExists(c)
		if err != nil {
			return "", err
		}
		if !ok {
			return c, nil
		}
	}

	return "", fmt.Errorf("could not find unique display name for account %q", baseName)
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
