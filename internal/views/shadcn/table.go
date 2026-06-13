package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Table
type TableProps struct {
	Class string
}

func Table(props TableProps, children ...g.Node) g.Node {
	return h.Div(
		h.Class("relative w-full overflow-auto"),
		h.Table(
			h.Class(Cn("w-full caption-bottom text-sm", props.Class)),
			g.Group(children),
		),
	)
}

func TableHeader(children ...g.Node) g.Node {
	return h.THead(
		h.Class("[&_tr]:border-b border-border"),
		g.Group(children),
	)
}

func TableBody(children ...g.Node) g.Node {
	return h.TBody(
		h.Class("[&_tr:last-child]:border-0"),
		g.Group(children),
	)
}

func TableFooter(children ...g.Node) g.Node {
	return h.TFoot(
		h.Class("border-t border-border bg-card/50 font-medium [&>tr]:last:border-b-0"),
		g.Group(children),
	)
}

func TableRow(children ...g.Node) g.Node {
	return h.Tr(
		h.Class("border-b border-border transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted"),
		g.Group(children),
	)
}

func TableRowClickable(href string, children ...g.Node) g.Node {
	return h.Tr(
		h.Class("border-b border-border transition-colors hover:bg-muted/50 cursor-pointer"),
		g.Attr("onclick", "window.location.href='"+href+"'"),
		g.Group(children),
	)
}

func TableRowSelected(children ...g.Node) g.Node {
	return h.Tr(
		h.Class("border-b border-border transition-colors bg-muted"),
		DataState("selected"),
		g.Group(children),
	)
}

func TableHead(children ...g.Node) g.Node {
	return h.Th(
		h.Class("h-10 px-4 text-left align-middle font-medium text-muted-foreground [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px]"),
		g.Group(children),
	)
}

func TableHeadSortable(label string, sortKey string, currentSort string, currentDir string, sortURL string) g.Node {
	isSorted := sortKey == currentSort
	isAsc := currentDir == "asc"

	nextDir := "asc"
	if isSorted && isAsc {
		nextDir = "desc"
	}

	ascClass := "text-muted-foreground"
	if isSorted && isAsc {
		ascClass = "text-indigo-400"
	}
	descClass := "text-muted-foreground"
	if isSorted && !isAsc {
		descClass = "text-indigo-400"
	}

	return h.Th(
		h.Class("h-10 px-4 text-left align-middle font-medium text-muted-foreground"),
		h.Button(
			h.Type("button"),
			h.Class("inline-flex items-center gap-1 hover:text-foreground transition-colors"),
			HxGet(sortURL+"?sort="+sortKey+"&dir="+nextDir),
			HxTarget("closest table"),
			HxSwap("outerHTML"),
			g.Text(label),
			h.Span(
				h.Class("inline-flex flex-col -space-y-1.5"),
				h.Span(
					h.Class(Cn("text-[10px]", ascClass)),
					g.Text("▲"),
				),
				h.Span(
					h.Class(Cn("text-[10px]", descClass)),
					g.Text("▼"),
				),
			),
		),
	)
}

func TableCell(children ...g.Node) g.Node {
	return h.Td(
		h.Class("p-4 align-middle [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px] text-foreground"),
		g.Group(children),
	)
}

func TableCellNumeric(children ...g.Node) g.Node {
	return h.Td(
		h.Class("p-4 align-middle text-right tabular-nums text-foreground"),
		g.Group(children),
	)
}

func TableCellActions(children ...g.Node) g.Node {
	return h.Td(
		h.Class("p-4 align-middle text-right"),
		h.Div(
			h.Class("flex items-center justify-end gap-2"),
			g.Group(children),
		),
	)
}

func TableCaption(children ...g.Node) g.Node {
	return h.Caption(
		h.Class("mt-4 text-sm text-muted-foreground"),
		g.Group(children),
	)
}

// Table Utilities
func TableEmpty(colSpan int, message string) g.Node {
	return h.Tr(
		h.Td(
			g.Attr("colspan", string(rune('0'+colSpan))),
			h.Class("h-24 text-center text-muted-foreground"),
			g.Text(message),
		),
	)
}

