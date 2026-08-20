package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// withProfile activates a profile for the test and restores the previous one.
func withProfile(t *testing.T, name string) {
	t.Helper()
	prev := ActiveProfile()
	if err := SetActiveProfile(name); err != nil {
		t.Fatalf("SetActiveProfile(%q) failed: %v", name, err)
	}
	t.Cleanup(func() { _ = SetActiveProfile(prev) })
}

func TestValidateProfileName(t *testing.T) {
	valid := []string{"", "personal", "work-2", "acct.1", "A_b"}
	for _, name := range valid {
		if err := ValidateProfileName(name); err != nil {
			t.Errorf("ValidateProfileName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{"../evil", "a/b", "a\\b", ".hidden", "-flag", "with space"}
	for _, name := range invalid {
		if err := ValidateProfileName(name); err == nil {
			t.Errorf("ValidateProfileName(%q) = nil, want error", name)
		}
	}
}

func TestProfileConfigDirPaths(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	withProfile(t, "")
	if got, want := ConfigDirPath(), filepath.Join(tmpHome, ConfigDir); got != want {
		t.Errorf("default profile ConfigDirPath = %q, want %q", got, want)
	}

	withProfile(t, "personal")
	want := filepath.Join(tmpHome, ConfigDir, ProfilesDirName, "personal")
	if got := ConfigDirPath(); got != want {
		t.Errorf("named profile ConfigDirPath = %q, want %q", got, want)
	}
	if got := ConfigFilePath(); got != filepath.Join(want, ConfigFile) {
		t.Errorf("named profile ConfigFilePath = %q", got)
	}
	if got := StateFilePath(); got != filepath.Join(want, StateFile) {
		t.Errorf("named profile StateFilePath = %q", got)
	}
	if got := AgeKeyFilePath(); got != filepath.Join(want, AgeKeyFile) {
		t.Errorf("named profile AgeKeyFilePath = %q", got)
	}
}

func TestSetActiveProfileRejectsInvalidName(t *testing.T) {
	if err := SetActiveProfile("../evil"); err == nil {
		_ = SetActiveProfile("")
		t.Fatal("SetActiveProfile should reject path traversal names")
	}
}

func TestListProfiles(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// No profiles dir yet: empty, no error.
	names, err := ListProfiles()
	if err != nil || len(names) != 0 {
		t.Fatalf("ListProfiles on empty home = %v, %v; want empty, nil", names, err)
	}

	// Configured profiles are listed sorted; dirs without config.yaml are skipped.
	for _, name := range []string{"work", "personal"} {
		dir := filepath.Join(tmpHome, ConfigDir, ProfilesDirName, name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte("storage:\n  provider: r2\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(tmpHome, ConfigDir, ProfilesDirName, "empty"), 0700); err != nil {
		t.Fatal(err)
	}

	names, err = ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"personal", "work"}; !reflect.DeepEqual(names, want) {
		t.Errorf("ListProfiles = %v, want %v", names, want)
	}
}

func TestSaveLoadWithProfile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	withProfile(t, "personal")

	cfg := &Config{
		EncryptionKey: "~/.claude-sync/profiles/personal/age-key.txt",
		ClaudeDir:     "~/.claude-personal",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Config must land in the profile's directory, not the default one.
	profilePath := filepath.Join(tmpHome, ConfigDir, ProfilesDirName, "personal", ConfigFile)
	if _, err := os.Stat(profilePath); err != nil {
		t.Fatalf("expected config at %s: %v", profilePath, err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if want := filepath.Join(tmpHome, ".claude-personal"); loaded.ClaudeDir != want {
		t.Errorf("ClaudeDir = %q, want ~ expanded to %q", loaded.ClaudeDir, want)
	}
	if !strings.HasPrefix(loaded.EncryptionKey, tmpHome) {
		t.Errorf("EncryptionKey should be ~ expanded, got %q", loaded.EncryptionKey)
	}
}

func TestResolveClaudeDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	t.Run("default", func(t *testing.T) {
		cfg := &Config{}
		if got, want := cfg.ResolveClaudeDir(), filepath.Join(tmpHome, ".claude"); got != want {
			t.Errorf("ResolveClaudeDir = %q, want %q", got, want)
		}
	})

	t.Run("configured claude_dir wins over default", func(t *testing.T) {
		cfg := &Config{ClaudeDir: filepath.Join(tmpHome, ".claude-personal")}
		if got := cfg.ResolveClaudeDir(); got != cfg.ClaudeDir {
			t.Errorf("ResolveClaudeDir = %q, want %q", got, cfg.ClaudeDir)
		}
	})

	t.Run("test override wins over configured", func(t *testing.T) {
		cfg := &Config{
			ClaudeDir:         filepath.Join(tmpHome, ".claude-personal"),
			ClaudeDirOverride: "/override",
		}
		if got := cfg.ResolveClaudeDir(); got != "/override" {
			t.Errorf("ResolveClaudeDir = %q, want /override", got)
		}
	})
}

func TestResolveClaudeJSONPath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	t.Run("default dir uses ~/.claude.json", func(t *testing.T) {
		cfg := &Config{}
		if got, want := cfg.ResolveClaudeJSONPath(), filepath.Join(tmpHome, ".claude.json"); got != want {
			t.Errorf("ResolveClaudeJSONPath = %q, want %q", got, want)
		}
	})

	t.Run("explicit default dir still uses ~/.claude.json", func(t *testing.T) {
		cfg := &Config{ClaudeDir: filepath.Join(tmpHome, ".claude")}
		if got, want := cfg.ResolveClaudeJSONPath(), filepath.Join(tmpHome, ".claude.json"); got != want {
			t.Errorf("ResolveClaudeJSONPath = %q, want %q", got, want)
		}
	})

	t.Run("custom dir holds its own .claude.json (CLAUDE_CONFIG_DIR layout)", func(t *testing.T) {
		dir := filepath.Join(tmpHome, ".claude-personal")
		cfg := &Config{ClaudeDir: dir}
		if got, want := cfg.ResolveClaudeJSONPath(), filepath.Join(dir, ".claude.json"); got != want {
			t.Errorf("ResolveClaudeJSONPath = %q, want %q", got, want)
		}
	})

	t.Run("override wins", func(t *testing.T) {
		cfg := &Config{ClaudeJSONOverride: "/tmp/x.json", ClaudeDir: "/y"}
		if got := cfg.ResolveClaudeJSONPath(); got != "/tmp/x.json" {
			t.Errorf("ResolveClaudeJSONPath = %q, want override", got)
		}
	})
}
