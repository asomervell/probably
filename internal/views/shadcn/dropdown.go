package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Dropdown Menu - Using native <details> element

// Using <details> provides:
// - Native expand/collapse without JS
// - Keyboard support (space/enter to toggle)
// - Screen reader accessible

type DropdownMenuProps struct {
	ID    string
	Align PopoverAlign
	Class string
}

// This provides JS-free expand/collapse functionality
func DropdownMenu(props DropdownMenuProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "dropdown"
	}

	return h.Details(
		h.ID(id),
		h.Class(Cn("relative inline-block text-left group", props.Class)),
		g.Attr("data-dropdown", id),
		g.Group(children),
	)
}

func DropdownMenuTrigger(dropdownID string, children ...g.Node) g.Node {
	return g.El("summary",
		h.Class("inline-flex items-center justify-center cursor-pointer list-none [&::-webkit-details-marker]:hidden"),
		AriaHasPopup("menu"),
		g.Group(children),
	)
}

func DropdownMenuContent(dropdownID string, props DropdownMenuProps, children ...g.Node) g.Node {
	align := props.Align
	if align == "" {
		align = PopoverAlignEnd
	}

	alignClass := "right-0"
	if align == PopoverAlignStart {
		alignClass = "left-0"
	} else if align == PopoverAlignCenter {
		alignClass = "left-1/2 -translate-x-1/2"
	}

	return h.Div(
		h.ID(dropdownID+"-content"),
		h.Class(Cn(
			"absolute z-50 mt-2 min-w-[8rem] overflow-hidden rounded-md border border-border bg-card p-1 text-card-foreground shadow-lg",
			alignClass,
		)),
		g.Attr("data-dropdown-content", dropdownID),
		Role("menu"),
		AriaOrientation("vertical"),
		g.Group(children),
	)
}

// AriaOrientation creates an aria-orientation attribute
func AriaOrientation(orientation string) g.Node {
	return g.Attr("aria-orientation", orientation)
}

func DropdownMenuItem(children ...g.Node) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("relative flex w-full cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors hover:bg-muted focus:bg-muted focus:text-foreground"),
		Role("menuitem"),
		g.Group(children),
	)
}

func DropdownMenuItemLink(href string, children ...g.Node) g.Node {
	return h.A(
		h.Href(href),
		h.Class("relative flex w-full cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors hover:bg-muted focus:bg-muted focus:text-foreground"),
		Role("menuitem"),
		g.Group(children),
	)
}

func DropdownMenuItemWithIcon(icon g.Node, label string, onClick string) g.Node {
	attrs := []g.Node{
		h.Type("button"),
		h.Class("relative flex w-full cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none transition-colors hover:bg-muted focus:bg-muted focus:text-foreground"),
		Role("menuitem"),
	}
	if onClick != "" {
		attrs = append(attrs, g.Attr("onclick", onClick))
	}

	return h.Button(
		append(attrs,
			icon,
			h.Span(g.Text(label)),
		)...,
	)
}

func DropdownMenuItemDestructive(children ...g.Node) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("relative flex w-full cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors text-destructive hover:bg-destructive/10 hover:text-destructive focus:bg-destructive/10"),
		Role("menuitem"),
		g.Group(children),
	)
}

func DropdownMenuCheckboxItem(checked bool, onChange string, children ...g.Node) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("relative flex w-full cursor-pointer select-none items-center rounded-sm py-1.5 pl-8 pr-2 text-sm outline-none transition-colors hover:bg-muted focus:bg-muted"),
		Role("menuitemcheckbox"),
		AriaChecked(checked),
		g.If(onChange != "", g.Attr("onclick", onChange)),
		h.Span(
			h.Class("absolute left-2 flex h-3.5 w-3.5 items-center justify-center"),
			g.If(checked, IconCheck()),
		),
		g.Group(children),
	)
}

func DropdownMenuRadioItem(selected bool, value, onChange string, children ...g.Node) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("relative flex w-full cursor-pointer select-none items-center rounded-sm py-1.5 pl-8 pr-2 text-sm outline-none transition-colors hover:bg-muted focus:bg-muted"),
		Role("menuitemradio"),
		AriaChecked(selected),
		g.Attr("data-value", value),
		g.If(onChange != "", g.Attr("onclick", onChange)),
		h.Span(
			h.Class("absolute left-2 flex h-3.5 w-3.5 items-center justify-center"),
			g.If(selected, IconDot()),
		),
		g.Group(children),
	)
}

func DropdownMenuLabel(label string) g.Node {
	return h.Div(
		h.Class("px-2 py-1.5 text-sm font-semibold text-muted-foreground"),
		g.Text(label),
	)
}

func DropdownMenuSeparator() g.Node {
	return h.Div(h.Class("-mx-1 my-1 h-px bg-border"))
}

func DropdownMenuShortcut(shortcut string) g.Node {
	return h.Span(
		h.Class("ml-auto text-xs tracking-widest text-muted-foreground"),
		g.Text(shortcut),
	)
}

// DropdownMenuGroup groups related menu items
func DropdownMenuGroup(children ...g.Node) g.Node {
	return h.Div(
		Role("group"),
		g.Group(children),
	)
}

