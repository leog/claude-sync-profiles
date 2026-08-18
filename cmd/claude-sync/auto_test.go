package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leog/claude-sync-profiles/internal/config"
)

func TestAutoSyncTargetPath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	t.Run("global default without config falls back to ~/.claude/settings.json", func(t *testing.T) {
		got, err := autoSyncTargetPath("", false)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(tmpHome, ".claude", "settings.json")
		if got != want {
			t.Errorf("autoSyncTargetPath = %q, want %q", got, want)
		}
	})

	t.Run("global path follows the profile's claude_dir", func(t *testing.T) {
		if err := config.SetActiveProfile("personal"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = config.SetActiveProfile("") })

		claudeDir := filepath.Join(tmpHome, ".claude-personal")
		if err := config.Save(&config.Config{ClaudeDir: claudeDir}); err != nil {
			t.Fatal(err)
		}

		got, err := autoSyncTargetPath("", false)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(claudeDir, "settings.json")
		if got != want {
			t.Errorf("autoSyncTargetPath = %q, want %q", got, want)
		}
	})

	t.Run("project targets settings.local.json by default, settings.json with shared", func(t *testing.T) {
		proj := t.TempDir()

		got, err := autoSyncTargetPath(proj, false)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(proj, ".claude", "settings.local.json"); got != want {
			t.Errorf("autoSyncTargetPath = %q, want %q", got, want)
		}

		got, err = autoSyncTargetPath(proj, true)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(proj, ".claude", "settings.json"); got != want {
			t.Errorf("autoSyncTargetPath(shared) = %q, want %q", got, want)
		}
	})

	t.Run("missing project directory errors", func(t *testing.T) {
		if _, err := autoSyncTargetPath(filepath.Join(tmpHome, "does-not-exist"), false); err == nil {
			t.Error("expected error for missing project directory")
		}
	})

	t.Run("project path expands ~", func(t *testing.T) {
		proj := filepath.Join(tmpHome, "myproj")
		if err := os.MkdirAll(proj, 0700); err != nil {
			t.Fatal(err)
		}
		got, err := autoSyncTargetPath("~/myproj", false)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(proj, ".claude", "settings.local.json"); got != want {
			t.Errorf("autoSyncTargetPath = %q, want %q", got, want)
		}
	})
}

func TestAutoSyncHookConfigIsProfileAware(t *testing.T) {
	t.Run("default profile keeps plain commands", func(t *testing.T) {
		if err := config.SetActiveProfile(""); err != nil {
			t.Fatal(err)
		}
		cfg := autoSyncHookConfig()
		if cfg.PullCommand != "claude-sync pull -q" || cfg.PushCommand != "claude-sync push -q" {
			t.Errorf("unexpected default hook commands: %q / %q", cfg.PullCommand, cfg.PushCommand)
		}
	})

	t.Run("named profile embeds --profile flag", func(t *testing.T) {
		if err := config.SetActiveProfile("personal"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = config.SetActiveProfile("") })

		cfg := autoSyncHookConfig()
		if cfg.PullCommand != "claude-sync --profile personal pull -q" {
			t.Errorf("PullCommand = %q", cfg.PullCommand)
		}
		if cfg.PushCommand != "claude-sync --profile personal push -q" {
			t.Errorf("PushCommand = %q", cfg.PushCommand)
		}
	})
}
