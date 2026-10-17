// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"image/color"

	"charm.land/bubbles/v2/textinput"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
)

// themeRoles are the colours the console draws with, by purpose. Every
// role is set in every theme variant.
type themeRoles struct {
	fg, muted, faint             color.Color
	accent, onAccent             color.Color
	divider, dividerFocus        color.Color
	selection, selectionBlurred  color.Color
	warning, errorFg, success    color.Color
	convention, gotcha, decision color.Color
	preference, discovery, rule  color.Color
	// md are the same roles as Glamour colour strings (a hex value, or an
	// ANSI index for the terminal theme), plus the base markdown style.
	mdText, mdAccent, mdMuted string
	mdBase                    ansi.StyleConfig
}

// all returns every role, for the test that every theme sets each one.
func (r themeRoles) all() []color.Color {
	return []color.Color{
		r.fg, r.muted, r.faint, r.accent, r.onAccent, r.divider, r.dividerFocus,
		r.selection, r.selectionBlurred, r.warning, r.errorFg, r.success,
		r.convention, r.gotcha, r.decision, r.preference, r.discovery, r.rule,
	}
}

func (r themeRoles) category(name string) color.Color {
	switch name {
	case "convention":
		return r.convention
	case "gotcha":
		return r.gotcha
	case "decision":
		return r.decision
	case "preference":
		return r.preference
	case "discovery":
		return r.discovery
	case "rule":
		return r.rule
	}
	return r.muted
}

// consoleTheme is a named theme with a dark and a light variant; the
// console picks one from the terminal's background.
type consoleTheme struct {
	name        string
	dark, light themeRoles
}

func hexColor(s string) color.Color { return lipgloss.Color(s) }

// engramTheme carries the web console's tokens (ui/src/app.css): `.dark`
// for the dark variant and `:root` for the light one.
var engramTheme = consoleTheme{
	name: "engram",
	dark: themeRoles{
		fg: hexColor("#e6edf3"), muted: hexColor("#8b949e"), faint: hexColor("#6e7681"),
		accent: hexColor("#8B7BE8"), onAccent: hexColor("#0d1117"),
		divider: hexColor("#30363d"), dividerFocus: hexColor("#8B7BE8"),
		selection: hexColor("#252d3d"), selectionBlurred: hexColor("#6e7681"),
		warning: hexColor("#d29922"), errorFg: hexColor("#ffa657"), success: hexColor("#3fb950"),
		convention: hexColor("#a5d6ff"), gotcha: hexColor("#ffa657"), decision: hexColor("#E879F9"),
		preference: hexColor("#79c0ff"), discovery: hexColor("#2dd4bf"), rule: hexColor("#f0b72f"),
		mdText: "#e6edf3", mdAccent: "#8B7BE8", mdMuted: "#8b949e", mdBase: styles.DarkStyleConfig,
	},
	light: themeRoles{
		fg: hexColor("#1f2328"), muted: hexColor("#59636e"), faint: hexColor("#818b98"),
		accent: hexColor("#6E56CF"), onAccent: hexColor("#ffffff"),
		divider: hexColor("#d0d7de"), dividerFocus: hexColor("#6E56CF"),
		selection: hexColor("#e7e3fb"), selectionBlurred: hexColor("#818b98"),
		warning: hexColor("#9a6700"), errorFg: hexColor("#bc4c00"), success: hexColor("#1a7f37"),
		convention: hexColor("#0969da"), gotcha: hexColor("#bc4c00"), decision: hexColor("#C026D3"),
		preference: hexColor("#0550ae"), discovery: hexColor("#0d9488"), rule: hexColor("#9a6700"),
		mdText: "#1f2328", mdAccent: "#6E56CF", mdMuted: "#59636e", mdBase: styles.LightStyleConfig,
	},
}

// terminalRoles uses ANSI indices only, so the person's own terminal
// palette shows. The foreground is the terminal's default.
func terminalRoles(base ansi.StyleConfig) themeRoles {
	return themeRoles{
		fg: lipgloss.NoColor{}, muted: hexColor("8"), faint: hexColor("8"),
		accent: hexColor("5"), onAccent: hexColor("0"),
		divider: hexColor("8"), dividerFocus: hexColor("5"),
		selection: lipgloss.NoColor{}, selectionBlurred: hexColor("8"),
		warning: hexColor("3"), errorFg: hexColor("1"), success: hexColor("2"),
		convention: hexColor("4"), gotcha: hexColor("9"), decision: hexColor("5"),
		preference: hexColor("6"), discovery: hexColor("2"), rule: hexColor("3"),
		mdText: "", mdAccent: "5", mdMuted: "8", mdBase: base,
	}
}

