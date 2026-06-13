package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func (hdl *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	userEmail, phID := userContext(r)

	// Get logo filenames for demo companies
	ctx := r.Context()
	logoFilenames := hdl.getDemoCompanyLogos(ctx)

	page := layouts.MarketingLayout("Financial Clarity", userEmail, phID,
		renderHero(userEmail != ""),
		renderValueProps(),
		renderIntelligenceSection(),
		renderDemoSection(),
		renderTransformationSection(hdl.getLogoURL, logoFilenames),
		renderAutomationSection(),
		renderPersonalizedAISection(),
		renderPricingSection(userEmail != ""),
		renderPrivacySection(),
		renderCtaSection(userEmail != ""),
	)

	renderHTML(w, page)
}

// getDemoCompanyLogos queries the database for demo companies and returns their logo filenames
func (hdl *Handlers) getDemoCompanyLogos(ctx context.Context) map[string]string {
	logoFilenames := make(map[string]string)

	// Company names to look up
	companyNames := []string{"Austin Roasters", "Amazon", "Uber", "Spotify"}

	for _, name := range companyNames {
		entity, err := hdl.entities.GetByName(ctx, name)
		if err == nil && entity != nil && entity.LogoURL != "" {
			logoFilenames[name] = entity.LogoURL
		}
	}

	return logoFilenames
}

func renderHero(isLoggedIn bool) g.Node {
	return h.Section(
		h.Class("relative pt-32 pb-20 md:pt-48 md:pb-32 overflow-hidden bg-background"),
		// Background gradient effect - simplified and robust
		h.Div(
			h.Class("absolute inset-0 bg-gradient-to-br from-accent/20 via-background to-background pointer-events-none"),
		),
		// Gradient orb 1
		h.Div(
			h.Class("absolute -top-40 -right-40 w-96 h-96 bg-accent/20 rounded-full blur-3xl pointer-events-none"),
		),
		// Gradient orb 2
		h.Div(
			h.Class("absolute top-20 -left-20 w-72 h-72 bg-accent/20 rounded-full blur-3xl pointer-events-none"),
		),

		h.Div(
			h.Class("relative z-10 max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 text-center"),
			h.Div(
				h.Class("animate-fade-in"),
				h.H1(
					h.Class("text-4xl md:text-6xl lg:text-7xl font-bold tracking-tight text-foreground mb-6"),
					g.Text("Financial clarity, "),
					h.Span(
						h.Class("text-foreground"),
						g.Text("without the work."),
					),
				),
				h.P(
					h.Class("max-w-2xl mx-auto text-lg md:text-xl text-foreground/90 mb-10 leading-relaxed"),
					g.Text("Probably builds a living, breathing picture of your financial life—always current, always accurate. Make decisions from clarity, not guesswork."),
				),
				h.Div(
					h.Class("flex flex-col sm:flex-row items-center justify-center gap-4"),
					g.If(isLoggedIn,
						h.A(
							h.Href("/pulse"),
							h.Class("inline-flex items-center justify-center rounded-full px-8 py-3.5 text-sm font-semibold shadow-sm hover:opacity-90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 transition-all transform hover:scale-105 bg-primary text-primary-foreground focus-visible:outline-ring"),
							g.Text("Go to Dashboard"),
							g.Raw(`<svg class="ml-2 -mr-1 w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14m-7-7 7 7-7 7"/></svg>`),
						),
					),
					g.If(!isLoggedIn,
						g.Group([]g.Node{
							h.A(
								h.Href("/auth/register"),
								h.Class("inline-flex items-center justify-center rounded-full px-8 py-3.5 text-sm font-semibold shadow-sm hover:opacity-90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 transition-all transform hover:scale-105 bg-primary text-primary-foreground focus-visible:outline-ring"),
								g.Text("Get Started"),
								g.Raw(`<svg class="ml-2 -mr-1 w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14m-7-7 7 7-7 7"/></svg>`),
							),
							h.A(
								h.Href("#features"),
								h.Class("inline-flex items-center justify-center rounded-full border border-border bg-transparent text-foreground px-8 py-3.5 text-sm font-semibold hover:bg-accent hover:text-accent-foreground transition-all"),
								g.Text("Learn more"),
							),
						}),
					),
				),
			),
		),
	)
}

