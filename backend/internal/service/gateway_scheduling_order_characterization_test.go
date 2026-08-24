//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSortCandidatesForFallbackCharacterization(t *testing.T) {
	service := &GatewayService{}
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	older := now.Add(-time.Hour)

	for _, mode := range []string{"last_used", "unknown-falls-back-to-last-used"} {
		t.Run(mode, func(t *testing.T) {
			accounts := []*Account{
				{ID: 4, Priority: 2, LastUsedAt: nil},
				{ID: 3, Priority: 1, LastUsedAt: &now},
				{ID: 2, Priority: 1, LastUsedAt: &older},
				{ID: 1, Priority: 1, LastUsedAt: nil},
			}

			service.sortCandidatesForFallback(accounts, false, mode)
			require.Equal(t, []int64{1, 2, 3, 4}, accountIDsInOrder(accounts))
		})
	}

	t.Run("last-used mode preserves OAuth preference for equal keys", func(t *testing.T) {
		accounts := []*Account{
			{ID: 1, Priority: 1, Type: AccountTypeAPIKey},
			{ID: 2, Priority: 1, Type: AccountTypeOAuth},
		}

		service.sortCandidatesForFallback(accounts, true, "last_used")
		require.Equal(t, []int64{2, 1}, accountIDsInOrder(accounts))
	})

	t.Run("random mode preserves the candidate set and priority partitions", func(t *testing.T) {
		accounts := []*Account{
			{ID: 1, Priority: 3},
			{ID: 2, Priority: 1},
			{ID: 3, Priority: 2},
			{ID: 4, Priority: 1},
		}

		service.sortCandidatesForFallback(accounts, false, "random")
		require.Equal(t, []int{1, 1, 2, 3}, accountPrioritiesInOrder(accounts))
		require.ElementsMatch(t, []int64{1, 2, 3, 4}, accountIDsInOrder(accounts))
	})
}

func accountIDsInOrder(accounts []*Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids
}

func accountPrioritiesInOrder(accounts []*Account) []int {
	priorities := make([]int, 0, len(accounts))
	for _, account := range accounts {
		priorities = append(priorities, account.Priority)
	}
	return priorities
}
