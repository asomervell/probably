package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Native Select
// SelectOption represents an option in a select dropdown
type SelectOption struct {
	Value    string
	Label    string
	Disabled bool
	Selected bool
}

// SelectGroup represents an optgroup in a select dropdown
type SelectGroup struct {
	Label   string
	Options []SelectOption
}

type NativeSelectProps struct {
	Name        string
	ID          string
	Placeholder string
	Class       string
	Disabled    bool
	Required    bool
}

const nativeSelectBaseClass = "flex h-9 w-full items-center justify-between whitespace-nowrap rounded-md border border-border bg-card px-3 py-2 text-sm text-card-foreground shadow-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:cursor-not-allowed disabled:opacity-50"

func NativeSelect(props NativeSelectProps, options []SelectOption, attrs ...g.Node) g.Node {
	classes := Cn(nativeSelectBaseClass, props.Class)

	nodes := []g.Node{
		h.Class(classes),
	}

	if props.Name != "" {
		nodes = append(nodes, h.Name(props.Name))
	}
	if props.ID != "" {
		nodes = append(nodes, h.ID(props.ID))
	}
	if props.Disabled {
		nodes = append(nodes, h.Disabled())
	}
	if props.Required {
		nodes = append(nodes, h.Required())
	}

	// Add placeholder option
	if props.Placeholder != "" {
		nodes = append(nodes, h.Option(
			h.Value(""),
			h.Disabled(),
			h.Selected(),
			h.Class("text-muted-foreground"),
			g.Text(props.Placeholder),
		))
	}

	// Add options
	for _, opt := range options {
		optAttrs := []g.Node{
			h.Value(opt.Value),
		}
		if opt.Disabled {
			optAttrs = append(optAttrs, h.Disabled())
		}
		if opt.Selected {
			optAttrs = append(optAttrs, h.Selected())
		}
		optAttrs = append(optAttrs, g.Text(opt.Label))
		nodes = append(nodes, h.Option(optAttrs...))
	}

	nodes = append(nodes, g.Group(attrs))

	return h.Select(nodes...)
}

func NativeSelectWithGroups(props NativeSelectProps, groups []SelectGroup, attrs ...g.Node) g.Node {
	classes := Cn(nativeSelectBaseClass, props.Class)

	nodes := []g.Node{
		h.Class(classes),
	}

	if props.Name != "" {
		nodes = append(nodes, h.Name(props.Name))
	}
	if props.ID != "" {
		nodes = append(nodes, h.ID(props.ID))
	}
	if props.Disabled {
		nodes = append(nodes, h.Disabled())
	}
	if props.Required {
		nodes = append(nodes, h.Required())
	}

	// Add placeholder option
	if props.Placeholder != "" {
		nodes = append(nodes, h.Option(
			h.Value(""),
			h.Disabled(),
			h.Selected(),
			g.Text(props.Placeholder),
		))
	}

	// Add option groups
	for _, group := range groups {
		optGroupNodes := []g.Node{
			g.Attr("label", group.Label),
		}
		for _, opt := range group.Options {
			optAttrs := []g.Node{
				h.Value(opt.Value),
			}
			if opt.Disabled {
				optAttrs = append(optAttrs, h.Disabled())
			}
			if opt.Selected {
				optAttrs = append(optAttrs, h.Selected())
			}
			optAttrs = append(optAttrs, g.Text(opt.Label))
			optGroupNodes = append(optGroupNodes, h.Option(optAttrs...))
		}
		nodes = append(nodes, h.OptGroup(optGroupNodes...))
	}

	nodes = append(nodes, g.Group(attrs))

	return h.Select(nodes...)
}

// Select (Custom styled with HTMX)
type SelectProps struct {
	Name        string
	ID          string
	Placeholder string
	Value       string
	ValueLabel  string // Display label for the current value
	Class       string
	Disabled    bool
	Required    bool
	Open        bool
}

