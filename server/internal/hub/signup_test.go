package hub_test

import (
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
)

const erin game.UserID = "erin"

func registerRequest(user game.UserID, password, passphrase string) protocol.RegisterRequest {
	return protocol.RegisterRequest{V: protocol.Version, Username: user, Password: password, Passphrase: passphrase}
}

func TestRegisterLogsInAndCreatesTheAccount(t *testing.T) {
	t.Parallel()
	srv := newEnv(t).start(t)
	c := prototest.Dial(t, srv.addr)
	res := call[protocol.LoginResult](t, c, protocol.TypeAuthRegister, registerRequest(erin, "erin's secret", testPassphrase))
	if res.User != erin || res.HeartbeatMS != limits.Default().Heartbeat.Milliseconds() {
		t.Fatalf("register = %+v, want logged in as %s with the heartbeat", res, erin)
	}
	call[protocol.UserListResult](t, c, protocol.TypeUserList, protocol.Empty{})

	_, err := c.Request(protocol.TypeAuthRegister, registerRequest("frank", testPassword, testPassphrase))
	prototest.WantCode(t, err, protocol.CodeBadRequest)
	_, err = prototest.Dial(t, srv.addr).Request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: "frank", Password: testPassword})
	prototest.WantCode(t, err, protocol.CodeAuthFailed)

	again := prototest.Dial(t, srv.addr)
	call[protocol.LoginResult](t, again, protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: erin, Password: "erin's secret"})
}

func TestRegisterRejects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		signupOff bool
		req       protocol.RegisterRequest
		want      protocol.ErrorCode
	}{
		{"sign-up off", true, registerRequest(erin, testPassword, testPassphrase), protocol.CodeSignupDisabled},
		{"sign-up off before the passphrase", true, registerRequest(erin, testPassword, "wrong"), protocol.CodeSignupDisabled},
		{"wrong passphrase", false, registerRequest(erin, testPassword, "wrong"), protocol.CodePassphraseRejected},
		{"wrong passphrase before the taken name", false, registerRequest(prototest.Alice, testPassword, "wrong"), protocol.CodePassphraseRejected},
		{"wrong passphrase before the bad name", false, registerRequest("ev\nil", testPassword, "wrong"), protocol.CodePassphraseRejected},
		{"taken name", false, registerRequest(prototest.Alice, "another", testPassphrase), protocol.CodeUserExists},
		{"empty name", false, registerRequest("", testPassword, testPassphrase), protocol.CodeBadRequest},
		{"hidden characters in the name", false, registerRequest("ev\nil", testPassword, testPassphrase), protocol.CodeBadRequest},
		{"empty password", false, registerRequest(erin, "", testPassphrase), protocol.CodeBadRequest},
		{"password past bcrypt's 72 bytes", false, registerRequest(erin, strings.Repeat("p", 73), testPassphrase), protocol.CodeBadRequest},
		{"name past the length cap", false, registerRequest(game.UserID(strings.Repeat("e", game.MaxUserIDRunes+1)), testPassword, testPassphrase), protocol.CodeBadRequest},
		{"whitespace around the name", false, registerRequest("erin ", testPassword, testPassphrase), protocol.CodeBadRequest},
		{"other protocol version", false, protocol.RegisterRequest{V: protocol.Version + 1, Username: erin, Password: testPassword, Passphrase: testPassphrase}, protocol.CodeVersionMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			if tc.signupOff {
				e.passphrase = ""
			}
			c := prototest.Dial(t, e.start(t).addr)
			_, err := c.Request(protocol.TypeAuthRegister, tc.req)
			prototest.WantCode(t, err, tc.want)
			_, err = c.Request(protocol.TypeUserList, protocol.Empty{})
			prototest.WantCode(t, err, protocol.CodeUnauthenticated)
		})
	}
}

func TestWrongPassphraseCountsTowardLockout(t *testing.T) {
	t.Parallel()
	srv := newEnv(t).start(t)
	c := prototest.Dial(t, srv.addr)
	for range limits.Default().LockoutFailures {
		_, err := c.Request(protocol.TypeAuthRegister, registerRequest(erin, testPassword, "wrong"))
		prototest.WantCode(t, err, protocol.CodePassphraseRejected)
	}
	_, err := c.Request(protocol.TypeAuthRegister, registerRequest(erin, testPassword, testPassphrase))
	if locked := prototest.WantCode(t, err, protocol.CodeAuthFailed); locked != "authentication failed: "+lockedOutText {
		t.Fatalf("right passphrase while locked says %q, want %q", locked, lockedOutText)
	}
	_, err = c.Request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: prototest.Alice, Password: testPassword})
	prototest.WantCode(t, err, protocol.CodeAuthFailed)
}
