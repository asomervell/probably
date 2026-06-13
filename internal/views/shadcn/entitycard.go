package shadcn

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// EntityCard
type EntityCardProps struct {
	ID         string
	Name       string
	Subtype    string
	LogoURL    string
	GetLogoURL func(string) string
}

func EntityCard(props EntityCardProps) g.Node {
	logoURL := ""
	if props.LogoURL != "" && props.GetLogoURL != nil {
		logoURL = props.GetLogoURL(props.LogoURL)
	}

	subtypeDisplay := props.Subtype
	if subtypeDisplay == "" {
		subtypeDisplay = "Entity"
	}

	initials := "?"
	if len(props.Name) > 0 {
		initials = string(props.Name[0])
	}

	return h.A(
		h.Href(fmt.Sprintf("/entities/%s", props.ID)),
		h.Class("block rounded-xl bg-card/50 p-4 hover:bg-card transition-colors border border-border"),
		h.Div(
			h.Class("flex items-center gap-4"),
			// Logo
			h.Div(
				h.Class("w-12 h-12 rounded-lg overflow-hidden bg-secondary flex items-center justify-center flex-shrink-0"),
				g.If(logoURL != "",
					h.Img(
						h.Src(logoURL),
						h.Alt(props.Name),
						h.Class("w-full h-full object-contain"),
					),
				),
				g.If(logoURL == "",
					h.Span(h.Class("text-lg font-semibold text-muted-foreground"), g.Text(initials)),
				),
			),
			// Info
			h.Div(
				h.Class("flex-1 min-w-0"),
				h.P(h.Class("font-medium text-foreground truncate"), g.Text(props.Name)),
				h.P(h.Class("text-sm text-muted-foreground"), g.Text(subtypeDisplay)),
			),
		),
	)
}