// Use with SelectContent for the dropdown options
func Select(props SelectProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name + "-select"
	}

	selectDisabledClass := "cursor-pointer"
	if props.Disabled {
		selectDisabledClass = "cursor-not-allowed opacity-50"
	}
	triggerClasses := Cn(
		"flex h-9 w-full items-center justify-between whitespace-nowrap rounded-md border border-border bg-card px-3 py-2 text-sm shadow-sm ring-offset-background focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:cursor-not-allowed disabled:opacity-50 [&>span]:line-clamp-1",
		selectDisabledClass,
		props.Class,
	)

	state := "closed"
	if props.Open {
		state = "open"
	}

	displayValue := props.Placeholder
	if props.ValueLabel != "" {
		displayValue = props.ValueLabel
	} else if props.Value != "" {
		displayValue = props.Value
	}

	textClass := "text-card-foreground"
	if displayValue == props.Placeholder {
		textClass = "text-muted-foreground"
	}

	return h.Div(
		h.ID(id),
		h.Class("relative"),
		DataState(state),
		// Hidden input to store the actual value
		h.Input(
			h.Type("hidden"),
			h.Name(props.Name),
			h.Value(props.Value),
			h.ID(id+"-value"),
		),
		// Trigger button
		h.Button(
			h.Type("button"),
			h.Class(triggerClasses),
			Role("combobox"),
			AriaExpanded(props.Open),
			AriaHasPopup("listbox"),
			g.If(props.Disabled, h.Disabled()),
			g.Attr("onclick", "toggleSelect('"+id+"')"),
			h.Span(h.Class(textClass), g.Text(displayValue)),
			IconChevronDown(),
		),
		// Content container
		g.Group(children),
	)
}

func SelectContent(selectID string, children ...g.Node) g.Node {
	return h.Div(
		h.Class("absolute z-50 top-full left-0 right-0 mt-1 max-h-96 overflow-auto rounded-md border border-border bg-card shadow-lg hidden"),
		h.ID(selectID+"-content"),
		Role("listbox"),
		g.Attr("data-select-content", selectID),
		g.Group(children),
	)
}

func SelectItem(selectID, value, label string, selected bool) g.Node {
	selectedClass := ""
	if selected {
		selectedClass = "bg-muted"
	}
	state := "unchecked"
	if selected {
		state = "checked"
	}
	return h.Button(
		h.Type("button"),
		h.Class(Cn(
			"relative flex w-full cursor-pointer select-none items-center rounded-sm py-1.5 pl-8 pr-2 text-sm text-card-foreground outline-none hover:bg-muted focus:bg-muted",
			selectedClass,
		)),
		Role("option"),
		AriaSelected(selected),
		DataState(state),
		g.Attr("onclick", "selectOption('"+selectID+"', '"+value+"', '"+label+"')"),
		// Check indicator
		h.Span(
			h.Class("absolute left-2 flex h-3.5 w-3.5 items-center justify-center"),
			g.If(selected, IconCheck()),
		),
		g.Text(label),
	)
}

func SelectSeparator() g.Node {
	return h.Div(h.Class("-mx-1 my-1 h-px bg-border"))
}

func SelectLabel(label string) g.Node {
	return h.Div(
		h.Class("px-2 py-1.5 text-sm font-semibold text-muted-foreground"),
		g.Text(label),
	)
}

// SelectGroup groups related options with an optional label
func SelectGroupEl(label string, children ...g.Node) g.Node {
	return h.Div(
		Role("group"),
		g.If(label != "", SelectLabel(label)),
		g.Group(children),
	)
}

// SelectScript returns the JavaScript needed for Select functionality
func SelectScript() g.Node {
	return g.Raw(`<script>
function toggleSelect(id) {
    const container = document.getElementById(id);
    const content = document.getElementById(id + '-content');
    const state = container.getAttribute('data-state');
    
    // Close all other selects
    document.querySelectorAll('[data-select-content]').forEach(el => {
        if (el.id !== id + '-content') {
            el.classList.add('hidden');
            const otherId = el.getAttribute('data-select-content');
            document.getElementById(otherId)?.setAttribute('data-state', 'closed');
        }
    });
    
    if (state === 'closed') {
        content.classList.remove('hidden');
        container.setAttribute('data-state', 'open');
        // Close on click outside
        setTimeout(() => {
            document.addEventListener('click', function closeSelect(e) {
                if (!container.contains(e.target)) {
                    content.classList.add('hidden');
                    container.setAttribute('data-state', 'closed');
                    document.removeEventListener('click', closeSelect);
                }
            });
        }, 0);
    } else {
        content.classList.add('hidden');
        container.setAttribute('data-state', 'closed');
    }
}


function selectOption(selectId, value, label) {
    const container = document.getElementById(selectId);
    const hiddenInput = document.getElementById(selectId + '-value');
    const content = document.getElementById(selectId + '-content');
    const trigger = container.querySelector('button');
    const displaySpan = trigger.querySelector('span');
    
    // Update hidden input value
    hiddenInput.value = value;
    
    // Update display
    displaySpan.textContent = label;
    displaySpan.classList.remove('text-muted-foreground');
    displaySpan.classList.add('text-card-foreground');
    
    // Update selected state of items
    content.querySelectorAll('[role="option"]').forEach(item => {
        const isSelected = item.textContent.trim() === label;
        item.setAttribute('aria-selected', isSelected);
        item.setAttribute('data-state', isSelected ? 'checked' : 'unchecked');
        item.classList.toggle('bg-muted', isSelected);
        const checkSpan = item.querySelector('span');
        if (checkSpan) {
            checkSpan.innerHTML = isSelected ? '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>' : '';
        }
    });
    
    // Close dropdown
    content.classList.add('hidden');
    container.setAttribute('data-state', 'closed');
    
    // Dispatch change event
    hiddenInput.dispatchEvent(new Event('change', { bubbles: true }));
}

</script>`)
}