var terminalTheme = consoleTheme{
	name:  "terminal",
	dark:  terminalRoles(styles.DarkStyleConfig),
	light: terminalRoles(styles.LightStyleConfig),
}

// catppuccinTheme is Catppuccin Mocha (dark) and Latte (light).
var catppuccinTheme = consoleTheme{
	name: "catppuccin",
	dark: themeRoles{
		fg: hexColor("#cdd6f4"), muted: hexColor("#a6adc8"), faint: hexColor("#7f849c"),
		accent: hexColor("#cba6f7"), onAccent: hexColor("#1e1e2e"),
		divider: hexColor("#45475a"), dividerFocus: hexColor("#cba6f7"),
		selection: hexColor("#313244"), selectionBlurred: hexColor("#7f849c"),
		warning: hexColor("#f9e2af"), errorFg: hexColor("#f38ba8"), success: hexColor("#a6e3a1"),
		convention: hexColor("#89b4fa"), gotcha: hexColor("#fab387"), decision: hexColor("#f5c2e7"),
		preference: hexColor("#74c7ec"), discovery: hexColor("#94e2d5"), rule: hexColor("#f9e2af"),
		mdText: "#cdd6f4", mdAccent: "#cba6f7", mdMuted: "#a6adc8", mdBase: styles.DarkStyleConfig,
	},
	light: themeRoles{
		fg: hexColor("#4c4f69"), muted: hexColor("#6c6f85"), faint: hexColor("#8c8fa1"),
		accent: hexColor("#8839ef"), onAccent: hexColor("#eff1f5"),
		divider: hexColor("#bcc0cc"), dividerFocus: hexColor("#8839ef"),
		selection: hexColor("#ccd0da"), selectionBlurred: hexColor("#8c8fa1"),
		warning: hexColor("#df8e1d"), errorFg: hexColor("#d20f39"), success: hexColor("#40a02b"),
		convention: hexColor("#1e66f5"), gotcha: hexColor("#fe640b"), decision: hexColor("#ea76cb"),
		preference: hexColor("#209fb5"), discovery: hexColor("#179299"), rule: hexColor("#df8e1d"),
		mdText: "#4c4f69", mdAccent: "#8839ef", mdMuted: "#6c6f85", mdBase: styles.LightStyleConfig,
	},
}

// tokyoNightTheme is Tokyo Night's Night (dark) and Day (light) styles.
var tokyoNightTheme = consoleTheme{
	name: "tokyo-night",
	dark: themeRoles{
		fg: hexColor("#c0caf5"), muted: hexColor("#737aa2"), faint: hexColor("#565f89"),
		accent: hexColor("#bb9af7"), onAccent: hexColor("#1a1b26"),
		divider: hexColor("#3b4261"), dividerFocus: hexColor("#bb9af7"),
		selection: hexColor("#292e42"), selectionBlurred: hexColor("#565f89"),
		warning: hexColor("#e0af68"), errorFg: hexColor("#f7768e"), success: hexColor("#9ece6a"),
		convention: hexColor("#7aa2f7"), gotcha: hexColor("#ff9e64"), decision: hexColor("#bb9af7"),
		preference: hexColor("#7dcfff"), discovery: hexColor("#1abc9c"), rule: hexColor("#e0af68"),
		mdText: "#c0caf5", mdAccent: "#bb9af7", mdMuted: "#737aa2", mdBase: styles.DarkStyleConfig,
	},
	light: themeRoles{
		fg: hexColor("#3760bf"), muted: hexColor("#6172b0"), faint: hexColor("#848cb5"),
		accent: hexColor("#9854f1"), onAccent: hexColor("#e1e2e7"),
		divider: hexColor("#a8aecb"), dividerFocus: hexColor("#9854f1"),
		selection: hexColor("#c4c8da"), selectionBlurred: hexColor("#848cb5"),
		warning: hexColor("#8c6c3e"), errorFg: hexColor("#f52a65"), success: hexColor("#587539"),
		convention: hexColor("#2e7de9"), gotcha: hexColor("#b15c00"), decision: hexColor("#9854f1"),
		preference: hexColor("#007197"), discovery: hexColor("#118c74"), rule: hexColor("#8c6c3e"),
		mdText: "#3760bf", mdAccent: "#9854f1", mdMuted: "#6172b0", mdBase: styles.LightStyleConfig,
	},
}

