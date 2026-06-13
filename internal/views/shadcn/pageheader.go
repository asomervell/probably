package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// PageHeader
func PageHeader(title string, subtitle string, actions ...g.Node) g.Node {
	return h.Div(
		h.Class("flex items-start justify-between gap-4 mb-8"),
		h.Div(
			h.Class("min-w-0 flex-1"),
			h.H1(h.Class("text-2xl font-bold text-foreground"), g.Text(title)),
			g.If(subtitle != "", h.P(h.Class("text-muted-foreground mt-1 hidden sm:block"), g.Text(subtitle))),
		),
		g.If(len(actions) > 0, h.Div(h.Class("flex items-center gap-3 flex-shrink-0"), g.Group(actions))),
	)
}