func renderValueProps() g.Node {
	return h.Section(
		h.ID("features"),
		h.Class("py-24 bg-card border-y border-border/50"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			// Two column layout
			h.Div(
				h.Class("grid grid-cols-1 lg:grid-cols-2 gap-16 lg:gap-24"),
				// Left column - main message
				h.Div(
					h.Class("lg:sticky lg:top-32 lg:self-start"),
					h.P(
						h.Class("text-sm font-medium text-card-foreground mb-4"),
						g.Text("Why Probably exists"),
					),
					h.H2(
						h.Class("text-3xl font-semibold tracking-tight text-card-foreground sm:text-4xl mb-6"),
						g.Text("Know where you stand. See where you're going."),
					),
					h.P(
						h.Class("text-lg text-card-foreground leading-relaxed mb-6"),
						g.Text("Can I afford the vacation? Why does it feel tight when it shouldn't? Where's the opportunity I'm missing? These questions deserve clear answers."),
					),
					h.P(
						h.Class("text-lg text-card-foreground leading-relaxed"),
						g.Text("Probably gives you the complete picture—so you can feel confident today, make smarter choices tomorrow, and build toward what matters most."),
					),
				),
				// Right column - benefits
				h.Div(
					h.Class("space-y-12"),
					renderValueProp(
						"You log in to see, not to type",
						"Probably syncs your accounts, categorizes transactions with AI that learns from you, and spots transfers automatically. The system works while you sleep, so you can focus on living.",
						`<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v4m0 12v4M4.93 4.93l2.83 2.83m8.48 8.48l2.83 2.83M2 12h4m12 0h4M4.93 19.07l2.83-2.83m8.48-8.48l2.83-2.83"/></svg>`,
					),
					renderValueProp(
						"Context, not just numbers",
						"A $47 charge tells you what happened, not what it meant. Probably enriches every transaction with merchant logos, clean names, and categories—turning cryptic bank codes into a story you can actually read.",
						`<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/></svg>`,
					),
					renderValueProp(
						"No more 3am wondering",
						"The compound interest of anxiety is more destructive than any credit card rate. When you know exactly where you stand—net worth, burn rate, upcoming expenses—you stop waking up wondering if the numbers add up to safety.",
						`<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2a10 10 0 1 0 10 10H12V2z"/><path d="M12 2a10 10 0 0 1 10 10"/><path d="M12 12l7-7"/></svg>`,
					),
					renderValueProp(
						"Your data, forever",
						"Your financial history belongs to you. One click exports everything to portable JSON—human-readable, completely portable. Your records go wherever you go.",
						`<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>`,
					),
				),
			),
		),
	)
}

func renderValueProp(title, description, icon string) g.Node {
	return h.Div(
		h.Class("flex gap-5"),
		h.Div(
			h.Class("flex-none w-10 h-10 bg-secondary border border-border rounded-lg flex items-center justify-center text-secondary-foreground"),
			g.Raw(icon),
		),
		h.Div(
			h.H3(
				h.Class("text-lg font-medium text-card-foreground mb-2"),
				g.Text(title),
			),
			h.P(
				h.Class("text-card-foreground leading-relaxed"),
				g.Text(description),
			),
		),
	)
}