func TableLoading(colSpan int) g.Node {
	return h.Tr(
		h.Td(
			g.Attr("colspan", string(rune('0'+colSpan))),
			h.Class("h-24 text-center"),
			h.Div(
				h.Class("flex items-center justify-center gap-2 text-muted-foreground"),
				IconLoader(),
				g.Text("Loading..."),
			),
		),
	)
}

func TableCheckboxHeader(selectAllURL string) g.Node {
	return h.Th(
		h.Class("w-12 px-4"),
		Checkbox(CheckboxProps{
			ID:    "select-all",
			Class: "translate-y-[2px]",
		}),
	)
}

func TableCheckboxCell(id string, checked bool) g.Node {
	return h.Td(
		h.Class("w-12 px-4"),
		Checkbox(CheckboxProps{
			ID:      "select-" + id,
			Value:   id,
			Checked: checked,
			Class:   "translate-y-[2px]",
		}),
	)
}

// Simple Table Helper
// SimpleTableColumn defines a column in a SimpleTable
type SimpleTableColumn struct {
	Header   string
	Key      string
	Class    string
	Sortable bool
	Numeric  bool
}

type SimpleTableProps struct {
	Columns     []SimpleTableColumn
	EmptyText   string
	Class       string
	CurrentSort string
	SortDir     string
	SortURL     string
}

func SimpleTable(props SimpleTableProps, rows ...g.Node) g.Node {
	headers := make([]g.Node, len(props.Columns))
	for i, col := range props.Columns {
		if col.Sortable && props.SortURL != "" {
			headers[i] = TableHeadSortable(col.Header, col.Key, props.CurrentSort, props.SortDir, props.SortURL)
		} else {
			headers[i] = TableHead(g.Text(col.Header))
		}
	}

	return Table(TableProps{Class: props.Class},
		TableHeader(
			TableRow(g.Group(headers)),
		),
		TableBody(
			g.If(len(rows) == 0,
				TableEmpty(len(props.Columns), props.EmptyText),
			),
			g.Group(rows),
		),
	)
}

// Responsive Table Wrapper
// ResponsiveTable wraps a table for horizontal scrolling on small screens
func ResponsiveTable(children ...g.Node) g.Node {
	return h.Div(
		h.Class("relative w-full overflow-auto"),
		g.Group(children),
	)
}

func TableContainer(title string, description string, actions []g.Node, table g.Node) g.Node {
	return Card(CardProps{},
		CardHeaderActions(title, description, actions...),
		h.Div(
			h.Class("px-6 pb-6"),
			table,
		),
	)
}

// Data Table (HTMX-powered with sorting, filtering, pagination)
// DataTableColumn defines a column in a DataTable
type DataTableColumn struct {
	Key        string
	Header     string
	Sortable   bool
	Searchable bool
	Hidden     bool
	Class      string
	Render     func(value interface{}) g.Node // Custom renderer
}

type DataTableProps struct {
	ID             string
	Columns        []DataTableColumn
	BaseURL        string // URL for HTMX requests
	CurrentPage    int
	PageSize       int
	TotalItems     int
	TotalPages     int
	CurrentSort    string
	SortDirection  string
	SearchQuery    string
	ShowSearch     bool
	ShowPagination bool
	ShowPerPage    bool
	EmptyText      string
	Class          string
}

func DataTable(props DataTableProps, rows ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "datatable"
	}

	pageSize := props.PageSize
	if pageSize == 0 {
		pageSize = 10
	}

	emptyText := props.EmptyText
	if emptyText == "" {
		emptyText = "No results found."
	}

	// Build table headers
	headers := make([]g.Node, 0, len(props.Columns))
	for _, col := range props.Columns {
		if col.Hidden {
			continue
		}
		if col.Sortable {
			headers = append(headers, dataTableSortableHeader(id, props.BaseURL, col, props.CurrentSort, props.SortDirection, props.SearchQuery, pageSize))
		} else {
			headers = append(headers, TableHead(g.Text(col.Header)))
		}
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("space-y-4", props.Class)),
		g.Attr("data-datatable", id),

		// Toolbar (search, filters, per-page)
		g.If(props.ShowSearch || props.ShowPerPage,
			dataTableToolbar(id, props),
		),

		// Table
		Table(TableProps{},
			TableHeader(
				TableRow(g.Group(headers)),
			),
			TableBody(
				g.If(len(rows) == 0,
					TableEmpty(len(props.Columns), emptyText),
				),
				g.Group(rows),
			),
		),

		// Pagination
		g.If(props.ShowPagination && props.TotalPages > 1,
			dataTablePagination(id, props),
		),
	)
}

