package shadcn

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Progress
type ProgressProps struct {
	Value int // 0-100
	Max   int
	Class string
}

func Progress(props ProgressProps) g.Node {
	max := props.Max
	if max == 0 {
		max = 100
	}

	value := props.Value
	if value > max {
		value = max
	}
	if value < 0 {
		value = 0
	}

	percentage := float64(value) / float64(max) * 100

	return h.Div(
		h.Class(Cn("relative h-2 w-full overflow-hidden rounded-full bg-muted", props.Class)),
		Role("progressbar"),
		g.Attr("aria-valuenow", fmt.Sprintf("%d", value)),
		g.Attr("aria-valuemin", "0"),
		g.Attr("aria-valuemax", fmt.Sprintf("%d", max)),
		h.Div(
			h.Class("h-full bg-primary transition-all"),
			h.Style(fmt.Sprintf("width: %.1f%%", percentage)),
		),
	)
}

func ProgressWithLabel(props ProgressProps, label string) g.Node {
	max := props.Max
	if max == 0 {
		max = 100
	}

	return h.Div(
		h.Class("space-y-2"),
		h.Div(
			h.Class("flex justify-between text-sm"),
			h.Span(h.Class("text-muted-foreground"), g.Text(label)),
			h.Span(h.Class("text-muted-foreground"), g.Textf("%d%%", props.Value*100/max)),
		),
		Progress(props),
	)
}

func ProgressIndeterminate(class string) g.Node {
	return h.Div(
		h.Class(Cn("relative h-2 w-full overflow-hidden rounded-full bg-muted", class)),
		Role("progressbar"),
		h.Div(
			h.Class("h-full w-1/3 bg-primary animate-[progress_1s_ease-in-out_infinite]"),
		),
	)
}

// Skeleton
type SkeletonProps struct {
	Class string
}

func Skeleton(props SkeletonProps) g.Node {
	return h.Div(
		h.Class(Cn("animate-pulse rounded-md bg-muted", props.Class)),
	)
}

func SkeletonText(lines int, class string) g.Node {
	nodes := make([]g.Node, lines)
	for i := 0; i < lines; i++ {
		width := "w-full"
		if i == lines-1 && lines > 1 {
			width = "w-3/4" // Last line is shorter
		}
		nodes[i] = Skeleton(SkeletonProps{Class: Cn("h-4", width)})
	}

	return h.Div(
		h.Class(Cn("space-y-2", class)),
		g.Group(nodes),
	)
}

func SkeletonCard() g.Node {
	return h.Div(
		h.Class("rounded-xl border border-border p-6 space-y-4"),
		Skeleton(SkeletonProps{Class: "h-6 w-1/2"}),
		Skeleton(SkeletonProps{Class: "h-4 w-full"}),
		Skeleton(SkeletonProps{Class: "h-4 w-3/4"}),
	)
}

func SkeletonAvatar(size AvatarSize) g.Node {
	sizeClass := avatarSizeClasses[size]
	if sizeClass == "" {
		sizeClass = avatarSizeClasses[AvatarSizeMd]
	}

	return Skeleton(SkeletonProps{Class: Cn(sizeClass, "rounded-full")})
}

func SkeletonTable(rows, cols int) g.Node {
	headerCells := make([]g.Node, cols)
	for i := 0; i < cols; i++ {
		headerCells[i] = h.Th(h.Class("p-4"), Skeleton(SkeletonProps{Class: "h-4 w-20"}))
	}

	tableRows := make([]g.Node, rows)
	for i := 0; i < rows; i++ {
		cells := make([]g.Node, cols)
		for j := 0; j < cols; j++ {
			width := "w-full"
			if j == 0 {
				width = "w-32"
			}
			cells[j] = h.Td(h.Class("p-4"), Skeleton(SkeletonProps{Class: Cn("h-4", width)}))
		}
		tableRows[i] = h.Tr(h.Class("border-b border-border"), g.Group(cells))
	}

	return h.Div(
		h.Class("rounded-xl border border-border"),
		h.Table(
			h.Class("w-full"),
			h.THead(
				h.Class("border-b border-border"),
				h.Tr(g.Group(headerCells)),
			),
			h.TBody(g.Group(tableRows)),
		),
	)
}

// Spinner
// SpinnerSize represents spinner sizes
type SpinnerSize string

const (
	SpinnerSizeSm SpinnerSize = "sm"
	SpinnerSizeMd SpinnerSize = "md"
	SpinnerSizeLg SpinnerSize = "lg"
)

