package layouts

import (
	"crypto/sha256"
	"fmt"
	"strings"

	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

// BaseWithMetaURLAndToasts renders the base HTML layout with all options including toasts
func BaseWithMetaURLAndToasts(title string, description string, baseURL string, toasts []g.Node, posthogDistinctID string, body ...g.Node) g.Node {
	fullTitle := title + " | Probably"

	// Build OG image URL - use absolute if baseURL provided, otherwise relative
	ogImagePath := "/static/favicon-512x512.png"
	ogImage := ogImagePath
	if baseURL != "" {
		ogImage = strings.TrimSuffix(baseURL, "/") + ogImagePath
	}

	return c.HTML5(c.HTML5Props{
		Title:    fullTitle,
		Language: "en",
		Head: []g.Node{
			// Theme detection script - must run immediately to prevent flash.
			// Preference is stored in localStorage ("light", "dark", or "system").
			h.Script(g.Raw(`
				(function() {
					const THEME_KEY = 'probably-theme';
					const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
					const validPreferences = ['light', 'dark', 'system'];

					const readThemePreference = () => {
						try {
							const stored = localStorage.getItem(THEME_KEY);
							if (stored && validPreferences.includes(stored)) {
								return stored;
							}
						} catch (_) {}
						return 'system';
					};

					const resolveTheme = (preference) => {
						if (preference === 'dark') return 'dark';
						if (preference === 'light') return 'light';
						return mediaQuery.matches ? 'dark' : 'light';
					};

					const applyResolvedTheme = (resolvedTheme) => {
						if (resolvedTheme === 'dark') {
							document.documentElement.classList.add('dark');
						} else {
							document.documentElement.classList.remove('dark');
						}
					};

					const applyThemePreference = (preference) => {
						const resolved = resolveTheme(preference);
						applyResolvedTheme(resolved);
						document.documentElement.setAttribute('data-theme-preference', preference);
						document.documentElement.setAttribute('data-theme-resolved', resolved);
					};

					window.__probablyTheme = {
						key: THEME_KEY,
						getPreference: readThemePreference,
						setPreference: (preference) => {
							const safePreference = validPreferences.includes(preference) ? preference : 'system';
							try {
								localStorage.setItem(THEME_KEY, safePreference);
							} catch (_) {}
							applyThemePreference(safePreference);
						},
						applyCurrentPreference: () => applyThemePreference(readThemePreference()),
					};

					// Apply immediately.
					applyThemePreference(readThemePreference());

					// Re-resolve only for "system" when the OS setting changes.
					const onPreferenceChange = () => {
						if (readThemePreference() === 'system') {
							applyThemePreference('system');
						}
					};
					if (typeof mediaQuery.addEventListener === 'function') {
						mediaQuery.addEventListener('change', onPreferenceChange);
					} else if (typeof mediaQuery.addListener === 'function') {
						mediaQuery.addListener(onPreferenceChange);
					}
				})();
			`)),
			// Viewport and theme
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1, viewport-fit=cover")),
			h.Meta(h.Name("theme-color"), g.Attr("media", "(prefers-color-scheme: light)"), h.Content("#F6F6F3")),
			h.Meta(h.Name("theme-color"), g.Attr("media", "(prefers-color-scheme: dark)"), h.Content("#14120b")),
			h.Meta(h.Name("description"), h.Content(description)),

			// Favicons
			h.Link(h.Rel("icon"), h.Type("image/png"), g.Attr("sizes", "32x32"), h.Href("/static/favicon-32x32.png")),
			h.Link(h.Rel("icon"), h.Type("image/png"), g.Attr("sizes", "16x16"), h.Href("/static/favicon-16x16.png")),
			h.Link(h.Rel("apple-touch-icon"), g.Attr("sizes", "180x180"), h.Href("/static/apple-touch-icon.png")),
			h.Link(h.Rel("manifest"), h.Href("/static/site.webmanifest")),

			// OpenGraph meta tags
			h.Meta(g.Attr("property", "og:type"), h.Content("website")),
			h.Meta(g.Attr("property", "og:title"), h.Content(fullTitle)),
			h.Meta(g.Attr("property", "og:description"), h.Content(description)),
			h.Meta(g.Attr("property", "og:image"), h.Content(ogImage)),
			h.Meta(g.Attr("property", "og:site_name"), h.Content("Probably")),

			// Twitter Card meta tags
			h.Meta(h.Name("twitter:card"), h.Content("summary")),
			h.Meta(h.Name("twitter:title"), h.Content(fullTitle)),
			h.Meta(h.Name("twitter:description"), h.Content(description)),
			h.Meta(h.Name("twitter:image"), h.Content(ogImage)),

			// Stylesheets
			h.Link(h.Rel("stylesheet"), h.Href("/static/output.css")),
			// HTMX
			h.Script(h.Src("https://unpkg.com/htmx.org@2.0.3")),
			posthogHeadFragment(posthogDistinctID),
			// HTMX indicator styles and shadcn animation styles
			h.StyleEl(g.Raw(`
				.htmx-indicator { opacity: 0; transition: opacity 200ms ease-in; }
				.htmx-request .htmx-indicator, .htmx-request.htmx-indicator { opacity: 1; }
				
				/* shadcn animation keyframes */
				@keyframes progress {
					0% { transform: translateX(-100%); }
					50% { transform: translateX(0%); }
					100% { transform: translateX(100%); }
				}
				@keyframes slideInFromRight {
					from { transform: translateX(100%); opacity: 0; }
					to { transform: translateX(0); opacity: 1; }
				}
				@keyframes slideOutToRight {
					from { transform: translateX(0); opacity: 1; }
					to { transform: translateX(100%); opacity: 0; }
				}
				.animate-in { animation: slideInFromRight 0.2s ease-out; }
				.animate-out { animation: slideOutToRight 0.15s ease-in; }
				.slide-in-from-right { animation-name: slideInFromRight; }
				.slide-out-to-right { animation-name: slideOutToRight; }
				
				/* Toast animations - CSS-only auto-dismiss */
				@keyframes toast-enter {
					from { transform: translateX(100%); opacity: 0; }
					to { transform: translateX(0); opacity: 1; }
				}
				@keyframes toast-exit {
					from { transform: translateX(0); opacity: 1; }
					to { transform: translateX(100%); opacity: 0; visibility: hidden; }
				}
				[data-toast] {
					animation: toast-enter 0.2s ease-out;
				}
			`)),
		},
		Body: []g.Node{
			h.Class("min-h-screen bg-background text-foreground font-sans antialiased"),
			g.Group(body),
			// shadcn Portal containers
			h.Div(h.ID("modal-portal"), h.Class("contents")), // For dialogs, sheets, drawers
			// Toast container with server-rendered toasts
			h.Div(
				h.ID("toast-container"),
				h.Class("fixed bottom-4 right-4 z-50 flex flex-col gap-2 w-full max-w-sm pointer-events-none [&>*]:pointer-events-auto"),
				g.Attr("data-toast-container", ""),
				g.Group(toasts),
			),
			// Minimal JS for showToast (still needed for Plaid errors and other dynamic cases)
			// Note: Most toasts are now server-rendered, but external SDKs may call showToast
			h.Script(g.Raw(`
function showToast(options) {
    const container = document.querySelector('[data-toast-container]');
    if (!container) return;
    
    const id = 'toast-' + Date.now();
    const variant = options.variant || 'default';
    const duration = options.duration !== undefined ? options.duration : 5000;
    
    const variantClasses = {
        default: 'bg-card border-border text-card-foreground',
        success: 'bg-chart-2/10 border-chart-2/30 text-chart-2 backdrop-blur-sm',
        error: 'bg-destructive/10 border-destructive/30 text-destructive backdrop-blur-sm',
        warning: 'bg-ring/10 border-ring/30 text-ring backdrop-blur-sm',
        info: 'bg-primary/10 border-primary/30 text-primary backdrop-blur-sm'
    };
    
    const icons = {
        success: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>',
        error: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>',
        warning: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
        info: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>'
    };
    
    const toast = document.createElement('div');
    toast.id = id;
    toast.className = 'group pointer-events-auto relative flex w-full items-start gap-3 overflow-hidden rounded-lg border p-4 shadow-xl ' + variantClasses[variant];
    toast.setAttribute('role', 'alert');
    toast.setAttribute('data-toast', '');
    
    const icon = icons[variant] || '';
    const escapeHtml = (t) => { const d = document.createElement('div'); d.textContent = t; return d.innerHTML; };
    toast.innerHTML = (icon ? '<div class="shrink-0 mt-0.5">' + icon + '</div>' : '') +
        '<div class="flex-1 min-w-0 grid gap-1">' +
        (options.title ? '<div class="text-sm font-semibold leading-tight">' + escapeHtml(options.title) + '</div>' : '') +
        (options.description ? '<div class="text-sm opacity-90 leading-tight">' + escapeHtml(options.description) + '</div>' : '') +
        '</div>' +
        '<button type="button" class="absolute right-2 top-2 rounded-md p-1 opacity-0 transition-opacity hover:opacity-100 group-hover:opacity-100 focus:opacity-100 hover:bg-white/10" onclick="this.closest(\'[data-toast]\').remove()" aria-label="Dismiss">' +
        '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>' +
        '</button>';
    
    // Apply CSS animation for auto-dismiss
    if (duration > 0) {
        toast.style.animation = 'toast-enter 0.2s ease-out, toast-exit 0.2s ease-in ' + (duration/1000) + 's forwards';
        setTimeout(() => toast.remove(), duration + 200);
    }
    
    container.appendChild(toast);
}
			`)),
			// Scripts for dropdowns and utility functions
			h.Script(g.Raw(`
				// Legacy function for backward compatibility (now using CSS checkbox hack)
				function toggleSettingsSheet() {
					const toggle = document.getElementById('settings-sheet-toggle');
					if (toggle) toggle.checked = !toggle.checked;
				}

				function toggleDateDropdown() {
					const menu = document.getElementById('date-dropdown-menu');
					menu.classList.toggle('hidden');
					
					// Close on click outside
					if (!menu.classList.contains('hidden')) {
						setTimeout(() => {
							document.addEventListener('click', closeDateDropdown);
						}, 0);
					}
				}

				
				function closeDateDropdown(e) {
					const dropdown = document.getElementById('date-range-dropdown');
					if (dropdown && !dropdown.contains(e.target)) {
						const menu = document.getElementById('date-dropdown-menu');
						if (menu) menu.classList.add('hidden');
						document.removeEventListener('click', closeDateDropdown);
					}
				}

				// Localize dates: elements with data-date="YYYY-MM-DD" get formatted in user's locale
				function localizeDates() {
					document.querySelectorAll('[data-date]').forEach(el => {
						const d = el.dataset.date;
						if (!d || el.dataset.localized) return;
						const [y, m, day] = d.split('-').map(Number);
						el.textContent = new Date(y, m - 1, day).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
						el.dataset.localized = '1';
					});
				}
				localizeDates();
				document.body.addEventListener('htmx:afterSwap', localizeDates);

				// Combobox filter function (toggle handled by <details> element)
				function filterCombobox(id, query) {
					const container = document.getElementById(id);
					const items = container.querySelectorAll('[data-combobox-item]');
					const lowerQuery = query.toLowerCase();
					
					items.forEach(item => {
						const label = item.getAttribute('data-label').toLowerCase();
						const form = item.closest('form');
						if (label.includes(lowerQuery)) {
							form.style.display = '';
						} else {
							form.style.display = 'none';
						}
					});
				}
				
				// Legacy function for backward compatibility
				function toggleCombobox(id) {
					const details = document.getElementById(id);
					if (details && details.tagName === 'DETAILS') {
						details.open = !details.open;
					} else {
						// Fallback for old-style comboboxes
						const dropdown = document.getElementById(id)?.querySelector('[data-combobox-dropdown]');
						if (dropdown) dropdown.classList.toggle('hidden');
					}
				}
			`)),
		},
	})
}

// BaseWithToasts renders the base HTML layout with toasts
func BaseWithToasts(title string, toasts []g.Node, posthogDistinctID string, body ...g.Node) g.Node {
	return BaseWithMetaURLAndToasts(title, "Simple personal finance tracking", "", toasts, posthogDistinctID, body...)
}

// AuthLayout renders a centered auth page layout
func AuthLayout(title string, content ...g.Node) g.Node {
	return BaseWithMetaURLAndToasts(title, "Simple personal finance tracking", "", nil, "",
		h.Div(
			h.Class("min-h-screen flex items-center justify-center px-4"),
			h.Div(
				h.Class("w-full max-w-md"),
				// Logo
				h.Div(
					h.Class("text-center mb-8"),
					h.A(
						h.Href("/"),
						h.Class("inline-flex items-center gap-2"),
						h.Img(
							h.Src("/static/logo-pbw.png"),
							h.Alt("Probably"),
							h.Class("w-10 h-10 dark:invert"),
						),
						h.Span(
							h.Class("text-3xl font-bold tracking-tight text-foreground"),
							g.Text("probably"),
						),
					),
					h.P(
						h.Class("mt-2 text-muted-foreground"),
						g.Text("Simple personal finance"),
					),
				),
				// Card
				h.Div(
					h.Class("bg-card border border-border rounded-xl p-8 shadow-2xl"),
					g.Group(content),
				),
			),
		),
	)
}

// AppLayout renders the main app layout with sidebar
func AppLayout(title string, user string, posthogDistinctID string, content ...g.Node) g.Node {
	return AppLayoutWithToasts(title, user, posthogDistinctID, nil, content...)
}

// AppLayoutWithToasts renders the main app layout with sidebar and optional toasts
func AppLayoutWithToasts(title string, user string, posthogDistinctID string, toasts []g.Node, content ...g.Node) g.Node {
	return AppLayoutFull(title, user, "", posthogDistinctID, toasts, content...)
}

// AppLayoutFull renders the main app layout with all options including path for nav highlighting
func AppLayoutFull(title string, user string, currentPath string, posthogDistinctID string, toasts []g.Node, content ...g.Node) g.Node {
	return BaseWithToasts(title, toasts, posthogDistinctID,
		h.Div(
			h.Class("flex h-screen min-h-0"),
			// Desktop Sidebar (hidden on mobile)
			SidebarWithPath(user, currentPath),
			// Main content area
			h.Div(
				// min-h-0 is required so flex-1 + overflow-auto can shrink; otherwise main can collapse to 0 height (blank page)
				h.Class("flex-1 flex flex-col overflow-hidden min-h-0"),
				// Main content with bottom padding on mobile for floating tab bar
				h.Main(
					h.Class("flex-1 min-h-0 overflow-auto pb-24 md:pb-0"),
					h.Div(
						h.Class("max-w-6xl mx-auto p-4 md:p-6 min-h-0"),
						g.Group(content),
					),
				),
				// Mobile bottom tab bar
				MobileTabBarWithPath("", currentPath),
			),
		),
		// Settings sheet for mobile
		MobileSettingsSheet(user),
	)
}

// SidebarWithPath renders the navigation sidebar with active state based on path
func SidebarWithPath(user, currentPath string) g.Node {
	return h.Aside(
		h.Class("hidden md:flex w-64 bg-sidebar border-r border-sidebar-border flex-col"),
		// Logo
		h.Div(
			h.Class("p-6 border-b border-sidebar-border"),
			h.A(
				h.Href("/pulse"),
				h.Class("flex items-center gap-2"),
				h.Img(
					h.Src("/static/logo-pbw.png"),
					h.Alt("Probably"),
					h.Class("w-7 h-7 dark:invert"),
				),
				h.Span(
					h.Class("text-xl font-bold tracking-tight text-sidebar-foreground"),
					g.Text("probably"),
				),
			),
		),
		// Navigation
		h.Nav(
			h.ID("desktop-nav"),
			h.Class("flex-1 p-4 space-y-1"),
			// Group 1: AI & Insights
			NavLinkWithPath("/pulse", "Pulse", currentPath, IconPulse()),
			NavLinkWithPath("/intelligence", "Intelligence", currentPath, IconSparkles()),
			NavLinkWithPath("/chat", "Chat", currentPath, IconSparkles()),
			// Divider
			h.Div(h.Class("h-px bg-sidebar-border my-3")),
			// Group 2: Data & Transactions
			NavLinkWithPath("/statements", "Statements", currentPath, IconBarChart()),
			NavLinkWithPath("/patterns", "Patterns", currentPath, IconRecurring()),
			NavLinkWithPath("/transactions", "Transactions", currentPath, IconList()),
			NavLinkWithPath("/accounts", "Accounts", currentPath, IconWallet()),
			// Divider
			h.Div(h.Class("h-px bg-sidebar-border my-3")),
			// Group 3: Settings
			NavLinkWithPath("/settings", "Settings", currentPath, IconSettings()),
		),
		// User section - links to profile settings
		h.A(
			h.Href("/settings/profile"),
			h.Class("flex items-center gap-3 p-4 border-t border-sidebar-border hover:bg-sidebar-accent transition-colors"),
			h.Img(
				h.Src(fmt.Sprintf("https://www.gravatar.com/avatar/%x?d=mp", sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(user)))))),
				h.Alt("User Avatar"),
				h.Class("w-8 h-8 rounded-full bg-primary"),
			),
			h.Div(
				h.Class("flex-1 min-w-0"),
				h.P(h.Class("text-sm font-medium truncate"), g.Text(user)),
			),
		),
	)
}

