package shadcn

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Breadcrumb
type BreadcrumbProps struct {
	Class string
}

func Breadcrumb(props BreadcrumbProps, children ...g.Node) g.Node {
	return h.Nav(
		AriaLabel("Breadcrumb"),
		h.Class(props.Class),
		g.Group(children),
	)
}

func BreadcrumbList(children ...g.Node) g.Node {
	return h.Ol(
		h.Class("flex flex-wrap items-center gap-1.5 break-words text-sm text-muted-foreground sm:gap-2.5"),
		g.Group(children),
	)
}

func BreadcrumbItem(children ...g.Node) g.Node {
	return h.Li(
		h.Class("inline-flex items-center gap-1.5"),
		g.Group(children),
	)
}

func BreadcrumbLink(href string, children ...g.Node) g.Node {
	return h.A(
		h.Href(href),
		h.Class("transition-colors hover:text-foreground"),
		g.Group(children),
	)
}

func BreadcrumbPage(children ...g.Node) g.Node {
	return h.Span(
		h.Class("font-normal text-foreground"),
		Role("link"),
		AriaDisabled(true),
		g.Attr("aria-current", "page"),
		g.Group(children),
	)
}

func BreadcrumbSeparator() g.Node {
	return h.Li(
		h.Class("text-muted-foreground"),
		Role("presentation"),
		AriaHidden(true),
		IconChevronRight(),
	)
}

func BreadcrumbEllipsis() g.Node {
	return h.Span(
		h.Class("flex h-9 w-9 items-center justify-center"),
		Role("presentation"),
		AriaHidden(true),
		IconMoreHorizontal(),
		VisuallyHidden(g.Text("More")),
	)
}

type BreadcrumbItemData struct {
	Label string
	Href  string
}

func SimpleBreadcrumb(items []BreadcrumbItemData) g.Node {
	nodes := make([]g.Node, 0, len(items)*2-1)
	for i, item := range items {
		if i > 0 {
			nodes = append(nodes, BreadcrumbSeparator())
		}
		if i == len(items)-1 {
			// Last item is the current page
			nodes = append(nodes, BreadcrumbItem(BreadcrumbPage(g.Text(item.Label))))
		} else {
			nodes = append(nodes, BreadcrumbItem(BreadcrumbLink(item.Href, g.Text(item.Label))))
		}
	}

	return Breadcrumb(BreadcrumbProps{},
		BreadcrumbList(nodes...),
	)
}

// Tabs
type TabsProps struct {
	ID           string
	DefaultValue string
	Class        string
}

func Tabs(props TabsProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "tabs"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("w-full", props.Class)),
		g.Attr("data-tabs", props.DefaultValue),
		g.Group(children),
	)
}

func TabsList(children ...g.Node) g.Node {
	return h.Div(
		h.Class("inline-flex h-9 items-center justify-center rounded-lg bg-muted p-1 text-muted-foreground"),
		Role("tablist"),
		g.Group(children),
	)
}

func TabsTrigger(tabsID, value string, active bool, children ...g.Node) g.Node {
	activeClass := "hover:bg-muted/50"
	if active {
		activeClass = "bg-card text-card-foreground shadow"
	}
	classes := Cn(
		"inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1 text-sm font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50",
		activeClass,
	)

	return h.Button(
		h.Type("button"),
		h.Class(classes),
		Role("tab"),
		AriaSelected(active),
		AriaControls(tabsID+"-"+value),
		g.Attr("data-tab-trigger", value),
		g.Attr("onclick", fmt.Sprintf("switchTab('%s', '%s')", tabsID, value)),
		g.Group(children),
	)
}

func TabsContent(tabsID, value string, active bool, children ...g.Node) g.Node {
	hiddenClass := ""
	if !active {
		hiddenClass = "hidden"
	}

	return h.Div(
		h.ID(tabsID+"-"+value),
		h.Class(Cn("mt-2 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2", hiddenClass)),
		Role("tabpanel"),
		AriaLabelledBy(tabsID+"-trigger-"+value),
		g.Attr("data-tab-content", value),
		TabIndex(0),
		g.Group(children),
	)
}

