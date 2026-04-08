package permissions

import (
	"testing"
)

func TestMergeBlocklist_EmptyUser(t *testing.T) {
	result := MergeBlocklist(nil)
	if len(result) != len(DefaultBlocklist) {
		t.Errorf("expected %d entries, got %d", len(DefaultBlocklist), len(result))
	}
}

func TestMergeBlocklist_EmptySlice(t *testing.T) {
	result := MergeBlocklist([]string{})
	if len(result) != len(DefaultBlocklist) {
		t.Errorf("expected %d entries, got %d", len(DefaultBlocklist), len(result))
	}
}

func TestMergeBlocklist_AddsUserEntries(t *testing.T) {
	user := []string{"custom-blocked-cmd", "another-blocked"}
	result := MergeBlocklist(user)
	if len(result) != len(DefaultBlocklist)+2 {
		t.Errorf("expected %d entries, got %d", len(DefaultBlocklist)+2, len(result))
	}
}

func TestMergeBlocklist_NoDuplicates(t *testing.T) {
	user := []string{DefaultBlocklist[0], "new-entry"}
	result := MergeBlocklist(user)

	seen := make(map[string]int)
	for _, e := range result {
		seen[e]++
	}
	for k, v := range seen {
		if v > 1 {
			t.Errorf("entry %q appears %d times", k, v)
		}
	}
}

func TestDefaultBlocklist_NotEmpty(t *testing.T) {
	if len(DefaultBlocklist) == 0 {
		t.Error("DefaultBlocklist should not be empty")
	}
}