// MobileTabBarWithPath renders an iOS-style floating bottom tab bar with active state based on path
func MobileTabBarWithPath(activeTab, currentPath string) g.Node {
	return h.Nav(
		h.ID("mobile-tab-bar"),
		h.Class("md:hidden fixed bottom-0 left-0 right-0"),
		h.Div(
			h.Class("flex backdrop-blur-sm rounded-4xl mx-8 border-b border-border/70 shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"),
			h.Div(
				h.Class("flex flex-1 items-center justify-around h-16"),
				MobileTabWithPath("/pulse", "Pulse", currentPath, IconPulse()),
				MobileTabWithPath("/chat", "Chat", currentPath, IconSparkles()),
				MobileTabWithPath("/transactions", "Transactions", currentPath, IconList()),
				MobileTabWithPath("/accounts", "Accounts", currentPath, IconWallet()),
				// Settings button as label for the settings sheet checkbox
				h.Label(
					h.For("settings-sheet-toggle"),
					h.ID("tab-settings"),
					h.Class("flex flex-col items-center justify-center gap-1 px-4 py-3 min-w-8 transition-colors text-foreground cursor-pointer"),
					IconSettings(),
				),
			),
		),
	)
}

// MobileTabWithPath renders a single tab with active state based on current path
func MobileTabWithPath(href, label, currentPath string, icon g.Node) g.Node {
	// Check if this tab is active
	isActive := currentPath == href || (href != "/" && len(currentPath) > 0 && strings.HasPrefix(currentPath, href))

	baseClass := "flex flex-col items-center justify-center gap-1 px-4 py-3 min-w-8 transition-colors"
	activeClass := " text-foreground"
	inactiveClass := " text-muted-foreground"

	class := baseClass
	if isActive {
		class += activeClass
	} else {
		class += inactiveClass
	}

	return h.A(
		h.Href(href),
		h.ID("tab-"+label),
		h.Class(class),
		icon,
	)
}

