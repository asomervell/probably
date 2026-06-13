package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Popover
type PopoverAlign string

const (
	PopoverAlignStart  PopoverAlign = "start"
	PopoverAlignCenter PopoverAlign = "center"
	PopoverAlignEnd    PopoverAlign = "end"
)

type PopoverSide string

const (
	PopoverSideTop    PopoverSide = "top"
	PopoverSideRight  PopoverSide = "right"
	PopoverSideBottom PopoverSide = "bottom"
	PopoverSideLeft   PopoverSide = "left"
)

type PopoverProps struct {
	ID    string
	Open  bool
	Side  PopoverSide
	Align PopoverAlign
	Class string
}

func Popover(props PopoverProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "popover"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("relative inline-block", props.Class)),
		g.Attr("data-popover", id),
		g.Group(children),
	)
}

func PopoverTrigger(popoverID string, children ...g.Node) g.Node {
	return h.Button(
		h.Type("button"),
		g.Attr("onclick", "togglePopover('"+popoverID+"')"),
		g.Attr("aria-haspopup", "dialog"),
		g.Group(children),
	)
}

func PopoverContent(popoverID string, props PopoverProps, children ...g.Node) g.Node {
	side := props.Side
	if side == "" {
		side = PopoverSideBottom
	}

	align := props.Align
	if align == "" {
		align = PopoverAlignCenter
	}

	// Position classes based on side and align
	positionClass := getPopoverPositionClass(side, align)

	hiddenClass := "hidden"
	if props.Open {
		hiddenClass = ""
	}

	return h.Div(
		h.ID(popoverID+"-content"),
		h.Class(Cn(
			"absolute z-50 w-72 rounded-md border border-border bg-card p-4 text-card-foreground shadow-lg outline-none",
			positionClass,
			hiddenClass,
		)),
		g.Attr("data-popover-content", popoverID),
		Role("dialog"),
		g.Group(children),
	)
}

func getPopoverPositionClass(side PopoverSide, align PopoverAlign) string {
	// Vertical position
	verticalPos := ""
	switch side {
	case PopoverSideTop:
		verticalPos = "bottom-full mb-2"
	case PopoverSideBottom:
		verticalPos = "top-full mt-2"
	case PopoverSideLeft:
		verticalPos = "right-full mr-2 top-0"
	case PopoverSideRight:
		verticalPos = "left-full ml-2 top-0"
	}

	// Horizontal alignment (for top/bottom)
	if side == PopoverSideTop || side == PopoverSideBottom {
		switch align {
		case PopoverAlignStart:
			return verticalPos + " left-0"
		case PopoverAlignEnd:
			return verticalPos + " right-0"
		default:
			return verticalPos + " left-1/2 -translate-x-1/2"
		}
	}

	return verticalPos
}

// PopoverScript returns the JavaScript for popover functionality
func PopoverScript() g.Node {
	return g.Raw(`<script>
function togglePopover(id) {
    const content = document.getElementById(id + '-content');
    if (!content) return;
    
    const isOpen = !content.classList.contains('hidden');
    
    // Close all other popovers
    document.querySelectorAll('[data-popover-content]').forEach(p => {
        if (p.id !== id + '-content') {
            p.classList.add('hidden');
        }
    });
    
    if (isOpen) {
        content.classList.add('hidden');
    } else {
        content.classList.remove('hidden');
        // Close on click outside
        setTimeout(() => {
            document.addEventListener('click', function closePopover(e) {
                const container = document.getElementById(id);
                if (container && !container.contains(e.target)) {
                    content.classList.add('hidden');
                    document.removeEventListener('click', closePopover);
                }
            });
        }, 0);
    }
}


function closePopover(id) {
    const content = document.getElementById(id + '-content');
    if (content) {
        content.classList.add('hidden');
    }
}

</script>`)
}

// Tooltip
type TooltipProps struct {
	Content string
	Side    PopoverSide
	Class   string
}

func Tooltip(props TooltipProps, children ...g.Node) g.Node {
	side := props.Side
	if side == "" {
		side = PopoverSideTop
	}

	positionClass := getTooltipPositionClass(side)

	return h.Div(
		h.Class("relative inline-flex group"),
		g.Group(children),
		h.Div(
			h.Class(Cn(
				"absolute z-50 overflow-hidden rounded-md bg-muted px-3 py-1.5 text-xs text-card-foreground shadow-md",
				"opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-opacity duration-150",
				positionClass,
				props.Class,
			)),
			Role("tooltip"),
			g.Text(props.Content),
		),
	)
}

func getTooltipPositionClass(side PopoverSide) string {
	switch side {
	case PopoverSideTop:
		return "bottom-full left-1/2 -translate-x-1/2 mb-2"
	case PopoverSideBottom:
		return "top-full left-1/2 -translate-x-1/2 mt-2"
	case PopoverSideLeft:
		return "right-full top-1/2 -translate-y-1/2 mr-2"
	case PopoverSideRight:
		return "left-full top-1/2 -translate-y-1/2 ml-2"
	default:
		return "bottom-full left-1/2 -translate-x-1/2 mb-2"
	}
}

