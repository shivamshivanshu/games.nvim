package auth

import (
	"context"
	"fmt"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

// maxPasswordBytes is as much of a password as bcrypt reads.
const maxPasswordBytes = 72

var (
	// ErrEmptyPassword refuses an account no password could log in to.
	ErrEmptyPassword = game.BadRequestError{Msg: "password must not be empty"}
	// ErrPasswordTooLong refuses a password bcrypt cannot hash whole.
	ErrPasswordTooLong = game.BadRequestError{Msg: fmt.Sprintf("password must be at most %d bytes", maxPasswordBytes)}
)

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
	if len(password) > maxPasswordBytes {
		return ErrPasswordTooLong
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