// MobileSettingsSheet renders a bottom sheet with settings options using checkbox hack
func MobileSettingsSheet(user string) g.Node {
	return h.Div(
		h.ID("settings-sheet"),
		h.Class("md:hidden contents"),
		// Hidden checkbox for state management
		h.Input(
			h.Type("checkbox"),
			h.ID("settings-sheet-toggle"),
			h.Class("peer sr-only"),
			g.Attr("aria-hidden", "true"),
		),
		// Container that shows/hides based on checkbox
		h.Div(
			h.Class("fixed inset-0 z-50 pointer-events-none peer-checked:pointer-events-auto"),
			// Backdrop - fades in when checkbox is checked
			h.Label(
				h.For("settings-sheet-toggle"),
				h.Class("absolute inset-0 bg-black/60 opacity-0 peer-checked:opacity-100 transition-opacity duration-300 cursor-pointer"),
			),
			// Sheet - slides up when checkbox is checked
			h.Div(
				h.ID("settings-sheet-content"),
				h.Class("absolute bottom-0 left-0 right-0 bg-card rounded-t-3xl transform translate-y-full peer-checked:translate-y-0 transition-transform duration-300 ease-out max-h-[85vh] overflow-hidden safe-area-x"),
				// Handle (clicking closes the sheet)
				h.Label(
					h.For("settings-sheet-toggle"),
					h.Class("flex justify-center py-3 cursor-pointer"),
					h.Div(h.Class("w-10 h-1 bg-border rounded-full")),
				),
				// Content
				h.Div(
					h.Class("px-4 pb-12 overflow-y-auto max-h-[calc(85vh-40px)] safe-area-bottom"),
					// User info
					h.Div(
						h.Class("flex items-center gap-3 p-4 mb-4 bg-secondary/50 rounded-xl"),
						h.Img(
							h.Src(fmt.Sprintf("https://www.gravatar.com/avatar/%x?d=mp", sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(user)))))),
							h.Alt("User Avatar"),
							h.Class("w-10 h-10 rounded-full bg-primary"),
						),
						h.Div(
							h.Class("flex-1 min-w-0"),
							h.P(h.Class("font-medium truncate text-sm"), g.Text(user)),
						),
					),
					// Settings links
					h.Div(
						h.Class("space-y-1"),
						// Group 1: Profile, My Life
						SettingsSheetLink("/settings/profile", "Profile", IconUser()),
						SettingsSheetLink("/settings/preferences", "Preferences", IconSettings()),
						SettingsSheetLink("/settings/my-life", "My Life", IconHeart()),
						h.Div(h.Class("h-px bg-border my-3")),
						// Group 2: Tags, Banks, Ledger
						SettingsSheetLink("/tags", "Tags", IconTag()),
						SettingsSheetLink("/settings/banks", "Connected Banks", IconBank()),
						SettingsSheetLink("/settings/ledger", "Ledger", IconDatabase()),
						h.Div(h.Class("h-px bg-border my-3")),
						// Group 3: Billing, Backup, Danger
						SettingsSheetLink("/settings/billing", "Billing", IconWallet()),
						SettingsSheetLink("/settings/backup", "Data Backup", IconDownload()),
						SettingsSheetLink("/settings/danger", "Danger Zone", IconTrash()),
						h.Div(h.Class("h-px bg-border my-3")),
						// Logout button
						h.Form(
							h.Method("POST"),
							h.Action("/auth/logout"),
							h.Button(
								h.Type("submit"),
								h.Class("w-full flex items-center gap-4 px-4 py-3.5 rounded-xl text-muted-foreground hover:text-foreground active:bg-accent transition-colors"),
								IconLogout(),
								h.Span(h.Class("text-base"), g.Text("Log out")),
							),
						),
						// Bottom spacer for iOS safe area
						h.Div(h.Class("h-8")),
					),
				),
			),
		),
	)
}

