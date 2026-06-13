package shadcn

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// FilterTabs
// FilterTab represents a single filter tab
type FilterTab struct {
	Label string
	Value string
	Count int
	Href  string // Optional: if provided, use this instead of building from Value
}

func FilterTabs(tabs []FilterTab, activeValue string) g.Node {
	tabNodes := make([]g.Node, len(tabs))
	for _, tab := range tabs {
		isActive := tab.Value == activeValue
		href := tab.Href
		if href == "" {
			// Build href from value - caller should provide Href for complex cases
			href = "?filter=" + tab.Value
		}

		activeClass := ""
		if isActive {
			activeClass = "bg-accent text-foreground"
		} else {
			activeClass = "text-muted-foreground hover:text-foreground"
		}

		tabNodes = append(tabNodes,
			h.A(
				h.Href(href),
				h.Class(Cn("px-3 py-1.5 rounded-md text-sm font-medium transition-colors", activeClass)),
				g.Text(tab.Label),
				g.If(tab.Count > 0,
					h.Span(h.Class("ml-2 text-xs opacity-70"), g.Text(fmt.Sprintf("(%d)", tab.Count))),
				),
			),
		)
	}

	return h.Div(
		h.Class("flex items-center gap-1 bg-secondary rounded-lg p-1"),
		g.Group(tabNodes),
	)
}
