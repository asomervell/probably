// Package shadcn provides a Go gomponents implementation of shadcn/ui components.
// These components are designed for server-side rendering with HTMX for interactivity.
package shadcn

import (
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Class Utilities
// Cn combines multiple class strings, filtering out empty strings.
// This is the Go equivalent of shadcn's cn() utility.
func Cn(classes ...string) string {
	var result []string
	for _, c := range classes {
		if c = strings.TrimSpace(c); c != "" {
			result = append(result, c)
		}
	}
	return strings.Join(result, " ")
}

// Class creates an h.Class attribute from multiple class strings.
func Class(classes ...string) g.Node {
	return h.Class(Cn(classes...))
}

// Size Constants
type Size string

const (
	SizeDefault Size = "default"
	SizeSm      Size = "sm"
	SizeLg      Size = "lg"
	SizeIcon    Size = "icon"
)

// HTMX Helper Functions
// HxGet creates an hx-get attribute
func HxGet(url string) g.Node {
	return g.Attr("hx-get", url)
}

// HxPost creates an hx-post attribute
func HxPost(url string) g.Node {
	return g.Attr("hx-post", url)
}

// HxTarget creates an hx-target attribute
func HxTarget(selector string) g.Node {
	return g.Attr("hx-target", selector)
}

// HxSwap creates an hx-swap attribute
func HxSwap(value string) g.Node {
	return g.Attr("hx-swap", value)
}

// HxTrigger creates an hx-trigger attribute
func HxTrigger(value string) g.Node {
	return g.Attr("hx-trigger", value)
}

// HxPushURL creates an hx-push-url attribute
func HxPushURL(value string) g.Node {
	return g.Attr("hx-push-url", value)
}

// HxIndicator creates an hx-indicator attribute
func HxIndicator(selector string) g.Node {
	return g.Attr("hx-indicator", selector)
}

// HxSwapOOB creates an hx-swap-oob attribute
func HxSwapOOB(value string) g.Node {
	return g.Attr("hx-swap-oob", value)
}

// Data Attributes
// Data creates a data-* attribute
func Data(name, value string) g.Node {
	return g.Attr("data-"+name, value)
}

// DataState creates a data-state attribute (common in shadcn components)
func DataState(state string) g.Node {
	return g.Attr("data-state", state)
}

// DataOrientation creates a data-orientation attribute
func DataOrientation(orientation string) g.Node {
	return g.Attr("data-orientation", orientation)
}

// ARIA Attributes
// AriaLabel creates an aria-label attribute
func AriaLabel(label string) g.Node {
	return g.Attr("aria-label", label)
}

// AriaLabelledBy creates an aria-labelledby attribute
func AriaLabelledBy(id string) g.Node {
	return g.Attr("aria-labelledby", id)
}

// AriaDescribedBy creates an aria-describedby attribute
func AriaDescribedBy(id string) g.Node {
	return g.Attr("aria-describedby", id)
}

// AriaExpanded creates an aria-expanded attribute
func AriaExpanded(expanded bool) g.Node {
	if expanded {
		return g.Attr("aria-expanded", "true")
	}
	return g.Attr("aria-expanded", "false")
}

// AriaHidden creates an aria-hidden attribute
func AriaHidden(hidden bool) g.Node {
	if hidden {
		return g.Attr("aria-hidden", "true")
	}
	return nil
}

// AriaSelected creates an aria-selected attribute
func AriaSelected(selected bool) g.Node {
	if selected {
		return g.Attr("aria-selected", "true")
	}
	return g.Attr("aria-selected", "false")
}

// AriaDisabled creates an aria-disabled attribute
func AriaDisabled(disabled bool) g.Node {
	if disabled {
		return g.Attr("aria-disabled", "true")
	}
	return nil
}

// AriaControls creates an aria-controls attribute
func AriaControls(id string) g.Node {
	return g.Attr("aria-controls", id)
}

// AriaHasPopup creates an aria-haspopup attribute
func AriaHasPopup(value string) g.Node {
	return g.Attr("aria-haspopup", value)
}

// AriaPressed creates an aria-pressed attribute for toggle buttons
func AriaPressed(pressed bool) g.Node {
	if pressed {
		return g.Attr("aria-pressed", "true")
	}
	return g.Attr("aria-pressed", "false")
}

// AriaChecked creates an aria-checked attribute
func AriaChecked(checked bool) g.Node {
	if checked {
		return g.Attr("aria-checked", "true")
	}
	return g.Attr("aria-checked", "false")
}

// AriaInvalid creates an aria-invalid attribute
func AriaInvalid(invalid bool) g.Node {
	if invalid {
		return g.Attr("aria-invalid", "true")
	}
	return nil
}

// AriaLive creates an aria-live attribute
func AriaLive(value string) g.Node {
	return g.Attr("aria-live", value)
}

// Role creates a role attribute
func Role(role string) g.Node {
	return g.Attr("role", role)
}

// TabIndex creates a tabindex attribute
func TabIndex(index int) g.Node {
	if index == 0 {
		return g.Attr("tabindex", "0")
	}
	return g.Attr("tabindex", "-1")
}

// Common Element Helpers
func VisuallyHidden(children ...g.Node) g.Node {
	return h.Span(
		h.Class("sr-only"),
		g.Group(children),
	)
}

func Portal(id string) g.Node {
	return h.Div(
		h.ID(id),
		h.Class("contents"),
	)
}

// Icon creates an SVG icon from raw paths (16px)
func Icon(paths string) g.Node {
	return g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`)
}

// IconLg creates a larger SVG icon (20px)
func IconLg(paths string) g.Node {
	return g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`)
}

// Common Icons (Lucide icons used by shadcn)
func IconCheck() g.Node {
	return Icon(`<path d="M20 6 9 17l-5-5"/>`)
}

func IconChevronDown() g.Node {
	return Icon(`<path d="m6 9 6 6 6-6"/>`)
}

func IconChevronRight() g.Node {
	return Icon(`<path d="m9 18 6-6-6-6"/>`)
}

func IconChevronLeft() g.Node {
	return Icon(`<path d="m15 18-6-6 6-6"/>`)
}

func IconX() g.Node {
	return Icon(`<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`)
}

func IconSearch() g.Node {
	return Icon(`<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`)
}

func IconLoader() g.Node {
	return g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="animate-spin"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>`)
}

func IconDot() g.Node {
	return g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="12" r="4"/></svg>`)
}

func IconAlertCircle() g.Node {
	return Icon(`<circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/>`)
}

func IconAlertTriangle() g.Node {
	return Icon(`<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/>`)
}

func IconInfo() g.Node {
	return Icon(`<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>`)
}

func IconCheckCircle() g.Node {
	return Icon(`<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><path d="m9 11 3 3L22 4"/>`)
}

func IconMoreHorizontal() g.Node {
	return Icon(`<circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/>`)
}

func IconMoreVertical() g.Node {
	return Icon(`<circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/>`)
}

func IconCalendar() g.Node {
	return Icon(`<path d="M8 2v4"/><path d="M16 2v4"/><rect width="18" height="18" x="3" y="4" rx="2"/><path d="M3 10h18"/>`)
}

func IconExternalLink() g.Node {
	return Icon(`<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>`)
}

func IconGripVertical() g.Node {
	return Icon(`<circle cx="9" cy="12" r="1"/><circle cx="9" cy="5" r="1"/><circle cx="9" cy="19" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="15" cy="5" r="1"/><circle cx="15" cy="19" r="1"/>`)
}