// SettingsSheetLink renders a link in the settings sheet
func SettingsSheetLink(href, label string, icon g.Node) g.Node {
	return h.A(
		h.Href(href),
		h.Class("flex items-center gap-4 px-4 py-3.5 rounded-xl text-foreground hover:text-foreground active:bg-accent transition-colors"),
		icon,
		h.Span(h.Class("text-base"), g.Text(label)),
		h.Span(h.Class("ml-auto text-muted-foreground"), IconChevronRight()),
	)
}

// NavLinkWithPath renders a navigation link with active state based on current path
func NavLinkWithPath(href, text, currentPath string, icon g.Node) g.Node {
	// Check if this link is active
	isActive := currentPath == href || (href != "/" && len(currentPath) > 0 && strings.HasPrefix(currentPath, href))

	baseClass := "flex items-center gap-3 px-3 py-2 rounded-lg transition-colors"
	activeClass := " text-foreground bg-accent"
	inactiveClass := " text-muted-foreground hover:text-foreground hover:bg-accent"

	class := baseClass
	if isActive {
		class += activeClass
	} else {
		class += inactiveClass
	}

	return h.A(
		h.Href(href),
		h.Class(class),
		icon,
		h.Span(g.Text(text)),
	)
}

// SettingsLayout renders the settings layout with sub-navigation
func SettingsLayout(title string, user string, currentSection string, posthogDistinctID string, content ...g.Node) g.Node {
	return SettingsLayoutWithToasts(title, user, currentSection, posthogDistinctID, nil, content...)
}