func renderIntelligenceSection() g.Node {
	return h.Section(
		h.Class("py-24 relative overflow-hidden"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("lg:grid lg:grid-cols-2 lg:gap-16 items-center"),
				// Left Content
				h.Div(
					h.Class("mb-12 lg:mb-0"),
					h.Div(
						h.Class("inline-flex items-center rounded-full border border-border bg-accent px-3 py-1 text-sm font-medium text-accent-foreground mb-6"),
						g.Text("Intelligence Dashboard"),
					),
					h.H2(
						h.Class("text-3xl font-bold tracking-tight text-foreground sm:text-4xl mb-6"),
						g.Text("The End of \"Financial Fog\""),
					),
					h.Div(
						h.Class("space-y-6 text-foreground"),
						h.P(
							h.Class("text-lg"),
							g.Text("Imagine knowing your true burn rate and net worth at any moment, not just at the end of the month. That clarity changes everything."),
						),
						h.P(
							h.Class("text-lg"),
							g.Text("Our AI Insights engine doesn't just list accounts; it calculates your real-time position. It spots anomalies, tells you if a subscription quietly increased, and acts as a proactive analyst."),
						),
						h.Ul(
							h.Class("space-y-4 mt-8"),
							renderCheckItem("Real-time Net Worth calculation"),
							renderCheckItem("AI-detected spending anomalies"),
							renderCheckItem("Trend analysis for every category"),
						),
					),
				),
				// Right Visual - Abstract Dashboard
				h.Div(
					h.Class("relative"),
					h.Div(
						h.Class("absolute -inset-4 bg-gradient-to-r from-accent/20 to-accent/10 rounded-2xl blur-xl"),
					),
					h.Div(
						h.Class("relative bg-card border border-border rounded-2xl p-6 shadow-2xl"),
						// Mock Insight Card
						h.Div(
							h.Class("bg-background border border-border rounded-xl p-5 mb-4"),
							h.Div(
								h.Class("flex items-center justify-between mb-3"),
								h.Div(
									h.Class("flex items-center gap-3"),
									h.Div(h.Class("w-8 h-8 rounded-full bg-ring/20 flex items-center justify-center text-ring"),
										g.Raw(`<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>`),
									),
									h.Span(h.Class("text-sm font-medium text-card-foreground"), g.Text("Spending Alert")),
								),
								h.Span(h.Class("text-xs text-card-foreground/70"), g.Text("Just now")),
							),
							h.P(h.Class("text-sm text-card-foreground"), g.Text("Oh it's birthday month, YOLO!")),
						),
						// Mock Chart Area
						h.Div(
							h.Class("bg-background border border-border rounded-xl p-5"),
							h.Div(h.Class("h-40 flex items-end justify-between gap-2 px-2"),
								renderBar(40), renderBar(65), renderBar(45), renderBar(80),
								renderBar(55), renderBar(90), renderBar(70),
							),
						),
					),
				),
			),
		),
	)
}

// New sections for "Transformation" and "Automation"

func renderTransformationSection(getLogoURL func(string) string, logoFilenames map[string]string) g.Node {
	return h.Section(
		h.Class("py-24 bg-card relative border-t border-border/50"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("text-center max-w-3xl mx-auto mb-16"),
				h.H2(
					h.Class("text-3xl font-bold tracking-tight text-card-foreground sm:text-4xl mb-4"),
					g.Text("Turn Data into Meaning"),
				),
				h.P(
					h.Class("text-lg text-card-foreground"),
					g.Text("Raw bank data is noisy and confusing. We transform it into a clean, visual narrative of your spending."),
				),
			),
			h.Div(
				h.Class("flex flex-col lg:flex-row items-center gap-12"),
				// Before
				h.Div(
					h.Class("flex-1 w-full space-y-4"),
					h.P(h.Class("text-sm font-semibold text-card-foreground uppercase tracking-wider text-center lg:text-left"), g.Text("Before: The Bank Statement")),
					h.Div(
						h.Class("bg-secondary/50 border border-border rounded-xl p-6 space-y-4 font-mono text-sm"),
						h.Div(h.Class("flex justify-between text-card-foreground/70"), g.Text("TST* AUSTIN ROASTERS 4452"), g.Text("$4.50")),
						h.Div(h.Class("flex justify-between text-card-foreground/70"), g.Text("AMZN MKTP US*2X4Y LE"), g.Text("$24.99")),
						h.Div(h.Class("flex justify-between text-card-foreground/70"), g.Text("UBER* TRIP San Francisco"), g.Text("$18.42")),
						h.Div(h.Class("flex justify-between text-card-foreground/70"), g.Text("SPOTIFY USA NEW YORK"), g.Text("$11.99")),
					),
				),
				// Arrow
				h.Div(
					h.Class("flex-none text-card-foreground/70 rotate-90 lg:rotate-0"),
					g.Raw(`<svg class="w-8 h-8" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14m-7-7 7 7-7 7"/></svg>`),
				),
				// After
				h.Div(
					h.Class("flex-1 w-full space-y-4"),
					h.P(h.Class("text-sm font-semibold text-card-foreground uppercase tracking-wider text-center lg:text-left"), g.Text("After: Probably")),
					h.Div(
						h.Class("bg-card border border-border rounded-xl overflow-hidden shadow-2xl"),
						renderEnrichedRow("Austin Roasters", "Coffee & Tea", "$4.50", logoFilenames["Austin Roasters"], getLogoURL),
						renderEnrichedRow("Amazon", "Shopping", "$24.99", logoFilenames["Amazon"], getLogoURL),
						renderEnrichedRow("Uber", "Transportation", "$18.42", logoFilenames["Uber"], getLogoURL),
						renderEnrichedRow("Spotify", "Entertainment", "$11.99", logoFilenames["Spotify"], getLogoURL),
					),
				),
			),
		),
	)
}

