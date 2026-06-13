package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// SearchInput
func SearchInput(name, placeholder, value string, icon g.Node, attrs ...g.Node) g.Node {
	return h.Div(
		h.Class("relative"),
		Input(InputProps{
			Type:        "search",
			Name:        name,
			Placeholder: placeholder,
			Value:       value,
			Class:       "pl-10",
		}, attrs...),
		h.Div(
			h.Class("absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none"),
			icon,
		),
	)
}
