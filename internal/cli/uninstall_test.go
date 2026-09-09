package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveAliasOnlyRemovesOurOwnLink(t *testing.T) {
	newInstall := func(t *testing.T) (dir, exe string) {
		t.Helper()
		dir = t.TempDir()
		exe = filepath.Join(dir, "limitping")
		if err := os.WriteFile(exe, []byte("binary"), 0o755); err != nil {
			t.Fatal(err)
		}
		return dir, exe
	}

	t.Run("removes a link pointing at the binary", func(t *testing.T) {
		dir, exe := newInstall(t)
		alias := filepath.Join(dir, BinaryAlias)
		if err := os.Symlink("limitping", alias); err != nil {
			t.Fatal(err)
		}
		if err := removeAlias(exe, io.Discard); err != nil {
			t.Fatalf("removeAlias: %v", err)
		}
		if _, err := os.Lstat(alias); !os.IsNotExist(err) {
			t.Fatal("alias survived")
		}
	})

	t.Run("keeps an unrelated binary of the same name", func(t *testing.T) {
		dir, exe := newInstall(t)
		alias := filepath.Join(dir, BinaryAlias)
		if err := os.WriteFile(alias, []byte("someone else's lmp"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := removeAlias(exe, io.Discard); err != nil {
			t.Fatalf("removeAlias: %v", err)
		}
		if _, err := os.Stat(alias); err != nil {
			t.Fatalf("removeAlias deleted a real binary it does not own: %v", err)
		}
	})

	t.Run("keeps a link pointing somewhere else", func(t *testing.T) {
		dir, exe := newInstall(t)
		other := filepath.Join(dir, "other")
		if err := os.WriteFile(other, []byte("other"), 0o755); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(dir, BinaryAlias)
		if err := os.Symlink("other", alias); err != nil {
			t.Fatal(err)
		}
		if err := removeAlias(exe, io.Discard); err != nil {
			t.Fatalf("removeAlias: %v", err)
		}
		if _, err := os.Lstat(alias); err != nil {
			t.Fatalf("removeAlias deleted a link it does not own: %v", err)
		}
	})

	t.Run("no alias installed", func(t *testing.T) {
		_, exe := newInstall(t)
		if err := removeAlias(exe, io.Discard); err != nil {
			t.Fatalf("removeAlias: %v", err)
		}
	})
}