func dataTableSortableHeader(tableID, baseURL string, col DataTableColumn, currentSort, sortDir, search string, pageSize int) g.Node {
	isSorted := col.Key == currentSort
	isAsc := sortDir == "asc"

	nextDir := "asc"
	if isSorted && isAsc {
		nextDir = "desc"
	}

	// Build URL with all current params
	url := baseURL + "?sort=" + col.Key + "&dir=" + nextDir + "&page=1&per_page=" + string(rune('0'+pageSize))
	if search != "" {
		url += "&search=" + search
	}

	dtAscClass := "text-muted-foreground"
	if isSorted && isAsc {
		dtAscClass = "text-indigo-400"
	}
	dtDescClass := "text-muted-foreground"
	if isSorted && !isAsc {
		dtDescClass = "text-indigo-400"
	}

	return h.Th(
		h.Class(Cn("h-10 px-4 text-left align-middle font-medium text-muted-foreground", col.Class)),
		h.Button(
			h.Type("button"),
			h.Class("inline-flex items-center gap-1 hover:text-foreground transition-colors"),
			HxGet(url),
			HxTarget("#"+tableID),
			HxSwap("outerHTML"),
			HxPushURL("true"),
			g.Text(col.Header),
			h.Span(
				h.Class("inline-flex flex-col -space-y-1.5"),
				h.Span(
					h.Class(Cn("text-[10px]", dtAscClass)),
					g.Text("▲"),
				),
				h.Span(
					h.Class(Cn("text-[10px]", dtDescClass)),
					g.Text("▼"),
				),
			),
		),
	)
}

func dataTableToolbar(tableID string, props DataTableProps) g.Node {
	return h.Div(
		h.Class("flex items-center justify-between"),
		// Search
		g.If(props.ShowSearch,
			h.Div(
				h.Class("flex items-center gap-2"),
				Input(InputProps{
					Type:        "search",
					ID:          tableID + "-search",
					Placeholder: "Search...",
					Value:       props.SearchQuery,
					Class:       "w-64",
				},
					HxGet(props.BaseURL),
					HxTarget("#"+tableID),
					HxSwap("outerHTML"),
					HxTrigger("keyup changed delay:300ms"),
					g.Attr("name", "search"),
				),
			),
		),
		// Per-page selector
		g.If(props.ShowPerPage,
			h.Div(
				h.Class("flex items-center gap-2"),
				h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Show")),
				NativeSelect(NativeSelectProps{
					ID:    tableID + "-perpage",
					Class: "w-20",
				}, []SelectOption{
					{Value: "10", Label: "10", Selected: props.PageSize == 10},
					{Value: "25", Label: "25", Selected: props.PageSize == 25},
					{Value: "50", Label: "50", Selected: props.PageSize == 50},
					{Value: "100", Label: "100", Selected: props.PageSize == 100},
				},
					HxGet(props.BaseURL),
					HxTarget("#"+tableID),
					HxSwap("outerHTML"),
					g.Attr("name", "per_page"),
					g.Attr("onchange", "this.form.submit()"),
				),
				h.Span(h.Class("text-sm text-muted-foreground"), g.Text("entries")),
			),
		),
	)
}

