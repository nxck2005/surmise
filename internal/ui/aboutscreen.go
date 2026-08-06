package ui

import (
	"slices"
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

	// width and height are the terminal's, pushed down by the root. Zero means
	// unmeasured, which counts as unbounded.
	width, height int
}

func (a *aboutScreen) resize(w, h int) { a.width, a.height = w, h }

// aboutRow is one label/value line. optional marks a row the screen may drop
// when the terminal is too short for all of them — the credits, which are a
// courtesy rather than something a bug report needs.
type aboutRow struct {
	label, value string
	optional     bool
}

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
		{label: "version", value: info.Version},
	}
	if c := info.Commit(); c != "" {
		rows = append(rows, aboutRow{label: "commit", value: c})
	}
	if info.Time != "" {
		rows = append(rows, aboutRow{label: "built", value: info.Time})
	}
	rows = append(rows, aboutRow{label: "go", value: info.Toolchain()})

	if dataDir != "" {
		rows = append(rows,
			aboutRow{label: "data", value: dataDir},
			aboutRow{label: "themes", value: theme.Dir(dataDir)},
		)
	}

	rows = append(rows,
		aboutRow{label: "repo", value: repoURL},
		aboutRow{label: "license", value: license},
	)
	for _, c := range words.Credits {
		rows = append(rows, aboutRow{label: c.What, value: c.Source, optional: true})
	}
	return rows
}

// affordableRows drops optional rows, last first, until the rest fit the
// budget. A budget of zero — an unmeasured terminal — keeps everything, which
// is what the headless tests see. The required rows are never dropped: if they
// alone do not fit, the screen scrolls instead (see offset).
func affordableRows(rows []aboutRow, budget int) []aboutRow {
	if budget <= 0 || len(rows) <= budget {
		return rows
	}
	kept := make([]aboutRow, 0, len(rows))
	drop := len(rows) - budget
	// Walk backwards so the last optional rows are the first to go.
	for i := len(rows) - 1; i >= 0; i-- {
		if drop > 0 && rows[i].optional {
			drop--
			continue
		}
		kept = append(kept, rows[i])
	}
	slices.Reverse(kept)
	return kept
}

func (a *aboutScreen) view(h *hitMap) string {
	// A screen opened by any path other than applyChoice has no rows yet;
	// building them is cheap enough to just do it.
	rows := a.rows
	if len(rows) == 0 {
		rows = aboutRows("")
	}
	rows = affordableRows(rows, bodyBudget(a.height))

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
