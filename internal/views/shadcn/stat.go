package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Stat
type StatProps struct {
	Label    string
	Value    string
	Trend    string
	Positive bool
}

func Stat(props StatProps) g.Node {
	return Card(CardProps{},
		CardContentFull(
			h.P(h.Class("text-sm text-muted-foreground"), g.Text(props.Label)),
			h.P(h.Class("text-2xl font-bold text-card-foreground mt-1"), g.Text(props.Value)),
			g.If(props.Trend != "",
				h.P(
					h.Class("text-sm mt-1"),
					g.If(props.Positive, h.Span(h.Class("text-chart-2"), g.Text(props.Trend))),
					g.If(!props.Positive, h.Span(h.Class("text-destructive"), g.Text(props.Trend))),
				),
			),
		),
	)
}