// consoleThemes are listed in the picker's order, which matches
// config.TUIThemes.
var consoleThemes = []consoleTheme{engramTheme, terminalTheme, catppuccinTheme, tokyoNightTheme}

func themeNamed(name string) consoleTheme {
	for _, t := range consoleThemes {
		if t.name == name {
			return t
		}
	}
	return engramTheme
}

// noColorRoles draws nothing in colour, for NO_COLOR.
func noColorRoles() themeRoles {
	none := lipgloss.NoColor{}
	return themeRoles{
		fg: none, muted: none, faint: none, accent: none, onAccent: none,
		divider: none, dividerFocus: none, selection: none, selectionBlurred: none,
		warning: none, errorFg: none, success: none,
		convention: none, gotcha: none, decision: none, preference: none, discovery: none, rule: none,
		mdBase: styles.ASCIIStyleConfig,
	}
}

// consoleStyles are the lipgloss styles built from one theme variant.
type consoleStyles struct {
	roles                                   themeRoles
	text, muted, faint, bold, accent        lipgloss.Style
	tabActive, tabInactive                  lipgloss.Style
	bar, barBlurred, selected               lipgloss.Style
	divider, dividerFocus                   lipgloss.Style
	warning, errorFg, success, key, overlay lipgloss.Style
}

func newConsoleStyles(r themeRoles) consoleStyles {
	s := lipgloss.NewStyle()
	return consoleStyles{
		roles:        r,
		text:         s.Foreground(r.fg),
		muted:        s.Foreground(r.muted),
		faint:        s.Foreground(r.faint),
		bold:         s.Foreground(r.fg).Bold(true),
		accent:       s.Foreground(r.accent),
		tabActive:    s.Foreground(r.onAccent).Background(r.accent).Bold(true).Padding(0, 1),
		tabInactive:  s.Foreground(r.muted).Padding(0, 1),
		bar:          s.Foreground(r.accent),
		barBlurred:   s.Foreground(r.selectionBlurred),
		selected:     s.Background(r.selection),
		divider:      s.Foreground(r.divider),
		dividerFocus: s.Foreground(r.dividerFocus),
		warning:      s.Foreground(r.warning),
		errorFg:      s.Foreground(r.errorFg),
		success:      s.Foreground(r.success),
		key:          s.Foreground(r.accent).Bold(true),
		overlay: s.Border(lipgloss.RoundedBorder()).BorderForeground(r.accent).
			Padding(0, 1),
	}
}

// inputStyles colours a text input from the theme, keeping its cursor
// settings.
func (s consoleStyles) inputStyles(in textinput.Styles) textinput.Styles {
	in.Focused = textinput.StyleState{Prompt: s.accent, Text: s.text, Placeholder: s.faint, Suggestion: s.faint}
	in.Blurred = in.Focused
	in.Blurred.Prompt = s.muted
	return in
}

func (s consoleStyles) category(name string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.roles.category(name))
}

// markdownStyle is the Glamour style for a theme variant: its base style
// with no outer margin and the theme's text and accent colours.
func markdownStyle(r themeRoles) ansi.StyleConfig {
	st := r.mdBase
	zero := uint(0)
	str := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	st.Document = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: str(r.mdText)}, Margin: &zero}
	if r.mdAccent == "" {
		return st
	}
	bold := true
	heading := func(b ansi.StyleBlock, prefix string) ansi.StyleBlock {
		b.Color, b.BackgroundColor, b.Bold = str(r.mdAccent), nil, &bold
		b.Prefix, b.Suffix = prefix, ""
		return b
	}
	st.H1 = heading(st.H1, "# ")
	st.H2 = heading(st.H2, "## ")
	st.H3 = heading(st.H3, "### ")
	st.Link.Color = str(r.mdAccent)
	st.LinkText.Color = str(r.mdAccent)
	st.BlockQuote.Color = str(r.mdMuted)
	return st
}
