package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Button Variants
type ButtonVariant string

const (
	ButtonDefault     ButtonVariant = "default"
	ButtonDestructive ButtonVariant = "destructive"
	ButtonOutline     ButtonVariant = "outline"
	ButtonSecondary   ButtonVariant = "secondary"
	ButtonGhost       ButtonVariant = "ghost"
	ButtonLinkVariant ButtonVariant = "link"
)

type ButtonSize string

const (
	ButtonSizeDefault ButtonSize = "default"
	ButtonSizeSm      ButtonSize = "sm"
	ButtonSizeLg      ButtonSize = "lg"
	ButtonSizeIcon    ButtonSize = "icon"
)

// Button
// buttonBaseClass contains the base styles for all buttons
const buttonBaseClass = "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 cursor-pointer [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0"

// buttonVariantClasses maps variants to their CSS classes
var buttonVariantClasses = map[ButtonVariant]string{
	ButtonDefault:     "bg-primary text-primary-foreground shadow hover:opacity-90",
	ButtonDestructive: "bg-destructive text-primary-foreground shadow-sm hover:opacity-90",
	ButtonOutline:     "border border-border bg-transparent shadow-sm hover:bg-accent hover:text-accent-foreground",
	ButtonSecondary:   "bg-secondary text-secondary-foreground shadow-sm hover:opacity-90",
	ButtonGhost:       "hover:bg-accent hover:text-accent-foreground",
	ButtonLinkVariant: "text-primary underline-offset-4 hover:underline",
}

// buttonSizeClasses maps sizes to their CSS classes
var buttonSizeClasses = map[ButtonSize]string{
	ButtonSizeDefault: "h-9 px-4 py-2",
	ButtonSizeSm:      "h-8 rounded-md px-3 text-xs",
	ButtonSizeLg:      "h-10 rounded-md px-8",
	ButtonSizeIcon:    "h-9 w-9",
}

type ButtonProps struct {
	Variant  ButtonVariant
	Size     ButtonSize
	Disabled bool
	Type     string // "button", "submit", "reset"
	Class    string // Additional classes
}

func Button(props ButtonProps, children ...g.Node) g.Node {
	variant := props.Variant
	if variant == "" {
		variant = ButtonDefault
	}

	size := props.Size
	if size == "" {
		size = ButtonSizeDefault
	}

	buttonType := props.Type
	if buttonType == "" {
		buttonType = "button"
	}

	classes := Cn(
		buttonBaseClass,
		buttonVariantClasses[variant],
		buttonSizeClasses[size],
		props.Class,
	)

	attrs := []g.Node{
		h.Type(buttonType),
		h.Class(classes),
	}

	if props.Disabled {
		attrs = append(attrs, h.Disabled())
	}

	return h.Button(
		append(attrs, g.Group(children))...,
	)
}

func ButtonAnchor(props ButtonProps, href string, children ...g.Node) g.Node {
	variant := props.Variant
	if variant == "" {
		variant = ButtonDefault
	}

	size := props.Size
	if size == "" {
		size = ButtonSizeDefault
	}

	classes := Cn(
		buttonBaseClass,
		buttonVariantClasses[variant],
		buttonSizeClasses[size],
		props.Class,
	)

	attrs := []g.Node{
		h.Href(href),
		h.Class(classes),
	}

	if props.Disabled {
		attrs = append(attrs, AriaDisabled(true), h.Class("pointer-events-none opacity-50"))
	}

	return h.A(
		append(attrs, g.Group(children))...,
	)
}

// Button Group
func ButtonGroup(children ...g.Node) g.Node {
	return h.Div(
		h.Class("inline-flex -space-x-px rounded-md shadow-sm"),
		Role("group"),
		g.Group(children),
	)
}

// ButtonGroupItem wraps a button for use within a ButtonGroup
// It adjusts border radius for proper grouping
func ButtonGroupItem(position string, props ButtonProps, children ...g.Node) g.Node {
	var positionClass string
	switch position {
	case "first":
		positionClass = "rounded-r-none"
	case "middle":
		positionClass = "rounded-none"
	case "last":
		positionClass = "rounded-l-none"
	default:
		positionClass = ""
	}

	props.Class = Cn(props.Class, positionClass)
	return Button(props, children...)
}

// Toggle
type ToggleVariant string

const (
	ToggleDefault ToggleVariant = "default"
	ToggleOutline ToggleVariant = "outline"
)

type ToggleSize string

const (
	ToggleSizeDefault ToggleSize = "default"
	ToggleSizeSm      ToggleSize = "sm"
	ToggleSizeLg      ToggleSize = "lg"
)

const toggleBaseClass = "inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 data-[state=on]:bg-accent data-[state=on]:text-accent-foreground cursor-pointer [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0"

var toggleVariantClasses = map[ToggleVariant]string{
	ToggleDefault: "bg-transparent",
	ToggleOutline: "border border-border bg-transparent shadow-sm hover:bg-accent",
}

var toggleSizeClasses = map[ToggleSize]string{
	ToggleSizeDefault: "h-9 px-2 min-w-9",
	ToggleSizeSm:      "h-8 px-1.5 min-w-8",
	ToggleSizeLg:      "h-10 px-2.5 min-w-10",
}

type ToggleProps struct {
	Variant  ToggleVariant
	Size     ToggleSize
	Pressed  bool
	Disabled bool
	Class    string
	Name     string // For form submission
	Value    string // For form submission
}

func Toggle(props ToggleProps, children ...g.Node) g.Node {
	variant := props.Variant
	if variant == "" {
		variant = ToggleDefault
	}

	size := props.Size
	if size == "" {
		size = ToggleSizeDefault
	}

	state := "off"
	if props.Pressed {
		state = "on"
	}

	classes := Cn(
		toggleBaseClass,
		toggleVariantClasses[variant],
		toggleSizeClasses[size],
		props.Class,
	)

	attrs := []g.Node{
		h.Type("button"),
		h.Class(classes),
		AriaPressed(props.Pressed),
		DataState(state),
	}

	if props.Disabled {
		attrs = append(attrs, h.Disabled())
	}

	if props.Name != "" {
		attrs = append(attrs, h.Name(props.Name))
	}
	if props.Value != "" {
		attrs = append(attrs, h.Value(props.Value))
	}

	return h.Button(
		append(attrs, g.Group(children))...,
	)
}

// Toggle Group
type ToggleGroupType string

const (
	ToggleGroupSingle   ToggleGroupType = "single"
	ToggleGroupMultiple ToggleGroupType = "multiple"
)

type ToggleGroupProps struct {
	Type     ToggleGroupType
	Variant  ToggleVariant
	Size     ToggleSize
	Class    string
	Disabled bool
}

func ToggleGroup(props ToggleGroupProps, children ...g.Node) g.Node {
	classes := Cn(
		"flex items-center justify-center gap-1",
		props.Class,
	)

	return h.Div(
		h.Class(classes),
		Role("group"),
		g.Group(children),
	)
}

func ToggleGroupItem(props ToggleProps, children ...g.Node) g.Node {
	return Toggle(props, children...)
}

// Icon Button Helpers
func IconButton(props ButtonProps, icon g.Node) g.Node {
	props.Size = ButtonSizeIcon
	return Button(props, icon)
}

func IconButtonWithLabel(props ButtonProps, icon g.Node, label string) g.Node {
	props.Size = ButtonSizeIcon
	return Button(props,
		icon,
		VisuallyHidden(g.Text(label)),
	)
}