var spinnerSizeClasses = map[SpinnerSize]string{
	SpinnerSizeSm: "h-4 w-4",
	SpinnerSizeMd: "h-6 w-6",
	SpinnerSizeLg: "h-8 w-8",
}

type SpinnerProps struct {
	Size  SpinnerSize
	Class string
}

func Spinner(props SpinnerProps) g.Node {
	size := props.Size
	if size == "" {
		size = SpinnerSizeMd
	}

	return g.Raw(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" class="animate-spin %s %s" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>`,
		spinnerSizeClasses[size],
		props.Class,
	))
}

func SpinnerWithText(props SpinnerProps, text string) g.Node {
	return h.Div(
		h.Class("flex items-center gap-2 text-muted-foreground"),
		Spinner(props),
		h.Span(g.Text(text)),
	)
}

func LoadingOverlay(text string) g.Node {
	return h.Div(
		h.Class("absolute inset-0 flex items-center justify-center bg-background/80 backdrop-blur-sm z-10"),
		SpinnerWithText(SpinnerProps{Size: SpinnerSizeLg}, text),
	)
}

// Toast - Server-Side Rendered with CSS Auto-Dismiss
// ToastVariant represents toast styles
type ToastVariant string

const (
	ToastDefault ToastVariant = "default"
	ToastSuccess ToastVariant = "success"
	ToastError   ToastVariant = "error"
	ToastWarning ToastVariant = "warning"
	ToastInfo    ToastVariant = "info"
)

var toastVariantClasses = map[ToastVariant]string{
	ToastDefault: "bg-card border-border text-card-foreground",
	ToastSuccess: "bg-chart-2/10 border-chart-2/30 text-chart-2 backdrop-blur-sm",
	ToastError:   "bg-destructive/10 border-destructive/30 text-destructive backdrop-blur-sm",
	ToastWarning: "bg-ring/10 border-ring/30 text-ring backdrop-blur-sm",
	ToastInfo:    "bg-primary/10 border-primary/30 text-primary backdrop-blur-sm",
}

type ToastProps struct {
	ID          string
	Variant     ToastVariant
	Title       string
	Description string
	Duration    int // seconds for CSS animation, 0 for persistent (default 5)
	Class       string
}

