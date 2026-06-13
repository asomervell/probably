package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Card
type CardProps struct {
	Class string
}

func Card(props CardProps, children ...g.Node) g.Node {
	classes := Cn(
		"rounded-xl border border-border bg-card text-card-foreground",
		props.Class,
	)

	return h.Div(
		h.Class(classes),
		g.Group(children),
	)
}

func CardHeader(children ...g.Node) g.Node {
	return h.Div(
		h.Class("flex flex-col space-y-1.5 p-6"),
		g.Group(children),
	)
}

func CardHeaderActions(title, description string, actions ...g.Node) g.Node {
	return h.Div(
		h.Class("flex items-start justify-between p-6"),
		h.Div(
			h.Class("flex flex-col space-y-1.5"),
			CardTitle(g.Text(title)),
			g.If(description != "", CardDescription(g.Text(description))),
		),
		g.If(len(actions) > 0,
			h.Div(h.Class("flex items-center gap-2"), g.Group(actions)),
		),
	)
}

func CardTitle(children ...g.Node) g.Node {
	return h.H3(
		h.Class("font-semibold leading-none tracking-tight text-card-foreground"),
		g.Group(children),
	)
}

func CardDescription(children ...g.Node) g.Node {
	return h.P(
		h.Class("text-sm text-muted-foreground"),
		g.Group(children),
	)
}

func CardContent(children ...g.Node) g.Node {
	return h.Div(
		h.Class("p-6 pt-0"),
		g.Group(children),
	)
}

func CardContentFull(children ...g.Node) g.Node {
	return h.Div(
		h.Class("p-6"),
		g.Group(children),
	)
}

func CardFooter(children ...g.Node) g.Node {
	return h.Div(
		h.Class("flex items-center p-6 pt-0"),
		g.Group(children),
	)
}

func CardFooterFull(children ...g.Node) g.Node {
	return h.Div(
		h.Class("flex items-center p-6 border-t border-border"),
		g.Group(children),
	)
}

// Card Variants
func CardInteractive(props CardProps, children ...g.Node) g.Node {
	classes := Cn(
		"rounded-xl border border-border bg-card text-card-foreground shadow transition-colors hover:bg-muted/50 cursor-pointer",
		props.Class,
	)

	return h.Div(
		h.Class(classes),
		g.Group(children),
	)
}

func CardLink(props CardProps, href string, children ...g.Node) g.Node {
	classes := Cn(
		"block rounded-xl border border-border bg-card text-card-foreground shadow transition-colors hover:bg-muted/50 hover:border-border cursor-pointer",
		props.Class,
	)

	return h.A(
		h.Href(href),
		h.Class(classes),
		g.Group(children),
	)
}

func CardBordered(props CardProps, children ...g.Node) g.Node {
	classes := Cn(
		"rounded-xl border-2 border-border bg-card text-card-foreground shadow",
		props.Class,
	)

	return h.Div(
		h.Class(classes),
		g.Group(children),
	)
}

func CardFlat(props CardProps, children ...g.Node) g.Node {
	classes := Cn(
		"rounded-xl border border-border bg-card text-card-foreground",
		props.Class,
	)

	return h.Div(
		h.Class(classes),
		g.Group(children),
	)
}

func CardGhost(props CardProps, children ...g.Node) g.Node {
	classes := Cn(
		"rounded-xl text-card-foreground",
		props.Class,
	)

	return h.Div(
		h.Class(classes),
		g.Group(children),
	)
}

// Card with Image
func CardImage(src, alt string) g.Node {
	return h.Img(
		h.Src(src),
		h.Alt(alt),
		h.Class("rounded-t-xl w-full object-cover"),
	)
}

func CardImageAspect(src, alt string, aspect string) g.Node {
	aspectClass := "aspect-video"
	switch aspect {
	case "square":
		aspectClass = "aspect-square"
	case "video":
		aspectClass = "aspect-video"
	case "portrait":
		aspectClass = "aspect-[3/4]"
	}

	return h.Div(
		h.Class(Cn(aspectClass, "overflow-hidden rounded-t-xl")),
		h.Img(
			h.Src(src),
			h.Alt(alt),
			h.Class("w-full h-full object-cover"),
		),
	)
}

// Stat Card
type StatCardProps struct {
	Label       string
	Value       string
	Description string
	Trend       string
	TrendUp     bool
	Icon        g.Node
	Class       string
}

func StatCard(props StatCardProps) g.Node {
	return Card(CardProps{Class: props.Class},
		CardContentFull(
			h.Div(
				h.Class("flex items-center justify-between"),
				h.Div(
					h.P(h.Class("text-sm font-medium text-muted-foreground"), g.Text(props.Label)),
					h.P(h.Class("text-2xl font-bold text-card-foreground mt-1"), g.Text(props.Value)),
					g.If(props.Description != "",
						h.P(h.Class("text-sm text-muted-foreground mt-1"), g.Text(props.Description)),
					),
					g.If(props.Trend != "",
						h.Div(
							h.Class("flex items-center gap-1 mt-2"),
							g.If(props.TrendUp,
								h.Span(h.Class("text-chart-2 text-sm"), g.Text("↑ "+props.Trend)),
							),
							g.If(!props.TrendUp,
								h.Span(h.Class("text-destructive text-sm"), g.Text("↓ "+props.Trend)),
							),
						),
					),
				),
				g.If(props.Icon != nil,
					h.Div(h.Class("text-muted-foreground"), props.Icon),
				),
			),
		),
	)
}

// Feature Card
type FeatureCardProps struct {
	Icon        g.Node
	Title       string
	Description string
	Class       string
}

func FeatureCard(props FeatureCardProps) g.Node {
	return Card(CardProps{Class: props.Class},
		CardContentFull(
			h.Div(
				h.Class("flex flex-col items-center text-center"),
				g.If(props.Icon != nil,
					h.Div(h.Class("mb-4 text-primary"), props.Icon),
				),
				h.H3(h.Class("font-semibold text-card-foreground mb-2"), g.Text(props.Title)),
				h.P(h.Class("text-sm text-muted-foreground"), g.Text(props.Description)),
			),
		),
	)
}