func renderEnrichedRow(name, category, amount, logoFilename string, getLogoURL func(string) string) g.Node {
	// If logoFilename is empty, try to use a fallback (empty string will show placeholder)
	logoURL := ""
	if logoFilename != "" {
		logoURL = getLogoURL(logoFilename)
	}
	return h.Div(
		h.Class("flex items-center justify-between p-4 border-b border-border last:border-0 hover:bg-accent transition-colors"),
		h.Div(
			h.Class("flex items-center gap-4"),
			g.If(logoURL != "",
				h.Img(
					h.Src(logoURL),
					h.Alt(name),
					h.Class("w-10 h-10 rounded-lg object-contain bg-secondary"),
				),
			),
			g.If(logoURL == "",
				h.Div(
					h.Class("w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground text-xs font-medium"),
					g.Text(string([]rune(name)[0])),
				),
			),
			h.Div(
				h.H4(h.Class("text-sm font-medium text-foreground"), g.Text(name)),
				h.P(h.Class("text-xs text-muted-foreground"), g.Text(category)),
			),
		),
		h.Span(h.Class("text-sm font-medium text-foreground font-mono"), g.Text(amount)),
	)
}

func renderAutomationSection() g.Node {
	return h.Section(
		h.Class("py-24 bg-card border-y border-border/50"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("lg:grid lg:grid-cols-2 lg:gap-16 items-center"),
				// Content
				h.Div(
					h.Class("mb-12 lg:mb-0"),
					h.Div(
						h.Class("inline-flex items-center rounded-full border border-border bg-accent px-3 py-1 text-sm font-medium text-accent-foreground mb-6"),
						g.Text("AI-Powered Assistant"),
					),
					h.H2(
						h.Class("text-3xl font-bold tracking-tight text-card-foreground sm:text-4xl mb-6"),
						g.Text("Let AI handle the busywork"),
					),
					h.Div(
						h.Class("space-y-6 text-card-foreground"),
						h.P(
							h.Class("text-lg text-card-foreground"),
							g.Text("Probably's AI engine understands your transactions the way a thoughtful accountant would—learning your patterns, not just matching keywords."),
						),
						h.P(
							h.Class("text-lg text-card-foreground"),
							g.Text("It automatically identifies merchants, categorizes spending based on context, and even spots subscriptions or anomalies—all without you lifting a finger."),
						),
					),
				),
				// Visual - AI Processing Steps
				h.Div(
					h.Class("relative"),
					h.Div(
						h.Class("absolute -inset-4 bg-gradient-to-r from-accent/20 to-accent/10 rounded-2xl blur-xl"),
					),
					h.Div(
						h.Class("relative bg-card border border-border rounded-2xl p-8 shadow-2xl"),
						h.Div(
							h.Class("space-y-6"),
							renderAIStep("Matching transaction to known companies...", true),
							renderAIStep("Searching company to understand context...", true),
							renderAIStep("Understanding how this fits in your history...", true),
							renderAIStep("Generating spending insights...", true),
							renderAIStep("Updating financial reports...", false),
						),
					),
				),
			),
		),
	)
}

func renderAIStep(text string, completed bool) g.Node {
	icon := g.Raw(`<svg class="w-5 h-5 text-secondary-foreground animate-pulse" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/></svg>`)
	textClass := "text-card-foreground"
	iconBgClass := "bg-secondary border-border"

	if completed {
		icon = g.Raw(`<svg class="w-5 h-5 text-secondary-foreground" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 6L9 17l-5-5"/></svg>`)
		textClass = "text-card-foreground"
		iconBgClass = "bg-secondary border-border"
	}

	return h.Div(
		h.Class("flex items-center gap-4"),
		h.Div(
			h.Class("flex-none w-8 h-8 rounded-full "+iconBgClass+" flex items-center justify-center"),
			icon,
		),
		h.P(h.Class("text-sm font-medium "+textClass), g.Text(text)),
	)
}

