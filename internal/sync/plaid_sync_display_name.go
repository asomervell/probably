package sync

import "fmt"

// uniquePlaidAccountDisplayName returns a display name for a Plaid account
// that does not collide with any existing name. nameExists reports whether a
// candidate is already in use. It tries, in order:
//  1. baseName (defaulting to "Account" when empty)
//  2. baseName + " (...<lastFour>)"  — when lastFour is non-empty
//  3. baseName + " · " + tail(plaidID, 8)  — last 8 chars of the Plaid account ID
//  4. baseName + " #2", " #3", ...   — numeric disambiguation
func uniquePlaidAccountDisplayName(baseName, lastFour, plaidID string, nameExists func(string) (bool, error)) (string, error) {
	if baseName == "" {
		baseName = "Account"
	}

	try := func(candidate string) (string, bool, error) {
		taken, err := nameExists(candidate)
		if err != nil {
			return "", false, err
		}
		return candidate, !taken, nil
	}

	if name, ok, err := try(baseName); err != nil {
		return "", err
	} else if ok {
		return name, nil
	}

	if lastFour != "" {
		if name, ok, err := try(baseName + " (..." + lastFour + ")"); err != nil {
			return "", err
		} else if ok {
			return name, nil
		}
	}

	idSuffix := plaidID
	if len(idSuffix) > 8 {
		idSuffix = idSuffix[len(idSuffix)-8:]
	}
	if idSuffix != "" {
		if name, ok, err := try(baseName + " · " + idSuffix); err != nil {
			return "", err
		} else if ok {
			return name, nil
		}
	}

	for n := 2; n <= 50; n++ {
		if name, ok, err := try(fmt.Sprintf("%s #%d", baseName, n)); err != nil {
			return "", err
		} else if ok {
			return name, nil
		}
	}

	return "", fmt.Errorf("could not find unique display name for %q", baseName)
}
