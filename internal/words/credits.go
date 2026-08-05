package words

// Credit names where one of the shipped lists came from.
type Credit struct {
	// What the list is, in the player's terms.
	What string
	// Source is a short, human-readable provenance — not a URL, since the
	// about screen has a line, not a page.
	Source string
}

// Credits summarises data/SOURCES.md for display, so the UI never hardcodes
// provenance. Keep it in step with that file when the lists are regenerated
// from somewhere new; SOURCES.md stays the full account.
var Credits = []Credit{
	{What: "guesses", Source: "ENABLE1 word list (public domain)"},
	{What: "answers", Source: "google-10000-english frequency list"},
}
