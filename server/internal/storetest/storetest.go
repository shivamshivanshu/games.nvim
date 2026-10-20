// Package storetest opens throwaway stores for tests.
package storetest

import (
	"path/filepath"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

// Open opens a store in t's temp dir holding users with passwordHash.
func Open(t testing.TB, passwordHash []byte, users ...game.UserID) *store.Store {
	t.Helper()
	return OpenAt(t, filepath.Join(t.TempDir(), "games.db"), passwordHash, users...)
}

// OpenAt opens the store at path, closed on t's cleanup, holding users with passwordHash.
func OpenAt(t testing.TB, path string, passwordHash []byte, users ...game.UserID) *store.Store {
	t.Helper()
	s, err := store.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, user := range users {
		if err := s.CreateUser(t.Context(), user, passwordHash); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
