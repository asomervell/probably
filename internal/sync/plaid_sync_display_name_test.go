package sync

import (
	"errors"
	"testing"
)

func TestUniquePlaidAccountDisplayName_noCollision(t *testing.T) {
	t.Parallel()
	nameExists := func(string) (bool, error) { return false, nil }
	got, err := uniquePlaidAccountDisplayName("Checking", "1111", "x", nameExists)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Checking" {
		t.Fatalf("got %q want Checking", got)
	}
}

func TestUniquePlaidAccountDisplayName_emptyBaseName(t *testing.T) {
	t.Parallel()
	nameExists := func(string) (bool, error) { return false, nil }
	got, err := uniquePlaidAccountDisplayName("", "4242", "id", nameExists)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Account" {
		t.Fatalf("got %q want Account", got)
	}
}

func TestUniquePlaidAccountDisplayName_appendsLastFour(t *testing.T) {
	t.Parallel()
	taken := map[string]bool{"CREDIT CARD": true}
	nameExists := func(n string) (bool, error) { return taken[n], nil }

	got, err := uniquePlaidAccountDisplayName("CREDIT CARD", "5331", "plaid-id-1", nameExists)
	if err != nil {
		t.Fatal(err)
	}
	if want := "CREDIT CARD (...5331)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUniquePlaidAccountDisplayName_fallsBackToIDSuffix(t *testing.T) {
	t.Parallel()
	taken := map[string]bool{
		"CREDIT CARD":           true,
		"CREDIT CARD (...5331)": true,
	}
	nameExists := func(n string) (bool, error) { return taken[n], nil }

	got, err := uniquePlaidAccountDisplayName("CREDIT CARD", "5331", "longPlaidAccountIdentifier", nameExists)
	if err != nil {
		t.Fatal(err)
	}
	if want := "CREDIT CARD · entifier"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUniquePlaidAccountDisplayName_numericSuffix(t *testing.T) {
	t.Parallel()
	taken := map[string]bool{
		"CREDIT CARD": true, "CREDIT CARD (...5331)": true, "CREDIT CARD · xy": true,
	}
	nameExists := func(n string) (bool, error) { return taken[n], nil }

	got, err := uniquePlaidAccountDisplayName("CREDIT CARD", "5331", "xy", nameExists)
	if err != nil {
		t.Fatal(err)
	}
	if want := "CREDIT CARD #2"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUniquePlaidAccountDisplayName_propagatesError(t *testing.T) {
	t.Parallel()
	boom := errors.New("db")
	_, err := uniquePlaidAccountDisplayName("A", "", "id", func(string) (bool, error) {
		return false, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected db error, got %v", err)
	}
}
