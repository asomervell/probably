package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// StatusBadge
func StatusBadge(isPositive bool, text string) g.Node {
	var badgeClass string
	if isPositive {
		badgeClass = "bg-chart-2/20 text-chart-2"
	} else {
		badgeClass = "bg-destructive/20 text-destructive"
	}

	return h.Span(
		h.Class(Cn("inline-flex items-center px-2 py-0.5 rounded text-xs font-medium", badgeClass)),
		g.Text(text),
	)
}
