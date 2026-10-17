// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/seanb4t/engram/internal/config"
)

// consolePrefs is the console's preferences file: the choices a person
// makes inside the console. It holds no credential and no record data, and
// nothing in it is sent to the server.
type consolePrefs struct {
	Theme string `json:"theme,omitempty"`
	Mouse *bool  `json:"mouse,omitempty"`
}

// consoleSettings is the theme and mouse choice the console starts with,
// plus where to save a change.
type consoleSettings struct {
	theme string
	// themeLocked is true when --theme or ENGRAM_TUI_THEME chose the
	// theme, so a theme saved from the picker only applies once neither
	// is set.
	themeLocked bool
	mouse       bool
	noColor     bool
	prefsPath   string
	prefs       consolePrefs
	// note explains a preferences file the console had to ignore.
	note string
}

// consolePrefsPath is console.json beside the login cache.
func consolePrefsPath() (string, error) {
	p, err := credentialsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "console.json"), nil
}

// loadConsolePrefs reads the preferences file. A missing file is not an
// error. An unreadable or invalid one yields no preferences and a note, so
// the console still opens on its defaults.
func loadConsolePrefs(path string) (consolePrefs, string) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return consolePrefs{}, ""
	}
	if err != nil {
		return consolePrefs{}, fmt.Sprintf("ignored the console preferences: %v", err)
	}
	var p consolePrefs
	if err := json.Unmarshal(b, &p); err != nil || (p.Theme != "" && !slices.Contains(config.TUIThemes, p.Theme)) {
		return consolePrefs{}, "ignored invalid console preferences in " + path
	}
	return p, ""
}

// saveConsolePrefs writes the preferences file through a temporary file
// and a rename, as the login cache does, so a crash never leaves it half
// written. os.CreateTemp creates the file with mode 0600.
func saveConsolePrefs(path string, p consolePrefs) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the console preferences: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".console-*.json")
	if err != nil {
		return fmt.Errorf("write the console preferences: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write the console preferences: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write the console preferences: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("write the console preferences: %w", err)
	}
	return nil
}

// loadConsoleSettings resolves each setting as flag, then environment
// (both already merged into c by config.Load), then saved preference, then
// default. c has passed config.ValidateConsole.
func loadConsoleSettings(c config.ClientConfig, getenv func(string) string) consoleSettings {
	s := consoleSettings{theme: config.TUIThemes[0], noColor: getenv("NO_COLOR") != ""}
	if path, err := consolePrefsPath(); err == nil {
		s.prefsPath = path
		s.prefs, s.note = loadConsolePrefs(path)
	} else {
		s.note = fmt.Sprintf("console preferences will not be saved: %v", err)
	}
	switch {
	case c.TUITheme != "":
		s.theme, s.themeLocked = c.TUITheme, true
	case s.prefs.Theme != "":
		s.theme = s.prefs.Theme
	}
	switch {
	case c.TUIMouse != "":
		s.mouse, _ = strconv.ParseBool(c.TUIMouse)
	case s.prefs.Mouse != nil:
		s.mouse = *s.prefs.Mouse
	}
	return s
}