func DropdownMenuSub(id, label string, children ...g.Node) g.Node {
	return h.Details(
		h.Class("relative group/sub"),
		g.Attr("data-dropdown-sub", id),
		g.El("summary",
			h.Class("relative flex w-full cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors hover:bg-muted focus:bg-muted list-none [&::-webkit-details-marker]:hidden"),
			h.Span(g.Text(label)),
			h.Span(h.Class("ml-auto"), IconChevronRight()),
		),
		h.Div(
			h.ID(id+"-submenu"),
			h.Class("absolute left-full top-0 ml-1 min-w-[8rem] overflow-hidden rounded-md border border-border bg-card p-1 shadow-lg"),
			g.Group(children),
		),
	)
}

// DropdownMenuScript returns minimal JavaScript for dropdown functionality
// Note: <details> handles most functionality natively; this adds click-outside-to-close
func DropdownMenuScript() g.Node {
	return g.Raw(`<script>
// Close dropdowns when clicking outside
document.addEventListener('click', function(e) {
    document.querySelectorAll('details[data-dropdown]').forEach(function(details) {
        if (!details.contains(e.target)) {
            details.removeAttribute('open');
        }
    });
});

// Legacy functions for backward compatibility
function toggleDropdown(id) {
    const details = document.getElementById(id);
    if (details) details.open = !details.open;
}


function showSubmenu(id) {
    const details = document.querySelector('[data-dropdown-sub="' + id + '"]');
    if (details) details.open = true;
}


function hideSubmenu(id) {
    const details = document.querySelector('[data-dropdown-sub="' + id + '"]');
    if (details) details.open = false;
}

</script>`)
}

// Menubar - Using native <details> element
type MenubarProps struct {
	Class string
}

func Menubar(props MenubarProps, children ...g.Node) g.Node {
	return h.Div(
		h.Class(Cn(
			"flex h-9 items-center space-x-1 rounded-md border border-border bg-card p-1",
			props.Class,
		)),
		Role("menubar"),
		g.Group(children),
	)
}

func MenubarMenu(id string, children ...g.Node) g.Node {
	return h.Details(
		h.Class("relative"),
		g.Attr("data-menubar-menu", id),
		g.Group(children),
	)
}

func MenubarTrigger(menuID string, children ...g.Node) g.Node {
	return g.El("summary",
		h.Class("flex cursor-pointer select-none items-center rounded-sm px-3 py-1 text-sm font-medium outline-none hover:bg-muted hover:text-foreground focus:bg-muted focus:text-foreground list-none [&::-webkit-details-marker]:hidden"),
		g.Group(children),
	)
}

func MenubarContent(menuID string, children ...g.Node) g.Node {
	return h.Div(
		h.ID(menuID+"-content"),
		h.Class("absolute left-0 top-full z-50 mt-1 min-w-[12rem] overflow-hidden rounded-md border border-border bg-card p-1 shadow-lg"),
		g.Attr("data-menubar-content", menuID),
		Role("menu"),
		g.Group(children),
	)
}

func MenubarItem(children ...g.Node) g.Node {
	return DropdownMenuItem(children...)
}

func MenubarSeparator() g.Node {
	return DropdownMenuSeparator()
}

func MenubarShortcut(shortcut string) g.Node {
	return DropdownMenuShortcut(shortcut)
}

// MenubarScript returns minimal JavaScript for menubar functionality
// Uses the same close-on-click-outside behavior as dropdowns
func MenubarScript() g.Node {
	return g.Raw(`<script>
// Close menubar menus when clicking outside
document.addEventListener('click', function(e) {
    document.querySelectorAll('details[data-menubar-menu]').forEach(function(details) {
        if (!details.contains(e.target)) {
            details.removeAttribute('open');
        }
    });
});

// Legacy function for backward compatibility
function toggleMenubarMenu(id) {
    const details = document.querySelector('[data-menubar-menu="' + id + '"]');
    if (details) details.open = !details.open;
}

</script>`)
}

// Simple Dropdown Helper
// SimpleDropdownItem represents an item in a simple dropdown
type SimpleDropdownItem struct {
	Label       string
	Href        string
	Icon        g.Node
	Destructive bool
	Separator   bool
	OnClick     string
}

// SimpleDropdown creates a dropdown from a list of items
func SimpleDropdown(id string, trigger g.Node, items []SimpleDropdownItem) g.Node {
	menuItems := make([]g.Node, 0, len(items))
	for _, item := range items {
		if item.Separator {
			menuItems = append(menuItems, DropdownMenuSeparator())
			continue
		}

		if item.Href != "" {
			menuItems = append(menuItems, DropdownMenuItemLink(item.Href,
				g.If(item.Icon != nil, item.Icon),
				h.Span(g.Text(item.Label)),
			))
		} else if item.Destructive {
			menuItems = append(menuItems, DropdownMenuItemDestructive(
				g.If(item.Icon != nil, item.Icon),
				h.Span(g.Text(item.Label)),
			))
		} else {
			menuItems = append(menuItems, DropdownMenuItemWithIcon(item.Icon, item.Label, item.OnClick))
		}
	}

	return DropdownMenu(DropdownMenuProps{ID: id},
		DropdownMenuTrigger(id, trigger),
		DropdownMenuContent(id, DropdownMenuProps{}, menuItems...),
	)
}

func ActionsDropdown(id string, items []SimpleDropdownItem) g.Node {
	trigger := Button(ButtonProps{Variant: ButtonGhost, Size: ButtonSizeIcon},
		IconMoreVertical(),
		VisuallyHidden(g.Text("Actions")),
	)
	return SimpleDropdown(id, trigger, items)
}