// Combobox
type ComboboxProps struct {
	ID                string
	Name              string
	Placeholder       string
	SearchPlaceholder string
	Value             string
	ValueLabel        string
	Class             string
	Disabled          bool
	Required          bool
	Open              bool
}

func Combobox(props ComboboxProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name + "-combobox"
	}

	searchPlaceholder := props.SearchPlaceholder
	if searchPlaceholder == "" {
		searchPlaceholder = "Search..."
	}

	comboDisabledClass := "cursor-pointer"
	if props.Disabled {
		comboDisabledClass = "cursor-not-allowed opacity-50"
	}
	comboTriggerClasses := Cn(
		"flex h-9 w-full items-center justify-between whitespace-nowrap rounded-md border border-border bg-card px-3 py-2 text-sm shadow-sm ring-offset-background focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:cursor-not-allowed disabled:opacity-50 [&>span]:line-clamp-1",
		comboDisabledClass,
		props.Class,
	)

	displayValue := props.Placeholder
	if props.ValueLabel != "" {
		displayValue = props.ValueLabel
	} else if props.Value != "" {
		displayValue = props.Value
	}

	textClass := "text-card-foreground"
	if displayValue == props.Placeholder {
		textClass = "text-muted-foreground"
	}

	// Using <details> for native toggle, with minimal JS for filtering and selection
	return h.Details(
		h.ID(id),
		h.Class("relative group"),
		g.Attr("data-combobox", id),
		// Hidden input to store the actual value
		h.Input(
			h.Type("hidden"),
			h.Name(props.Name),
			h.Value(props.Value),
			h.ID(id+"-value"),
		),
		// Trigger as <summary>
		g.El("summary",
			h.Class(Cn(comboTriggerClasses, "list-none [&::-webkit-details-marker]:hidden cursor-pointer")),
			Role("combobox"),
			AriaHasPopup("listbox"),
			g.If(props.Disabled, g.Attr("disabled", "")),
			h.Span(h.ID(id+"-display"), h.Class(textClass), g.Text(displayValue)),
			IconChevronDown(),
		),
		// Dropdown content (shown when details is open)
		h.Div(
			h.ID(id+"-content"),
			h.Class("absolute z-50 top-full left-0 right-0 mt-1 rounded-md border border-border bg-card shadow-lg"),
			// Search input
			h.Div(
				h.Class("flex items-center border-b border-border px-3"),
				IconSearch(),
				h.Input(
					h.Type("text"),
					h.ID(id+"-search"),
					h.Placeholder(searchPlaceholder),
					h.Class("flex h-10 w-full rounded-md bg-transparent py-3 pl-2 text-sm text-card-foreground outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50"),
					g.Attr("oninput", "filterComboboxItems('"+id+"', this.value)"),
				),
			),
			// Options list
			h.Div(
				h.ID(id+"-list"),
				h.Class("max-h-80 overflow-y-auto p-1"),
				Role("listbox"),
				g.Group(children),
			),
			// Empty state
			h.Div(
				h.ID(id+"-empty"),
				h.Class("py-6 text-center text-sm text-muted-foreground hidden"),
				g.Text("No results found."),
			),
		),
	)
}

func ComboboxItem(comboboxID, value, label string, selected bool, keywords ...string) g.Node {
	// Keywords help with search
	searchText := label
	if len(keywords) > 0 {
		for _, kw := range keywords {
			searchText += " " + kw
		}
	}

	comboItemSelectedClass := ""
	if selected {
		comboItemSelectedClass = "bg-muted"
	}

	return h.Button(
		h.Type("button"),
		h.Class(Cn(
			"relative flex w-full cursor-pointer select-none items-center rounded-sm py-1.5 pl-8 pr-2 text-sm text-card-foreground outline-none hover:bg-muted focus:bg-muted",
			comboItemSelectedClass,
		)),
		Role("option"),
		AriaSelected(selected),
		g.Attr("data-combobox-item", ""),
		g.Attr("data-value", value),
		g.Attr("data-label", label),
		g.Attr("data-search", searchText),
		g.Attr("onclick", "selectComboboxItem('"+comboboxID+"', '"+value+"', '"+label+"')"),
		// Check indicator
		h.Span(
			h.Class("absolute left-2 flex h-3.5 w-3.5 items-center justify-center"),
			g.If(selected, IconCheck()),
		),
		g.Text(label),
	)
}

