package layouts

import (
	"crypto/sha256"
	"fmt"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// MarketingLayout renders the marketing site layout
func MarketingLayout(title string, userEmail string, posthogDistinctID string, content ...g.Node) g.Node {
	return BaseWithMetaURLAndToasts(title, "Simple personal finance tracking", "", nil, posthogDistinctID,
			h.Div(
				h.Class("min-h-screen flex flex-col bg-background"),
			
			// Navigation
			MarketingNavbar(userEmail),
			
			// Main Content
			h.Main(
				h.Class("flex-1"),
				g.Group(content),
			),
			
			// Footer
			MarketingFooter(),
		),
	)
}

// MarketingNavbar renders the top navigation bar
func MarketingNavbar(userEmail string) g.Node {
	isLoggedIn := userEmail != ""
	
	return h.Nav(
		h.Class("fixed top-0 w-full z-50 border-b border-border bg-background/80 backdrop-blur-md"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("flex items-center justify-between h-16"),
				
				// Logo + Navigation (left aligned together)
				h.Div(
					h.Class("flex items-center gap-8"),
					// Logo
					h.A(
						h.Href("/"),
						h.Class("flex items-center gap-2"),
						h.Img(
							h.Src("/static/logo-pbw.png"),
							h.Alt("Probably"),
							h.Class("w-8 h-8 dark:invert"),
						),
						h.Span(
							h.Class("text-xl font-bold tracking-tight text-foreground"),
							g.Text("probably"),
						),
					),
					// Navigation Links
					h.Nav(
						h.Class("hidden md:flex items-center gap-6"),
					h.A(
						h.Href("/blog"),
						h.Class("text-sm font-medium text-foreground hover:opacity-90 transition-colors"),
						g.Text("Latest"),
					),
					),
				),

				// Right side actions
				h.Div(
					h.Class("flex items-center gap-4"),
				g.If(isLoggedIn,
					g.Group([]g.Node{
					h.A(
						h.Href("/pulse"),
						h.Class("text-sm font-medium text-foreground hover:opacity-90 transition-colors mr-2"),
						g.Text("Dashboard"),
					),
							h.Img(
								h.Src(fmt.Sprintf("https://www.gravatar.com/avatar/%x?d=mp", sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(userEmail)))))),
								h.Alt("User Avatar"),
								h.Class("w-8 h-8 rounded-full bg-primary"),
							),
						}),
					),
					g.If(!isLoggedIn,
						g.Group([]g.Node{
							h.A(
								h.Href("/auth/login"),
								h.Class("text-sm font-medium text-foreground hover:opacity-90 transition-colors"),
								g.Text("Log in"),
							),
							h.A(
								h.Href("/auth/register"),
								h.Class("inline-flex items-center justify-center rounded-full px-4 py-2 text-sm font-medium ring-1 ring-inset ring-border hover:opacity-90 transition-all bg-secondary text-secondary-foreground"),
								g.Text("Sign up"),
								g.Raw(`<svg class="ml-2 -mr-1 w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14m-7-7 7 7-7 7"/></svg>`),
							),
						}),
					),
				),
			),
		),
	)
}

// MarketingFooter renders the footer
func MarketingFooter() g.Node {
	return h.Footer(
		h.Class("bg-background border-t border-border py-12 md:py-16"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("flex flex-col md:flex-row justify-between items-center gap-8"),

				// Brand
				h.Div(
					h.Class("flex flex-col items-center md:items-start"),
					h.A(
						h.Href("/"),
						h.Class("flex items-center gap-2 mb-2"),
						h.Img(
							h.Src("/static/logo-pbw.png"),
							h.Alt("Probably"),
							h.Class("w-6 h-6 dark:invert"),
						),
						h.Span(
							h.Class("text-lg font-bold tracking-tight text-foreground"),
							g.Text("probably"),
						),
					),
					h.P(
						h.Class("text-sm text-foreground"),
						g.Text("Financial clarity, finally."),
					),
				),

				// Links
				h.Div(
					h.Class("flex items-center gap-6"),
					h.A(
						h.Href("/blog"),
						h.Class("text-sm text-foreground hover:opacity-90 transition-colors"),
						g.Text("Latest"),
					),
					h.A(
						h.Href("/legal"),
						h.Class("text-sm text-foreground hover:opacity-90 transition-colors"),
						g.Text("Legal"),
					),
				),

				// Copyright
				h.P(
					h.Class("text-sm text-foreground"),
					g.Text("© 2025 Probably. All rights reserved."),
				),
			),
		),
	)
}