func renderPersonalizedAISection() g.Node {
	return h.Section(
		h.Class("py-24 bg-background relative"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("lg:grid lg:grid-cols-2 lg:gap-16 items-center"),
				// Visual - Personalized Context
				h.Div(
					h.Class("relative"),
					h.Div(
						h.Class("absolute -inset-4 bg-gradient-to-r from-accent/20 to-accent/10 rounded-2xl blur-xl"),
					),
					h.Div(
						h.Class("relative bg-card border border-border rounded-2xl p-8 shadow-2xl"),
						h.Div(
							h.Class("space-y-6"),
							// Scenario 1: Generic
							h.Div(
								h.Class("p-4 bg-background border border-border rounded-xl opacity-50"),
								h.Div(
									h.Class("flex items-center gap-3 mb-2"),
									h.Div(h.Class("w-8 h-8 rounded-full bg-secondary flex items-center justify-center text-card-foreground/60"), g.Raw(`<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 9l6 6 6-6"/></svg>`)),
									h.P(h.Class("text-sm font-medium text-card-foreground/70"), g.Text("Generic App: \"Expense\"")),
								),
							),
							// Scenario 2: Personalized
							h.Div(
								h.Class("p-4 bg-background border border-border rounded-xl"),
								h.Div(
									h.Class("flex items-center gap-3 mb-3"),
									h.Div(h.Class("w-8 h-8 rounded-full bg-accent flex items-center justify-center text-foreground"), g.Raw(`<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>`)),
									h.P(h.Class("text-sm font-medium text-card-foreground"), g.Text("Probably AI:")),
								),
								h.P(h.Class("text-sm text-card-foreground leading-relaxed"),
									g.Text("\"I see this is a recurring payment to 'AWS' on the 3rd. Based on your history, this is likely infrastructure for your side project, not a personal subscription. Categorizing as 'Business Expense' and tagging 'Project Alpha'.\""),
								),
							),
						),
					),
				),
				// Content
				h.Div(
					h.Class("mt-12 lg:mt-0"),
					h.H2(
						h.Class("text-3xl font-bold tracking-tight text-foreground sm:text-4xl mb-6"),
						g.Text("Context is Everything"),
					),
					h.Div(
						h.Class("space-y-6 text-foreground"),
						h.P(
							h.Class("text-lg text-foreground"),
							g.Text("The reason personal finance apps seem so tedious is they treat everyone's transactions the same. A $50 payment to 'Shell' is gas for one person, but a snack run for another."),
						),
						h.P(
							h.Class("text-lg text-foreground"),
							g.Text("Probably burns AI cycles understanding how every transaction matters to YOU. It learns your patterns, your projects, and your life, turning generic data into a personalized financial map."),
						),
					),
				),
			),
		),
	)
}

