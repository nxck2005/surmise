package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/wortle/internal/build"
	"github.com/nxck2005/wortle/internal/theme"
	"github.com/nxck2005/wortle/internal/words"
)

// repoURL is where the project lives. It is here rather than in a package doc
// comment because this is the only place that shows it to a player.
const repoURL = "github.com/nxck2005/wortle"

// license is the shipped licence, in the shortest honest form. Matches LICENSE.
const license = "MIT © 2026 Nxck"

// aboutScreen is the "what am I running" screen: version, build, where the
// files are, and who the words belong to. It holds no cursor — the root handles
// its only two keys — so it is the lightest screen in the app.
type aboutScreen struct {
	rows []aboutRow
}

// aboutRow is one label/value line.
type aboutRow struct{ label, value string }

// reload rebuilds the content. dataDir may be empty, meaning the UI was never
// told where its files live (a zero Options, as the tests pass).
func (a *aboutScreen) reload(dataDir string) {
	a.rows = aboutRows(dataDir)
}

// aboutRows is the whole content of the about screen, in display order. Adding,
// removing or reordering an entry is an edit here and nowhere else — the view
// below only knows how to draw a label and a value.
func aboutRows(dataDir string) []aboutRow {
	info := build.Get()

	rows := []aboutRow{
		{"version", info.Version},
	}
	if c := info.Commit(); c != "" {
		rows = append(rows, aboutRow{"commit", c})
	}
	if info.Time != "" {
		rows = append(rows, aboutRow{"built", info.Time})
	}
	rows = append(rows, aboutRow{"go", info.Toolchain()})

	if dataDir != "" {
		rows = append(rows,
			aboutRow{"data", dataDir},
			aboutRow{"themes", theme.Dir(dataDir)},
		)
	}

	rows = append(rows,
		aboutRow{"repo", repoURL},
		aboutRow{"license", license},
	)
	for _, c := range words.Credits {
		rows = append(rows, aboutRow{c.What, c.Source})
	}
	return rows
}

func (a *aboutScreen) view(h *hitMap) string {
	// A screen opened by any path other than applyChoice has no rows yet;
	// building them is cheap enough to just do it.
	rows := a.rows
	if len(rows) == 0 {
		rows = aboutRows("")
	}

	width := 0
	for _, r := range rows {
		if w := lipgloss.Width(r.label); w > width {
			width = w
		}
	}
	// The gutter keeps the two columns apart once the labels are padded.
	label := lipgloss.NewStyle().Width(width + 2)

	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = label.Render(st.muted.Render(r.label)) + st.text.Render(r.value)
	}

	return titled("about", strings.Join(lines, "\n"))
}

func (a *aboutScreen) help(h *hitMap) string {
	return renderHelp(h, helpItem{keys: "esc", label: "menu", act: action{kind: actBack}})
}
