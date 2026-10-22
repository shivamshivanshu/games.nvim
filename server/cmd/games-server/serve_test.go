package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSignupPassphrase(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "passphrase")
	if err := os.WriteFile(file, []byte("from file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := func(value string) func(string) string {
		return func(name string) string {
			if name == signupPassphraseEnv {
				return value
			}
			return ""
		}
	}
	for _, tc := range []struct {
		name    string
		path    string
		env     string
		want    string
		wantErr bool
	}{
		{name: "file, newline trimmed", path: file, env: "from env", want: "from file"},
		{name: "env without a file", env: "from env", want: "from env"},
		{name: "neither turns sign-up off", want: ""},
		{name: "empty file", path: empty, wantErr: true},
		{name: "missing file", path: filepath.Join(dir, "missing"), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := signupPassphrase(tc.path, env(tc.env))
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("signupPassphrase = %q, %v; want %q, error %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
