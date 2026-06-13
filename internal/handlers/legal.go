package handlers

import (
	"log/slog"
	"net/http"

	"github.com/asomervell/probably/internal/legal"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/go-chi/chi/v5"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// LegalPage represents a single legal page
type LegalPage struct {
	Slug        string
	Title       string
	Description string
	FileName    string
}

// legalPages - the available legal documents
var legalPages = []LegalPage{
	{
		Slug:        "privacy",
		Title:       "Privacy Policy",
		Description: "How we collect, use, and protect your personal data.",
		FileName:    "privacy-policy.md",
	},
	{
		Slug:        "terms",
		Title:       "Terms of Service",
		Description: "The rules and regulations for using Probably.",
		FileName:    "terms-of-service.md",
	},
	{
		Slug:        "cookies",
		Title:       "Cookie Policy",
		Description: "Information about how we use cookies on our website.",
		FileName:    "cookie-policy.md",
	},
}

// LegalIndex renders the legal listing page
func (hdl *Handlers) LegalIndex(w http.ResponseWriter, r *http.Request) {
	userEmail, phID := userContext(r)

	page := layouts.MarketingLayout("Legal", userEmail, phID,
		renderLegalIndex(),
	)

	renderHTML(w, page)
}

// LegalPage renders a single legal policy page
func (hdl *Handlers) LegalPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var selectedPage *LegalPage
	for i := range legalPages {
		if legalPages[i].Slug == slug {
			selectedPage = &legalPages[i]
			break
		}
	}

	if selectedPage == nil {
		http.NotFound(w, r)
		return
	}

	// Read the markdown file from embedded filesystem
	content, err := legal.ReadPolicy(selectedPage.FileName)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to read legal file", "file", selectedPage.FileName, "err", err)
		http.Error(w, "Failed to load legal document", http.StatusInternalServerError)
		return
	}

	// Convert markdown to HTML
	contentHTML := renderMarkdownToHTML(string(content))

	userEmail, phID := userContext(r)

	page := layouts.MarketingLayout(selectedPage.Title, userEmail, phID,
		renderLegalPage(selectedPage, contentHTML),
	)

	renderHTML(w, page)
}

func renderLegalIndex() g.Node {
	return h.Section(
		h.Class("pt-32 pb-24 bg-background"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("lg:grid lg:grid-cols-12 lg:gap-16"),
				h.Div(
					h.Class("lg:col-span-8 lg:col-start-3"),
					h.Div(
						h.Class("mb-12"),
						h.P(
							h.Class("text-sm font-medium text-primary mb-3"),
							g.Text("Legal"),
						),
						h.H1(
							h.Class("text-3xl font-semibold tracking-tight text-foreground sm:text-4xl mb-4"),
							g.Text("Policies and terms"),
						),
						h.P(
							h.Class("text-base text-muted-foreground leading-relaxed max-w-xl"),
							g.Text("Our commitment to privacy, security, and transparency."),
						),
					),
					h.Div(
						h.Class("divide-y divide-border/50"),
						g.Group(g.Map(legalPages, func(lp LegalPage) g.Node {
							return h.Article(
								h.Class("group py-8 first:pt-0"),
								h.A(
									h.Href("/legal/"+lp.Slug),
									h.Class("block"),
									h.H2(
										h.Class("text-lg font-medium text-foreground group-hover:text-chart-3 transition-colors mt-2 mb-1"),
										g.Text(lp.Title),
									),
									h.P(
										h.Class("text-sm text-muted-foreground"),
										g.Text(lp.Description),
									),
								),
							)
						})),
					),
				),
			),
		),
	)
}

func renderLegalPage(lp *LegalPage, contentHTML string) g.Node {
	return h.Article(
		h.Class("pt-32 pb-24 bg-background"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("lg:grid lg:grid-cols-12 lg:gap-16"),
				h.Div(
					h.Class("lg:col-span-8 lg:col-start-3"),
					h.A(
						h.Href("/legal"),
						h.Class("inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors mb-10"),
						g.Raw(`<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 12H5m7 7-7-7 7-7"/></svg>`),
						g.Text("Legal Overview"),
					),
					h.Header(
						h.Class("mb-10"),
						h.H1(
							h.Class("text-2xl sm:text-3xl font-semibold tracking-tight text-foreground mt-3 mb-3"),
							g.Text(lp.Title),
						),
						h.P(
							h.Class("text-base text-muted-foreground"),
							g.Text(lp.Description),
						),
					),
					h.Div(
						h.Class("prose prose-p:text-muted-foreground prose-p:leading-relaxed prose-headings:text-foreground prose-headings:font-medium prose-a:text-primary prose-strong:text-foreground prose-strong:font-semibold max-w-none"),
						g.Raw(contentHTML),
					),
				),
			),
		),
	)
}