func ComboboxEmpty(message string) g.Node {
	return h.Div(
		h.Class("py-6 text-center text-sm text-muted-foreground"),
		g.Attr("data-combobox-empty", ""),
		g.Text(message),
	)
}

// ComboboxScript returns the JavaScript for combobox functionality
// Note: Toggle is handled by native <details> element; this provides filtering and selection
func ComboboxScript() g.Node {
	return g.Raw(`<script>
// Close combobox when clicking outside
document.addEventListener('click', function(e) {
    document.querySelectorAll('details[data-combobox]').forEach(function(details) {
        if (!details.contains(e.target)) {
            details.removeAttribute('open');
        }
    });
});

// Focus search input when combobox opens
document.addEventListener('toggle', function(e) {
    if (e.target.hasAttribute('data-combobox') && e.target.open) {
        const searchInput = e.target.querySelector('input[type="text"]');
        if (searchInput) {
            searchInput.value = '';
            searchInput.focus();
            filterComboboxItems(e.target.id, '');
        }
    }
}, true);

// Legacy function for backward compatibility
function toggleCombobox(id) {
    const details = document.getElementById(id);
    if (details) details.open = !details.open;
}


function filterComboboxItems(id, query) {
    const list = document.getElementById(id + '-list');
    const empty = document.getElementById(id + '-empty');
    const items = list.querySelectorAll('[data-combobox-item]');
    const lowerQuery = query.toLowerCase();
    
    let visibleCount = 0;
    items.forEach(item => {
        const searchText = (item.getAttribute('data-search') || item.getAttribute('data-label')).toLowerCase();
        const matches = searchText.includes(lowerQuery);
        item.style.display = matches ? '' : 'none';
        if (matches) visibleCount++;
    });
    
    if (empty) {
        empty.classList.toggle('hidden', visibleCount > 0);
    }
}


function selectComboboxItem(id, value, label) {
    const container = document.getElementById(id);
    const hiddenInput = document.getElementById(id + '-value');
    const content = document.getElementById(id + '-content');
    const trigger = container.querySelector('button');
    const displaySpan = trigger.querySelector('span');
    const list = document.getElementById(id + '-list');
    
    // Update hidden input value
    hiddenInput.value = value;
    
    // Update display
    displaySpan.textContent = label;
    displaySpan.classList.remove('text-muted-foreground');
    displaySpan.classList.add('text-card-foreground');
    
    // Update selected state of items
    list.querySelectorAll('[data-combobox-item]').forEach(item => {
        const isSelected = item.getAttribute('data-value') === value;
        item.setAttribute('aria-selected', isSelected);
        item.classList.toggle('bg-muted', isSelected);
        const checkSpan = item.querySelector('span');
        if (checkSpan) {
            checkSpan.innerHTML = isSelected ? '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>' : '';
        }
    });
    
    // Close dropdown
    content.classList.add('hidden');
    container.setAttribute('data-state', 'closed');
    
    // Dispatch change event
    hiddenInput.dispatchEvent(new Event('change', { bubbles: true }));
}

</script>`)
}

// Command (Command palette / search)
type CommandProps struct {
	ID          string
	Placeholder string
	Class       string
}

func Command(props CommandProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "command"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("flex h-full w-full flex-col overflow-hidden rounded-md bg-card text-card-foreground", props.Class)),
		g.Attr("data-command", id),
		g.Group(children),
	)
}

func CommandInput(commandID, placeholder string) g.Node {
	if placeholder == "" {
		placeholder = "Type a command or search..."
	}

	return h.Div(
		h.Class("flex items-center border-b border-border px-3"),
		IconSearch(),
		h.Input(
			h.Type("text"),
			h.ID(commandID+"-input"),
			h.Placeholder(placeholder),
			h.Class("flex h-11 w-full rounded-md bg-transparent py-3 pl-2 text-sm text-card-foreground outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50"),
			g.Attr("oninput", "filterCommandItems('"+commandID+"', this.value)"),
		),
	)
}

