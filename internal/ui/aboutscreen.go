package ui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/build"
	"github.com/nxck2005/surmise/internal/theme"
	"github.com/nxck2005/surmise/internal/words"
)

// repoURL is where the project lives, shown to the player on the about screen.
// It comes from internal/brand so that a rename does not have to find it here.
const repoURL = brand.Repo

// license is the shipped licence, in the shortest honest form. LICENSE holds
// the full text and the copyright line. It is scoped deliberately: the MIT grant covers this project's own code and
// data, while the linked modules, the adapted themes and the word lists stay
// under their own terms. THIRD_PARTY_NOTICES.md, which ships in every release
// archive, is where those live.
const license = "MIT — deps under their own terms"

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
// courtesy rather than something a bug report needs. path marks a value that is
// cut from the left when it does not fit, because the end of a path is the part
// that tells two of them apart.
type aboutRow struct {
	label, value string
	optional     bool
	path         bool
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
			aboutRow{label: "data", value: dataDir, path: true},
			aboutRow{label: "themes", value: theme.Dir(dataDir), path: true},
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
	// What is left of the panel for a value. Nothing else on the screen is that
	// wide, so a value the terminal cannot hold would widen the panel past its
	// edge rather than wrap; zero is an unmeasured terminal, which is unbounded.
	room := 0
	if w := bodyWidth(a.width); w > 0 {
		room = max(w-(width+2), 1)
	}

	lines := make([]string, len(rows))
	for i, r := range rows {
		// A value can be a path from -data, which is the shell's to choose and
		// so reaches the frame unvalidated; labels are all literals.
		lines[i] = label.Render(st.muted.Render(r.label)) + st.text.Render(fitValue(safeText(r.value), room, r.path))
	}

	return block(strings.Join(lines, "\n"))
}

// fitValue cuts a value to room cells, marking the cut with an ellipsis: from
// the left for a path, from the right for anything else. A room of zero leaves
// it whole.
func fitValue(v string, room int, path bool) string {
	over := lipgloss.Width(v) - room
	if room <= 0 || over <= 0 {
		return v
	}
	if path {
		return ansi.TruncateLeft(v, over+1, "…")
	}
	return ansi.Truncate(v, room, "…")
}

func (a *aboutScreen) help(h *hitMap) string {
	return renderHelp(h, helpItem{keys: "esc", label: "menu", act: action{kind: actBack}})
}