// TabsScript returns the JavaScript for tabs functionality
func TabsScript() g.Node {
	return g.Raw(`<script>
function switchTab(tabsId, value) {
    const container = document.getElementById(tabsId);
    if (!container) return;
    
    // Update triggers
    container.querySelectorAll('[data-tab-trigger]').forEach(trigger => {
        const isActive = trigger.getAttribute('data-tab-trigger') === value;
        trigger.setAttribute('aria-selected', isActive);
        if (isActive) {
            trigger.classList.add('bg-card', 'text-card-foreground', 'shadow');
            trigger.classList.remove('hover:bg-muted/50');
        } else {
            trigger.classList.remove('bg-card', 'text-card-foreground', 'shadow');
            trigger.classList.add('hover:bg-muted/50');
        }
    });
    
    // Update content panels
    container.querySelectorAll('[data-tab-content]').forEach(content => {
        const isActive = content.getAttribute('data-tab-content') === value;
        content.classList.toggle('hidden', !isActive);
    });
    
    container.setAttribute('data-tabs', value);
}

</script>`)
}

// HTMXTabs - Alternative tabs using HTMX for server-side state
func HTMXTabsTrigger(tabsID, value string, active bool, url string, children ...g.Node) g.Node {
	activeClass := "hover:bg-muted/50"
	if active {
		activeClass = "bg-card text-card-foreground shadow"
	}
	classes := Cn(
		"inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1 text-sm font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50",
		activeClass,
	)

	return h.Button(
		h.Type("button"),
		h.Class(classes),
		Role("tab"),
		AriaSelected(active),
		HxGet(url),
		HxTarget("#"+tabsID+"-content"),
		HxSwap("innerHTML"),
		HxPushURL("true"),
		g.Group(children),
	)
}

// Pagination
type PaginationProps struct {
	CurrentPage int
	TotalPages  int
	BaseURL     string // URL pattern with %d for page number
	Class       string
}

func Pagination(props PaginationProps, children ...g.Node) g.Node {
	return h.Nav(
		h.Class(Cn("mx-auto flex w-full justify-center", props.Class)),
		Role("navigation"),
		AriaLabel("pagination"),
		g.Group(children),
	)
}

func PaginationContent(children ...g.Node) g.Node {
	return h.Ul(
		h.Class("flex flex-row items-center gap-1"),
		g.Group(children),
	)
}

func PaginationItem(children ...g.Node) g.Node {
	return h.Li(g.Group(children))
}

func PaginationLink(href string, page int, active bool) g.Node {
	activeClass := "hover:bg-muted hover:text-foreground text-muted-foreground"
	if active {
		activeClass = "border border-border bg-muted text-foreground"
	}
	classes := Cn(
		"inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors h-9 w-9",
		activeClass,
	)

	if active {
		return h.Span(
			h.Class(classes),
			AriaSelected(true),
			g.Textf("%d", page),
		)
	}

	return h.A(
		h.Href(href),
		h.Class(classes),
		g.Textf("%d", page),
	)
}

func PaginationPrevious(href string, disabled bool) g.Node {
	disabledClass := "hover:bg-muted hover:text-foreground text-muted-foreground"
	if disabled {
		disabledClass = "pointer-events-none opacity-50 text-muted-foreground"
	}
	classes := Cn(
		"inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors h-9 px-3 gap-1",
		disabledClass,
	)

	if disabled {
		return h.Span(
			h.Class(classes),
			IconChevronLeft(),
			h.Span(g.Text("Previous")),
		)
	}

	return h.A(
		h.Href(href),
		h.Class(classes),
		IconChevronLeft(),
		h.Span(g.Text("Previous")),
	)
}

func PaginationNext(href string, disabled bool) g.Node {
	disabledClass := "hover:bg-muted hover:text-foreground text-muted-foreground"
	if disabled {
		disabledClass = "pointer-events-none opacity-50 text-muted-foreground"
	}
	classes := Cn(
		"inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors h-9 px-3 gap-1",
		disabledClass,
	)

	if disabled {
		return h.Span(
			h.Class(classes),
			h.Span(g.Text("Next")),
			IconChevronRight(),
		)
	}

	return h.A(
		h.Href(href),
		h.Class(classes),
		h.Span(g.Text("Next")),
		IconChevronRight(),
	)
}

func PaginationEllipsis() g.Node {
	return h.Span(
		h.Class("flex h-9 w-9 items-center justify-center text-muted-foreground"),
		AriaHidden(true),
		IconMoreHorizontal(),
		VisuallyHidden(g.Text("More pages")),
	)
}