func dataTablePagination(tableID string, props DataTableProps) g.Node {
	// Calculate item range
	start := (props.CurrentPage-1)*props.PageSize + 1
	end := start + props.PageSize - 1
	if end > props.TotalItems {
		end = props.TotalItems
	}

	return h.Div(
		h.Class("flex items-center justify-between px-2"),
		// Item count info
		h.Div(
			h.Class("text-sm text-muted-foreground"),
			g.Textf("Showing %d to %d of %d entries", start, end, props.TotalItems),
		),
		// Pagination controls
		h.Div(
			h.Class("flex items-center gap-1"),
			// Previous button
			dataTablePageButton(tableID, props, props.CurrentPage-1, "Previous", props.CurrentPage <= 1),
			// Page numbers
			g.Group(dataTablePageNumbers(tableID, props)),
			// Next button
			dataTablePageButton(tableID, props, props.CurrentPage+1, "Next", props.CurrentPage >= props.TotalPages),
		),
	)
}

func dataTablePageButton(tableID string, props DataTableProps, page int, label string, disabled bool) g.Node {
	if disabled {
		return h.Span(
			h.Class("inline-flex items-center justify-center rounded-md text-sm font-medium h-8 px-3 text-muted-foreground cursor-not-allowed"),
			g.Text(label),
		)
	}

	url := buildDataTableURL(props.BaseURL, page, props.PageSize, props.CurrentSort, props.SortDirection, props.SearchQuery)

	return h.Button(
		h.Type("button"),
		h.Class("inline-flex items-center justify-center rounded-md text-sm font-medium h-8 px-3 hover:bg-muted text-muted-foreground hover:text-foreground"),
		HxGet(url),
		HxTarget("#"+tableID),
		HxSwap("outerHTML"),
		HxPushURL("true"),
		g.Text(label),
	)
}

func dataTablePageNumbers(tableID string, props DataTableProps) []g.Node {
	pages := calculatePaginationPages(props.CurrentPage, props.TotalPages)
	nodes := make([]g.Node, 0)

	lastShown := 0
	for _, page := range pages {
		if lastShown > 0 && page > lastShown+1 {
			nodes = append(nodes, h.Span(
				h.Class("flex h-8 w-8 items-center justify-center text-muted-foreground"),
				g.Text("..."),
			))
		}

		isActive := page == props.CurrentPage
		url := buildDataTableURL(props.BaseURL, page, props.PageSize, props.CurrentSort, props.SortDirection, props.SearchQuery)

		buttonClass := "inline-flex items-center justify-center rounded-md text-sm font-medium h-8 w-8"
		if isActive {
			buttonClass += " border border-border bg-muted text-foreground"
		} else {
			buttonClass += " hover:bg-muted text-muted-foreground hover:text-foreground"
		}

		if isActive {
			nodes = append(nodes, h.Span(
				h.Class(buttonClass),
				g.Textf("%d", page),
			))
		} else {
			nodes = append(nodes, h.Button(
				h.Type("button"),
				h.Class(buttonClass),
				HxGet(url),
				HxTarget("#"+tableID),
				HxSwap("outerHTML"),
				HxPushURL("true"),
				g.Textf("%d", page),
			))
		}

		lastShown = page
	}

	return nodes
}

func buildDataTableURL(baseURL string, page, pageSize int, sort, dir, search string) string {
	url := baseURL + "?"
	url += "page=" + string(rune('0'+page))
	url += "&per_page=" + string(rune('0'+pageSize))
	if sort != "" {
		url += "&sort=" + sort
		url += "&dir=" + dir
	}
	if search != "" {
		url += "&search=" + search
	}
	return url
}

// DataTableScript returns the JavaScript for DataTable functionality
func DataTableScript() g.Node {
	return g.Raw(`<script>
// DataTable keyboard navigation
document.addEventListener('keydown', function(e) {
    const table = document.querySelector('[data-datatable]');
    if (!table) return;
    
    const search = table.querySelector('input[type="search"]');
    if (document.activeElement === search) return;
    
    // Arrow key navigation for pagination
    if (e.key === 'ArrowLeft') {
        const prevBtn = table.querySelector('[hx-get*="page="]');
        if (prevBtn && !prevBtn.disabled) prevBtn.click();
    } else if (e.key === 'ArrowRight') {
        const nextBtn = table.querySelectorAll('[hx-get*="page="]');
        const btn = nextBtn[nextBtn.length - 1];
        if (btn && !btn.disabled) btn.click();
    }
});
</script>`)
}