func renderPricingSection(isLoggedIn bool) g.Node {
	return h.Section(
		h.Class("py-24 bg-card border-y border-border/50"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("text-center max-w-3xl mx-auto mb-16"),
				h.H2(
					h.Class("text-3xl font-bold tracking-tight text-card-foreground sm:text-4xl mb-4"),
					g.Text("Simple, transparent pricing"),
				),
				h.P(
					h.Class("text-lg text-card-foreground"),
					g.Text("Choose how you want to run Probably. No hidden fees, no data selling."),
				),
			),
			h.Div(
				h.Class("grid grid-cols-1 md:grid-cols-2 gap-8 max-w-4xl mx-auto"),
				// Self-Hosted Card
				h.Div(
					h.Class("relative p-8 bg-card border border-border rounded-2xl flex flex-col"),
					h.H3(h.Class("text-xl font-semibold text-card-foreground mb-2"), g.Text("Self-Hosted")),
					h.P(h.Class("text-card-foreground/80 mb-6"), g.Text("For the DIY enthusiasts who want total control.")),
					h.Div(
						h.Class("flex items-baseline mb-8"),
						h.Span(h.Class("text-4xl font-bold text-card-foreground"), g.Text("Free")),
						h.Span(h.Class("text-card-foreground/70 ml-2"), g.Text("forever")),
					),
					h.Ul(
						h.Class("space-y-4 mb-8 flex-1"),
						renderPricingCheck("Host on your own hardware"),
						renderPricingCheck("Bring your own API keys (LLM, etc)"),
						renderPricingCheck("Full feature set"),
						renderPricingCheck("Community support"),
					),
					h.A(
						h.Href("https://github.com/asomervell/probably"),
						h.Target("_blank"),
						h.Class("w-full inline-flex items-center justify-center rounded-lg px-4 py-2.5 text-sm font-semibold transition-colors bg-secondary text-secondary-foreground hover:opacity-90 border border-border"),
						g.Text("View on GitHub"),
					),
				),
				// Cloud Card
				h.Div(
					h.Class("relative p-8 bg-card/50 border border-border rounded-2xl flex flex-col relative overflow-hidden"),
					h.Div(
						h.Class("absolute top-0 right-0 -mt-2 -mr-2 w-24 h-24 bg-accent/20 blur-2xl rounded-full pointer-events-none"),
					),
					h.Div(
						h.Class("absolute top-4 right-4 inline-flex items-center rounded-full bg-ring px-2.5 py-0.5 text-xs font-medium text-foreground border border-border"),
						g.Text("Most Popular"),
					),
					h.H3(h.Class("text-xl font-semibold text-card-foreground mb-2"), g.Text("Probably Cloud")),
					h.P(h.Class("text-card-foreground/80 mb-4"), g.Text("We handle the infrastructure and AI costs for you.")),
					h.Div(
						h.Class("mb-4 p-3 bg-accent border border-border rounded-lg"),
						h.P(h.Class("text-sm text-card-foreground"),
							g.Text("45-day free trial • We want you to see a full month to really understand how game-changing this is"),
						),
					),
					h.Div(
						h.Class("flex items-baseline mb-8"),
						h.Span(h.Class("text-4xl font-bold text-card-foreground"), g.Text("$9")),
						h.Span(h.Class("text-card-foreground/70 ml-2"), g.Text("/month")),
					),
					h.Ul(
						h.Class("space-y-4 mb-8 flex-1"),
						renderPricingCheck("Instant setup & zero maintenance"),
						renderPricingCheck("AI costs included (Grok/Groq)"),
						renderPricingCheck("Automatic updates & backups"),
						renderPricingCheck("Priority support"),
					),
					g.If(!isLoggedIn,
						h.A(
							h.Href("/auth/register?plan=cloud"),
							h.Class("w-full inline-flex items-center justify-center rounded-lg px-4 py-2.5 text-sm font-semibold shadow-sm hover:opacity-90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 transition-colors bg-foreground text-background focus-visible:outline-ring"),
							g.Text("Start 45-Day Free Trial"),
						),
					),
					g.If(isLoggedIn,
						h.A(
							h.Href("/settings/billing"),
							h.Class("w-full inline-flex items-center justify-center rounded-lg px-4 py-2.5 text-sm font-semibold shadow-sm hover:opacity-90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 transition-colors bg-foreground text-background focus-visible:outline-ring"),
							g.Text("Upgrade to Cloud"),
						),
					),
				),
			),
		),
	)
}

func renderPricingCheck(text string) g.Node {
	return h.Li(
		h.Class("flex items-center gap-3 text-card-foreground"),
		h.Div(
			h.Class("flex-none w-5 h-5 rounded-full bg-accent flex items-center justify-center text-foreground"),
			g.Raw(`<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="20 6 9 17 4 12"/></svg>`),
		),
		g.Text(text),
	)
}

func renderPrivacySection() g.Node {
	return h.Section(
		h.Class("py-24 bg-background relative"),
		h.Div(
			h.Class("max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 text-center"),
			h.H2(
				h.Class("text-3xl font-bold tracking-tight text-foreground sm:text-4xl mb-6"),
				g.Text("Your Data, Your Rules"),
			),
			h.P(
				h.Class("text-lg text-foreground/90 mb-12"),
				g.Text("We believe financial data is sensitive and personal. You shouldn't have to trade your privacy for clarity."),
			),
			h.Div(
				h.Class("grid grid-cols-1 md:grid-cols-3 gap-8"),
				renderPrivacyCard("Encrypted Ledger", "Every customer gets their own isolated ledger, encrypted with a unique key. Your financial data is for your eyes only.", `<svg class="w-8 h-8 text-foreground" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>`),
				renderPrivacyCard("Portable", "Export your entire ledger to JSON at any time. You are never locked in.", `<svg class="w-8 h-8 text-foreground" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>`),
				renderPrivacyCard("Private", "We don't sell your data. We don't show you ads. You are the customer, not the product.", `<svg class="w-8 h-8 text-foreground" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>`),
			),
		),
	)
}

func renderPrivacyCard(title, description, icon string) g.Node {
	return h.Div(
		h.Class("p-6 bg-card/50 border border-border rounded-xl"),
		h.Div(
			h.Class("w-12 h-12 bg-secondary rounded-full flex items-center justify-center mx-auto mb-4"),
			g.Raw(icon),
		),
		h.H3(h.Class("text-lg font-semibold text-card-foreground mb-2"), g.Text(title)),
		h.P(h.Class("text-sm text-card-foreground/80"), g.Text(description)),
	)
}