func TooltipSimple(title string, children ...g.Node) g.Node {
	return h.Span(
		h.Title(title),
		g.Group(children),
	)
}

// Hover Card
type HoverCardProps struct {
	ID    string
	Side  PopoverSide
	Align PopoverAlign
	Class string
}

func HoverCard(props HoverCardProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "hovercard"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("relative inline-block group", props.Class)),
		g.Attr("data-hovercard", id),
		g.Group(children),
	)
}

func HoverCardTrigger(children ...g.Node) g.Node {
	return h.Span(
		h.Class("cursor-pointer"),
		g.Group(children),
	)
}

func HoverCardContent(props HoverCardProps, children ...g.Node) g.Node {
	side := props.Side
	if side == "" {
		side = PopoverSideBottom
	}

	align := props.Align
	if align == "" {
		align = PopoverAlignCenter
	}

	positionClass := getPopoverPositionClass(side, align)

	return h.Div(
		h.Class(Cn(
			"absolute z-50 w-64 rounded-md border border-border bg-card p-4 text-card-foreground shadow-lg",
			"opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 delay-200",
			positionClass,
		)),
		g.Group(children),
	)
}

func HoverCardWithAvatar(trigger g.Node, avatarSrc, name, username, bio string) g.Node {
	return HoverCard(HoverCardProps{},
		HoverCardTrigger(trigger),
		HoverCardContent(HoverCardProps{},
			h.Div(
				h.Class("flex space-x-4"),
				Avatar(AvatarProps{Size: AvatarSizeLg},
					g.If(avatarSrc != "", AvatarImage(avatarSrc, name)),
					g.If(avatarSrc == "", AvatarFallback(g.Text(GetInitials(name)))),
				),
				h.Div(
					h.Class("space-y-1"),
					h.H4(h.Class("text-sm font-semibold"), g.Text(name)),
					g.If(username != "",
						h.P(h.Class("text-sm text-muted-foreground"), g.Text("@"+username)),
					),
					g.If(bio != "",
						h.P(h.Class("text-sm text-muted-foreground mt-2"), g.Text(bio)),
					),
				),
			),
		),
	)
}

// Context Menu (Right-click menu)
type ContextMenuProps struct {
	ID    string
	Class string
}

func ContextMenu(props ContextMenuProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "context-menu"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("relative", props.Class)),
		g.Attr("data-context-menu", id),
		g.Group(children),
	)
}

// ContextMenuTrigger wraps content that triggers a context menu on right-click
func ContextMenuTrigger(contextMenuID string, children ...g.Node) g.Node {
	return h.Div(
		g.Attr("oncontextmenu", "showContextMenu(event, '"+contextMenuID+"'); return false;"),
		g.Group(children),
	)
}

func ContextMenuContent(contextMenuID string, children ...g.Node) g.Node {
	return h.Div(
		h.ID(contextMenuID+"-content"),
		h.Class("fixed z-50 min-w-[8rem] overflow-hidden rounded-md border border-border bg-card p-1 text-card-foreground shadow-lg hidden"),
		g.Attr("data-context-menu-content", contextMenuID),
		g.Group(children),
	)
}

func ContextMenuItem(onclick string, children ...g.Node) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("relative flex w-full cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors hover:bg-muted hover:text-card-foreground focus:bg-muted"),
		g.If(onclick != "", g.Attr("onclick", onclick)),
		g.Group(children),
	)
}

func ContextMenuSeparator() g.Node {
	return h.Div(h.Class("-mx-1 my-1 h-px bg-border"))
}

func ContextMenuLabel(label string) g.Node {
	return h.Div(
		h.Class("px-2 py-1.5 text-sm font-semibold text-muted-foreground"),
		g.Text(label),
	)
}

// ContextMenuScript returns the JavaScript for context menu functionality
func ContextMenuScript() g.Node {
	return g.Raw(`<script>
function showContextMenu(event, id) {
    event.preventDefault();
    
    // Hide all context menus first
    document.querySelectorAll('[data-context-menu-content]').forEach(m => {
        m.classList.add('hidden');
    });
    
    const menu = document.getElementById(id + '-content');
    if (!menu) return;
    
    menu.style.left = event.pageX + 'px';
    menu.style.top = event.pageY + 'px';
    menu.classList.remove('hidden');
    
    // Close on click anywhere
    const closeMenu = () => {
        menu.classList.add('hidden');
        document.removeEventListener('click', closeMenu);
        document.removeEventListener('contextmenu', closeMenu);
    };
    
    setTimeout(() => {
        document.addEventListener('click', closeMenu);
        document.addEventListener('contextmenu', closeMenu);
    }, 0);
}

</script>`)
}