// SettingsLayoutWithToasts renders the settings layout with sub-navigation and optional toasts
func SettingsLayoutWithToasts(title string, user string, currentSection string, posthogDistinctID string, toasts []g.Node, content ...g.Node) g.Node {
	// Settings pages have /settings path prefix
	currentPath := "/settings"
	if currentSection != "" && currentSection != "settings" {
		currentPath = "/settings/" + currentSection
	}

	return BaseWithToasts(title, toasts, posthogDistinctID,
		h.Div(
			h.Class("flex h-screen"),
			// Main sidebar (hidden on mobile) - highlight settings
			SidebarWithPath(user, "/settings"),
			// Settings sub-navigation and content
			h.Div(
				h.Class("flex flex-1 flex-col md:flex-row overflow-hidden"),
				// Desktop settings sub-nav (hidden on mobile)
				SettingsSubNav(currentSection),
				// Main content with bottom padding on mobile for tab bar
				h.Main(
					h.Class("flex-1 overflow-auto pb-24 md:pb-0"),
					h.Div(
						h.Class("max-w-4xl mx-auto p-4 md:p-6"),
						g.Group(content),
					),
				),
				// Mobile bottom tab bar
				MobileTabBarWithPath("settings", currentPath),
			),
		),
		// Settings sheet for mobile
		MobileSettingsSheet(user),
	)
}