func renderCtaSection(isLoggedIn bool) g.Node {
	return h.Section(
		h.Class("py-32 relative overflow-hidden bg-secondary"),
		h.Div(
			h.Class("absolute inset-0 bg-accent/10 pointer-events-none"),
		),
		h.Div(
			h.Class("max-w-4xl mx-auto px-4 text-center relative z-10"),
			h.H2(
				h.Class("text-3xl font-bold tracking-tight text-secondary-foreground sm:text-4xl mb-6"),
				g.Text("Ready to clarify your finances?"),
			),
			h.P(
				h.Class("text-lg text-secondary-foreground mb-10 max-w-2xl mx-auto"),
				g.Text("Join the others who have stopped guessing and started knowing."),
			),
			g.If(!isLoggedIn,
				h.A(
					h.Href("/auth/register"),
					h.Class("inline-flex items-center justify-center rounded-full px-8 py-4 text-base font-semibold shadow-sm hover:opacity-90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 transition-all transform hover:scale-105 bg-primary text-primary-foreground focus-visible:outline-ring"),
					g.Text("Start for Free"),
					g.Raw(`<svg class="ml-2 -mr-1 w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14m-7-7 7 7-7 7"/></svg>`),
				),
			),
			g.If(isLoggedIn,
				h.A(
					h.Href("/pulse"),
					h.Class("inline-flex items-center justify-center rounded-full px-8 py-4 text-base font-semibold shadow-sm hover:opacity-90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 transition-all transform hover:scale-105 bg-primary text-primary-foreground focus-visible:outline-ring"),
					g.Text("Go to Dashboard"),
				),
			),
		),
	)
}

func renderCheckItem(text string) g.Node {
	return h.Li(
		h.Class("flex items-center gap-3 text-foreground"),
		h.Div(
			h.Class("flex-none w-5 h-5 rounded-full bg-accent flex items-center justify-center text-foreground"),
			g.Raw(`<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="20 6 9 17 4 12"/></svg>`),
		),
		g.Text(text),
	)
}

// Helpers for mock visuals

func renderBar(height int) g.Node {
	return h.Div(
		h.Class("w-full bg-foreground rounded-t-sm hover:opacity-90 transition-colors"),
		h.Style("height: "+strconv.Itoa(height)+"%"),
	)
}

