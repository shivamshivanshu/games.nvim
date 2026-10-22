package auth

import (
	"context"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

var ErrEmptyPassword = game.BadRequestError{Msg: "password must not be empty"}

// CreateAccount creates user with password, or is store.ErrUserExists when
// the name is taken. It looks the name up before hashing, so a taken name
// costs no bcrypt.
func CreateAccount(ctx context.Context, st *store.Store, user game.UserID, password string) error {
	if err := game.CheckUserID(user); err != nil {
		return game.BadRequestError{Msg: err.Error()}
	}
	if password == "" {
		return ErrEmptyPassword
	}
	exists, err := st.UserExists(ctx, user)
	if err != nil {
		return err
	}
	if exists {
		return store.ErrUserExists
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return st.CreateUser(ctx, user, hash)
}