func SimplePagination(current, total int, baseURL string) g.Node {
	if total <= 1 {
		return nil
	}

	items := make([]g.Node, 0)

	// Previous button
	prevDisabled := current <= 1
	prevURL := fmt.Sprintf(baseURL, current-1)
	items = append(items, PaginationItem(PaginationPrevious(prevURL, prevDisabled)))

	// Page numbers with ellipsis logic
	showPages := calculatePaginationPages(current, total)
	lastShown := 0
	for _, page := range showPages {
		if lastShown > 0 && page > lastShown+1 {
			items = append(items, PaginationItem(PaginationEllipsis()))
		}
		pageURL := fmt.Sprintf(baseURL, page)
		items = append(items, PaginationItem(PaginationLink(pageURL, page, page == current)))
		lastShown = page
	}

	// Next button
	nextDisabled := current >= total
	nextURL := fmt.Sprintf(baseURL, current+1)
	items = append(items, PaginationItem(PaginationNext(nextURL, nextDisabled)))

	return Pagination(PaginationProps{},
		PaginationContent(items...),
	)
}

// calculatePaginationPages determines which page numbers to show
func calculatePaginationPages(current, total int) []int {
	if total <= 7 {
		pages := make([]int, total)
		for i := 0; i < total; i++ {
			pages[i] = i + 1
		}
		return pages
	}

	pages := []int{1}

	start := current - 1
	end := current + 1

	if start <= 2 {
		start = 2
		end = 4
	}
	if end >= total-1 {
		end = total - 1
		start = total - 3
	}

	for i := start; i <= end; i++ {
		pages = append(pages, i)
	}

	pages = append(pages, total)
	return pages
}

// Sidebar
type SidebarProps struct {
	ID        string
	Collapsed bool
	Class     string
}

func Sidebar(props SidebarProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "sidebar"
	}

	width := "w-64"
	if props.Collapsed {
		width = "w-16"
	}

	return h.Aside(
		h.ID(id),
		h.Class(Cn(
			"flex flex-col bg-card border-r border-border transition-all duration-300",
			width,
			props.Class,
		)),
		g.Attr("data-collapsed", fmt.Sprintf("%t", props.Collapsed)),
		g.Group(children),
	)
}

func SidebarHeader(children ...g.Node) g.Node {
	return h.Div(
		h.Class("p-4 border-b border-border"),
		g.Group(children),
	)
}

func SidebarContent(children ...g.Node) g.Node {
	return h.Div(
		h.Class("flex-1 overflow-auto p-4"),
		g.Group(children),
	)
}

func SidebarFooter(children ...g.Node) g.Node {
	return h.Div(
		h.Class("p-4 border-t border-border"),
		g.Group(children),
	)
}

func SidebarGroup(children ...g.Node) g.Node {
	return h.Div(
		h.Class("mb-4"),
		g.Group(children),
	)
}

func SidebarGroupLabel(label string) g.Node {
	return h.Div(
		h.Class("px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider"),
		g.Text(label),
	)
}

func SidebarItem(href string, icon g.Node, label string, active bool) g.Node {
	activeClass := "text-muted-foreground hover:text-foreground hover:bg-muted"
	if active {
		activeClass = "bg-muted text-foreground"
	}
	classes := Cn(
		"flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
		activeClass,
	)

	return h.A(
		h.Href(href),
		h.Class(classes),
		g.If(icon != nil, h.Span(h.Class("shrink-0"), icon)),
		h.Span(h.Class("truncate"), g.Text(label)),
	)
}

func SidebarCollapseTrigger(sidebarID string) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"),
		g.Attr("onclick", "toggleSidebar('"+sidebarID+"')"),
		AriaLabel("Toggle sidebar"),
		IconChevronLeft(),
	)
}

// SidebarScript returns the JavaScript for sidebar functionality
func SidebarScript() g.Node {
	return g.Raw(`<script>
function toggleSidebar(id) {
    const sidebar = document.getElementById(id);
    if (!sidebar) return;
    
    const isCollapsed = sidebar.getAttribute('data-collapsed') === 'true';
    
    if (isCollapsed) {
        sidebar.classList.remove('w-16');
        sidebar.classList.add('w-64');
        sidebar.setAttribute('data-collapsed', 'false');
    } else {
        sidebar.classList.remove('w-64');
        sidebar.classList.add('w-16');
        sidebar.setAttribute('data-collapsed', 'true');
    }
}

</script>`)
}