// The toast uses CSS animations for entrance, exit, and auto-removal
func Toast(props ToastProps) g.Node {
	variant := props.Variant
	if variant == "" {
		variant = ToastDefault
	}

	id := props.ID
	if id == "" {
		id = fmt.Sprintf("toast-%d", randomID())
	}

	// Duration in seconds for CSS animation
	duration := props.Duration
	if duration == 0 {
		duration = 5
	}

	// Build animation style for auto-dismiss
	// toast-auto-dismiss: slide in, wait, slide out, then remove
	animStyle := ""
	if duration > 0 {
		animStyle = fmt.Sprintf("animation: toast-enter 0.2s ease-out, toast-exit 0.2s ease-in %ds forwards;", duration)
	}

	classes := Cn(
		"group pointer-events-auto relative flex w-full items-start gap-3 overflow-hidden rounded-lg border p-4 shadow-xl",
		toastVariantClasses[variant],
		props.Class,
	)

	// Icon SVG based on variant
	var icon g.Node
	switch variant {
	case ToastSuccess:
		icon = g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`)
	case ToastError:
		icon = g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`)
	case ToastWarning:
		icon = g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`)
	case ToastInfo:
		icon = g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>`)
	}

	return h.Div(
		h.ID(id),
		h.Class(classes),
		g.If(animStyle != "", h.Style(animStyle)),
		Role("alert"),
		AriaLive("polite"),
		g.Attr("data-toast", ""),
		// Icon
		g.If(icon != nil, h.Div(h.Class("shrink-0 mt-0.5"), icon)),
		// Content
		h.Div(
			h.Class("flex-1 min-w-0 grid gap-1"),
			g.If(props.Title != "",
				h.Div(h.Class("text-sm font-semibold leading-tight"), g.Text(props.Title)),
			),
			g.If(props.Description != "",
				h.Div(h.Class("text-sm opacity-90 leading-tight"), g.Text(props.Description)),
			),
		),
		// Dismiss button - uses HTMX to remove itself
		h.Button(
			h.Type("button"),
			h.Class("absolute right-2 top-2 rounded-md p-1 opacity-0 transition-opacity hover:opacity-100 group-hover:opacity-100 focus:opacity-100 hover:bg-white/10"),
			g.Attr("onclick", "this.closest('[data-toast]').remove()"),
			AriaLabel("Dismiss"),
			g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`),
		),
	)
}

// randomID generates a simple pseudo-random ID for toasts
var toastCounter int

func randomID() int {
	toastCounter++
	return toastCounter
}

func ToastContainer(toasts ...g.Node) g.Node {
	return h.Div(
		h.ID("toast-container"),
		h.Class("fixed bottom-4 right-4 z-50 flex flex-col gap-2 w-full max-w-sm pointer-events-none [&>*]:pointer-events-auto"),
		g.Attr("data-toast-container", ""),
		g.Group(toasts),
	)
}

// ToastStyles returns the CSS keyframes needed for toast animations
// Include this in the page head or in your CSS file
func ToastStyles() g.Node {
	return h.StyleEl(g.Raw(`
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
	`))
}

// Toast Helper Functions for Common Messages
// ToastMessage represents a toast to be displayed
type ToastMessage struct {
	Variant     ToastVariant
	Title       string
	Description string
	Duration    int
}

// ToastFromURLParams parses URL query parameters and returns appropriate toasts
// Supports: success=1, error=<code>, note=<note>
func ToastFromURLParams(success, errorCode, note string) []g.Node {
	var toasts []g.Node

	if success == "1" {
		title := "Done"
		description := "Your changes have been saved."

		switch note {
		case "webhook_pending":
			title = "Subscription created"
			description = "Your subscription is being set up. It may take a moment to appear."
		case "orphaned_removed":
			title = "Subscription removed"
			description = "The orphaned subscription has been cleaned up."
		case "subscription_synced":
			title = "Subscription updated"
			description = "Your subscription details have been synced."
		case "subscription_canceled":
			title = "Subscription canceled"
			description = "Your subscription has been canceled."
		}

		toasts = append(toasts, Toast(ToastProps{
			Variant:     ToastSuccess,
			Title:       title,
			Description: description,
			Duration:    5,
		}))
	}

	if errorCode != "" {
		title := "Error"
		description := "An error occurred. Please try again."

		switch errorCode {
		case "customer_not_found":
			title = "Customer not found"
			description = "The customer record could not be found. Please try subscribing again."
		case "portal_failed":
			title = "Unable to access billing portal"
			description = "Please try again later or contact support if the issue persists."
		case "no_session":
			title = "No checkout session"
			description = "The checkout session could not be found."
		case "payment_pending":
			title = "Payment pending"
			description = "Your payment is still being processed. Please wait a moment."
		case "invalid_session":
			title = "Invalid session"
			description = "The checkout session is invalid. Please try subscribing again."
		case "not_orphaned":
			title = "Cannot remove"
			description = "This subscription is not orphaned and cannot be removed this way."
		case "cleanup_failed":
			title = "Cleanup failed"
			description = "Failed to remove the orphaned subscription. Please try again."
		case "sync_failed":
			title = "Sync failed"
			description = "Failed to sync subscription from Stripe. Please try again."
		}

		toasts = append(toasts, Toast(ToastProps{
			Variant:     ToastError,
			Title:       title,
			Description: description,
			Duration:    6,
		}))
	}

	return toasts
}

// SuccessToast creates a success toast
func SuccessToast(title, description string) g.Node {
	return Toast(ToastProps{
		Variant:     ToastSuccess,
		Title:       title,
		Description: description,
		Duration:    5,
	})
}

// ErrorToast creates an error toast
func ErrorToast(title, description string) g.Node {
	return Toast(ToastProps{
		Variant:     ToastError,
		Title:       title,
		Description: description,
		Duration:    6,
	})
}

// WarningToast creates a warning toast
func WarningToast(title, description string) g.Node {
	return Toast(ToastProps{
		Variant:     ToastWarning,
		Title:       title,
		Description: description,
		Duration:    5,
	})
}

// InfoToast creates an info toast
func InfoToast(title, description string) g.Node {
	return Toast(ToastProps{
		Variant:     ToastInfo,
		Title:       title,
		Description: description,
		Duration:    5,
	})
}

// Thinking Indicator (AI reasoning display)
type ThinkingIndicatorProps struct {
	ID    string
	Class string
}

// This shows a single line with shimmer effect that updates with each new thought,
// and a chevron to expand and see all thoughts.
func ThinkingIndicator(props ThinkingIndicatorProps) g.Node {
	id := props.ID
	if id == "" {
		id = "thinking-indicator"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("flex items-start gap-3", props.Class)),
		// Brain/sparkle icon
		h.Div(
			h.Class("flex h-8 w-8 items-center justify-center rounded-full bg-primary/20 shrink-0"),
			g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-primary"><path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275z"/></svg>`),
		),
		// Content container
		h.Div(
			h.Class("flex-1 rounded-lg bg-card border border-border px-4 py-3"),
			// Header with current thought and chevron
			h.Div(
				h.Class("flex items-center justify-between gap-2 cursor-pointer"),
				g.Attr("onclick", fmt.Sprintf("toggleThinkingExpand('%s')", id)),
				// Current thought text with shimmer
				h.Div(
					h.Class("flex items-center gap-2 min-w-0 flex-1"),
					h.Span(
						h.Class("text-xs font-medium text-primary shrink-0"),
						g.Text("Thinking"),
					),
					h.Span(
						h.ID(id+"-current"),
						h.Class("text-sm text-muted-foreground truncate animate-shimmer thought-text-enter"),
						g.Text("Processing..."),
					),
				),
				// Chevron button
				h.Button(
					h.Type("button"),
					h.Class("shrink-0 p-1 rounded hover:bg-muted transition-colors"),
					AriaLabel("Toggle thought history"),
					h.Span(
						h.ID(id+"-chevron"),
						h.Class("block text-muted-foreground thinking-chevron"),
						g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>`),
					),
				),
			),
			// Expandable thoughts history (hidden by default)
			h.Div(
				h.ID(id+"-history"),
				h.Class("hidden mt-3 pt-3 border-t border-border"),
				h.Div(
					h.ID(id+"-list"),
					h.Class("space-y-2 text-sm text-muted-foreground max-h-48 overflow-y-auto"),
					// Thoughts will be appended here via JavaScript
				),
			),
		),
	)
}

// ThinkingIndicatorScript returns the JavaScript for thinking indicator functionality
func ThinkingIndicatorScript() g.Node {
	return g.Raw(`<script>
// Thinking indicator state management
const thinkingIndicators = {};

function initThinkingIndicator(id) {
    if (!thinkingIndicators[id]) {
        thinkingIndicators[id] = {
            thoughts: [],
            expanded: false
        };
    }
    return thinkingIndicators[id];
}


function addThought(id, thought) {
    const state = initThinkingIndicator(id);
    const currentEl = document.getElementById(id + '-current');
    const listEl = document.getElementById(id + '-list');
    
    if (!currentEl || !listEl) return;
    
    // Add to history if there's a previous thought
    if (state.thoughts.length > 0) {
        const prevThought = state.thoughts[state.thoughts.length - 1];
        const thoughtEl = document.createElement('div');
        thoughtEl.className = 'flex items-start gap-2';
        thoughtEl.innerHTML = '<span class="text-muted-foreground shrink-0">•</span><span class="text-muted-foreground">' + escapeHtml(prevThought) + '</span>';
        listEl.appendChild(thoughtEl);
        
        // Auto-scroll to bottom
        listEl.scrollTop = listEl.scrollHeight;
    }
    
    // Update current thought with animation
    state.thoughts.push(thought);
    currentEl.classList.remove('thought-text-enter');
    void currentEl.offsetWidth; // Trigger reflow for animation restart
    currentEl.classList.add('thought-text-enter');
    currentEl.textContent = thought;
}


function toggleThinkingExpand(id) {
    const state = initThinkingIndicator(id);
    const container = document.getElementById(id);
    const historyEl = document.getElementById(id + '-history');
    const chevronEl = document.getElementById(id + '-chevron');
    
    if (!container || !historyEl || !chevronEl) return;
    
    state.expanded = !state.expanded;
    
    if (state.expanded) {
        historyEl.classList.remove('hidden');
        chevronEl.classList.add('rotate-180');
        container.classList.add('thinking-expanded');
    } else {
        historyEl.classList.add('hidden');
        chevronEl.classList.remove('rotate-180');
        container.classList.remove('thinking-expanded');
    }
}


function clearThinkingIndicator(id) {
    const state = initThinkingIndicator(id);
    state.thoughts = [];
    state.expanded = false;
    
    const listEl = document.getElementById(id + '-list');
    const historyEl = document.getElementById(id + '-history');
    const chevronEl = document.getElementById(id + '-chevron');
    
    if (listEl) listEl.innerHTML = '';
    if (historyEl) historyEl.classList.add('hidden');
    if (chevronEl) chevronEl.classList.remove('rotate-180');
}


function removeThinkingIndicator(id) {
    const el = document.getElementById(id);
    if (el) el.remove();
    delete thinkingIndicators[id];
}


// Helper to escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

</script>`)
}