// SettingsSubNav renders the settings sub-navigation sidebar
func SettingsSubNav(currentSection string) g.Node {
	type navItem struct {
		href string
		text string
		icon g.Node
	}

	// Helper to build a nav link
	buildNavLink := func(s navItem) g.Node {
		isActive := false
		if currentSection != "" {
			if s.href == "/settings/"+currentSection {
				isActive = true
			} else if s.href == "/"+currentSection {
				if currentSection == "tags" {
					isActive = true
				}
			}
		}

		activeClass := ""
		textClass := "text-muted-foreground"
		if isActive {
			activeClass = "bg-accent"
			textClass = "text-foreground"
		}
		return h.A(
			h.Href(s.href),
			h.Class("flex items-center gap-3 px-3 py-2 rounded-lg hover:text-foreground hover:bg-accent transition-colors "+activeClass),
			s.icon,
			h.Span(h.Class(textClass), g.Text(s.text)),
		)
	}

	// Group 1: Profile, Security, My Life
	group1 := []navItem{
		{"/settings/profile", "Profile", IconUser()},
		{"/settings/preferences", "Preferences", IconSettings()},
		{"/settings/security", "Security", IconKey()},
		{"/settings/my-life", "My Life", IconHeart()},
	}

	// Group 2: Tags, Banks, Ledger
	group2 := []navItem{
		{"/tags", "Tags", IconTag()},
		{"/settings/banks", "Connected Banks", IconBank()},
		{"/settings/ledger", "Ledger", IconDatabase()},
	}

	// Group 3: Billing, Data Backup, Danger Zone
	group3 := []navItem{
		{"/settings/billing", "Billing", IconWallet()},
		{"/settings/backup", "Data Backup", IconDownload()},
		{"/settings/danger", "Danger Zone", IconTrash()},
	}

	return h.Aside(
		h.Class("hidden md:flex w-64 bg-sidebar/50 border-r border-sidebar-border flex-col"),
		h.Div(
			h.Class("p-6 border-b border-sidebar-border"),
			h.H2(h.Class("text-xl font-bold tracking-tight text-sidebar-foreground"), g.Text("Settings")),
		),
		h.Nav(
			h.Class("flex-1 p-4"),
			// Group 1: Account
			h.Div(
				h.Class("space-y-1"),
				g.Group(g.Map(group1, buildNavLink)),
			),
			// Divider
			h.Div(h.Class("h-px bg-sidebar-border my-3")),
			// Group 2: Data
			h.Div(
				h.Class("space-y-1"),
				g.Group(g.Map(group2, buildNavLink)),
			),
			// Divider
			h.Div(h.Class("h-px bg-sidebar-border my-3")),
			// Group 3: System
			h.Div(
				h.Class("space-y-1"),
				g.Group(g.Map(group3, buildNavLink)),
			),
		),
		// Logout at bottom of settings nav
		h.Form(
			h.Method("POST"),
			h.Action("/auth/logout"),
			h.Class("border-t border-sidebar-border"),
			h.Button(
				h.Type("submit"),
				h.Class("flex items-center gap-3 p-4 text-muted-foreground hover:text-foreground hover:bg-accent transition-colors w-full"),
				IconLogout(),
				h.Span(g.Text("Log out")),
			),
		),
	)
}
