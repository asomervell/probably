package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Empty State
type EmptyProps struct {
	Icon        g.Node
	Title       string
	Description string
	Class       string
}

func Empty(props EmptyProps, actions ...g.Node) g.Node {
	return h.Div(
		h.Class(Cn("flex flex-col items-center justify-center py-12 px-4 text-center", props.Class)),
		g.If(props.Icon != nil,
			h.Div(
				h.Class("flex items-center justify-center w-12 h-12 rounded-full bg-muted text-muted-foreground mb-4"),
				props.Icon,
			),
		),
		h.H3(
			h.Class("text-lg font-semibold text-foreground mb-1"),
			g.Text(props.Title),
		),
		g.If(props.Description != "",
			h.P(
				h.Class("text-sm text-muted-foreground max-w-sm mb-4"),
				g.Text(props.Description),
			),
		),
		g.If(len(actions) > 0,
			h.Div(
				h.Class("flex items-center gap-2"),
				g.Group(actions),
			),
		),
	)
}

// Common Empty States
func EmptyNoData(title, description string, action g.Node) g.Node {
	return Empty(EmptyProps{
		Icon:        IconInbox(),
		Title:       title,
		Description: description,
	}, action)
}

func EmptyNoResults(searchTerm string) g.Node {
	description := "Try adjusting your search or filter to find what you're looking for."
	if searchTerm != "" {
		description = "No results found for \"" + searchTerm + "\". Try a different search term."
	}
	return Empty(EmptyProps{
		Icon:        IconSearch(),
		Title:       "No results found",
		Description: description,
	})
}

// Empty State Icons
func IconInbox() g.Node {
	return IconLg(`<path d="M22 12h-6l-2 3h-4l-2-3H2"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>`)
}

func IconSparkles() g.Node {
	return IconLg(`<path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275z"/><path d="M5 3v4"/><path d="M19 17v4"/><path d="M3 5h4"/><path d="M17 19h4"/>`)
}

func IconUsers() g.Node {
	return IconLg(`<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>`)
}