func CommandList(commandID string, children ...g.Node) g.Node {
	return h.Div(
		h.ID(commandID+"-list"),
		h.Class("max-h-80 overflow-y-auto overflow-x-hidden"),
		g.Group(children),
	)
}

func CommandEmpty(message string) g.Node {
	if message == "" {
		message = "No results found."
	}
	return h.Div(
		h.Class("py-6 text-center text-sm text-muted-foreground"),
		g.Attr("data-command-empty", ""),
		g.Text(message),
	)
}

func CommandGroup(heading string, children ...g.Node) g.Node {
	return h.Div(
		h.Class("overflow-hidden p-1 text-card-foreground"),
		g.Attr("data-command-group", ""),
		g.If(heading != "",
			h.Div(
				h.Class("px-2 py-1.5 text-xs font-medium text-muted-foreground"),
				g.Text(heading),
			),
		),
		g.Group(children),
	)
}

func CommandItem(label string, icon g.Node, onClick string, shortcut string) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none hover:bg-muted hover:text-card-foreground w-full"),
		g.Attr("data-command-item", ""),
		g.Attr("data-label", label),
		g.If(onClick != "", g.Attr("onclick", onClick)),
		g.If(icon != nil, h.Span(h.Class("mr-2"), icon)),
		h.Span(g.Text(label)),
		g.If(shortcut != "",
			h.Span(h.Class("ml-auto text-xs tracking-widest text-muted-foreground"), g.Text(shortcut)),
		),
	)
}

func CommandSeparator() g.Node {
	return h.Div(h.Class("-mx-1 h-px bg-border"))
}

// CommandScript returns the JavaScript for command functionality
func CommandScript() g.Node {
	return g.Raw(`<script>
function filterCommandItems(id, query) {
    const list = document.getElementById(id + '-list');
    const items = list.querySelectorAll('[data-command-item]');
    const groups = list.querySelectorAll('[data-command-group]');
    const lowerQuery = query.toLowerCase();
    
    let totalVisible = 0;
    
    // Filter items
    items.forEach(item => {
        const label = item.getAttribute('data-label').toLowerCase();
        const matches = label.includes(lowerQuery);
        item.style.display = matches ? '' : 'none';
        if (matches) totalVisible++;
    });
    
    // Hide empty groups
    groups.forEach(group => {
        const visibleItems = group.querySelectorAll('[data-command-item]:not([style*="display: none"])');
        group.style.display = visibleItems.length > 0 ? '' : 'none';
    });
    
    // Show/hide empty state
    const empty = list.querySelector('[data-command-empty]');
    if (empty) {
        empty.style.display = totalVisible === 0 ? '' : 'none';
    }
}

</script>`)
}

func CommandDialog(id string, open bool, children ...g.Node) g.Node {
	hiddenClass := "hidden"
	if open {
		hiddenClass = ""
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("fixed inset-0 z-50", hiddenClass)),
		g.Attr("data-command-dialog", id),
		// Backdrop
		h.Div(
			h.Class("fixed inset-0 bg-black/80"),
			g.Attr("onclick", "closeCommandDialog('"+id+"')"),
		),
		// Command palette
		h.Div(
			h.Class("fixed left-[50%] top-[50%] z-50 w-full max-w-lg translate-x-[-50%] translate-y-[-50%] rounded-lg border border-border bg-card shadow-lg"),
			Command(CommandProps{ID: id + "-cmd"},
				g.Group(children),
			),
		),
	)
}

// CommandDialogScript returns the JavaScript for command dialog
func CommandDialogScript() g.Node {
	return g.Raw(`<script>
function openCommandDialog(id) {
    const dialog = document.getElementById(id);
    if (dialog) {
        dialog.classList.remove('hidden');
        const input = document.getElementById(id + '-cmd-input');
        if (input) {
            input.value = '';
            input.focus();
        }
        document.body.classList.add('overflow-hidden');
    }
}


function closeCommandDialog(id) {
    const dialog = document.getElementById(id);
    if (dialog) {
        dialog.classList.add('hidden');
        document.body.classList.remove('overflow-hidden');
    }
}


// Open command dialog with keyboard shortcut (Cmd+K or Ctrl+K)
document.addEventListener('keydown', function(e) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        const dialog = document.querySelector('[data-command-dialog]');
        if (dialog) {
            if (dialog.classList.contains('hidden')) {
                openCommandDialog(dialog.id);
            } else {
                closeCommandDialog(dialog.id);
            }
        }
    }
    if (e.key === 'Escape') {
        document.querySelectorAll('[data-command-dialog]:not(.hidden)').forEach(d => {
            closeCommandDialog(d.id);
        });
    }
});
</script>`)
}