func renderDemoSection() g.Node {
	return h.Div(
		h.Class("bg-card border-y border-border/50"),
		// Pulse Dashboard Section - Text left, Panel right
		h.Section(
			h.Class("py-24"),
			h.Div(
				h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
				h.Div(
					h.Class("grid grid-cols-1 lg:grid-cols-2 gap-12 items-center"),
					// Left: Text content
					h.Div(
						h.H2(
							h.Class("text-3xl font-bold tracking-tight text-card-foreground mb-4"),
							g.Text("Know where you stand"),
						),
						h.P(
							h.Class("text-lg text-card-foreground/80 mb-4"),
							g.Text("The Pulse dashboard gives you an instant financial snapshot. See your net worth, left to spend, upcoming bills, and spending pace—all in one place."),
						),
						h.Ul(
							h.Class("space-y-3 text-card-foreground/80"),
							renderCheckItem("Real-time net worth calculation"),
							renderCheckItem("Left to spend with upcoming bills"),
							renderCheckItem("Spending pace vs last month"),
						),
					),
					// Right: Demo panel
					h.Div(
						h.Class("relative"),
						// Loading skeleton (shown during request)
						h.Div(
							h.Class("htmx-indicator absolute inset-0 space-y-4"),
							h.Div(h.Class("h-32 bg-secondary animate-pulse rounded-xl")),
							h.Div(h.Class("h-48 bg-secondary animate-pulse rounded-xl")),
						),
						// Content container
						h.Div(
							h.ID("demo-pulse"),
							g.Attr("hx-get", "/demo/pulse"),
							g.Attr("hx-trigger", "load, every 15s"),
							g.Attr("hx-swap", "innerHTML"),
						),
					),
				),
			),
		),
		// Transactions Section - Panel left, Text right
		h.Section(
			h.Class("py-24 border-t border-border/50"),
			h.Div(
				h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
				h.Div(
					h.Class("grid grid-cols-1 lg:grid-cols-2 gap-12 items-center"),
					// Left: Demo panel
					h.Div(
						h.Class("space-y-4"),
						// Search input
						h.Form(
							g.Attr("hx-get", "/demo/transactions"),
							g.Attr("hx-target", "#demo-transactions"),
							g.Attr("hx-trigger", "input delay:300ms"),
							h.Class("mb-4"),
							shadcn.Input(shadcn.InputProps{
								Type:        "text",
								Name:        "search",
								Placeholder: "Search transactions...",
							}),
						),
						// Transactions list
						h.Div(
							h.Class("relative"),
							// Loading skeleton (shown during request)
							h.Div(
								h.Class("htmx-indicator absolute inset-0 space-y-2"),
								h.Div(h.Class("h-16 bg-secondary animate-pulse rounded")),
								h.Div(h.Class("h-16 bg-secondary animate-pulse rounded")),
								h.Div(h.Class("h-16 bg-secondary animate-pulse rounded")),
							),
							// Content container
							h.Div(
								h.ID("demo-transactions"),
								g.Attr("hx-get", "/demo/transactions"),
								g.Attr("hx-trigger", "load"),
								g.Attr("hx-swap", "innerHTML"),
							),
						),
					),
					// Right: Text content
					h.Div(
						h.H2(
							h.Class("text-3xl font-bold tracking-tight text-card-foreground mb-4"),
							g.Text("Every transaction, enriched"),
						),
						h.P(
							h.Class("text-lg text-card-foreground/80 mb-4"),
							g.Text("Raw bank data becomes meaningful. Merchant logos, clean names, and smart categorization turn cryptic codes into a story you can actually read."),
						),
						h.Ul(
							h.Class("space-y-3 text-card-foreground/80"),
							renderCheckItem("Automatic merchant recognition"),
							renderCheckItem("Smart categorization with AI"),
							renderCheckItem("Real-time search and filtering"),
						),
					),
				),
			),
		),
		// Chat Section - Text left, Panel right
		h.Section(
			h.Class("py-24 border-t border-border/50"),
			h.Div(
				h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
				h.Div(
					h.Class("grid grid-cols-1 lg:grid-cols-2 gap-12 items-center"),
					// Left: Text content
					h.Div(
						h.H2(
							h.Class("text-3xl font-bold tracking-tight text-card-foreground mb-4"),
							g.Text("Ask anything, get answers"),
						),
						h.P(
							h.Class("text-lg text-card-foreground/80 mb-4"),
							g.Text("Stop digging through spreadsheets. Ask questions in plain English and get instant answers with charts, tables, and insights."),
						),
						h.Ul(
							h.Class("space-y-3 text-card-foreground/80"),
							renderCheckItem("Ask in plain English—no formulas needed"),
							renderCheckItem("Get instant answers with charts and tables"),
							renderCheckItem("Understand your spending patterns at a glance"),
						),
					),
					// Right: Demo panel
					h.Div(
						h.Class("bg-card border border-border rounded-xl overflow-hidden flex flex-col"),
						h.Div(
							h.Class("h-[500px] flex flex-col"),
							// Messages area
							h.Div(
								h.Class("flex-1 overflow-y-auto p-4 space-y-4 relative"),
								// Loading skeleton (shown during request)
								h.Div(
									h.Class("htmx-indicator absolute inset-0 p-4 space-y-4"),
									h.Div(h.Class("h-12 bg-secondary animate-pulse rounded-lg ml-auto w-3/4")),
									h.Div(h.Class("h-16 bg-secondary animate-pulse rounded-lg w-full")),
								),
								// Content container
								h.Div(
									h.ID("demo-chat-messages"),
									g.Attr("hx-get", "/demo/chat"),
									g.Attr("hx-trigger", "load"),
									g.Attr("hx-swap", "innerHTML"),
									h.Class("relative"),
								),
							),
							// Input area
							h.Div(
								h.Class("border-t border-border p-4"),
								h.Form(
									g.Attr("hx-post", "/demo/chat/message"),
									g.Attr("hx-target", "#demo-chat-messages"),
									g.Attr("hx-swap", "beforeend"),
									g.Attr("hx-on::after-request", "this.reset()"),
									h.Class("flex gap-2"),
									shadcn.Input(shadcn.InputProps{
										Type:        "text",
										Name:        "message",
										Placeholder: "Ask about your finances...",
										Class:       "flex-1",
									}),
									shadcn.Button(shadcn.ButtonProps{
										Type:  "submit",
										Class: "shrink-0",
									}, g.Text("Send")),
								),
							),
						),
					),
				),
			),
		),
	)
}
