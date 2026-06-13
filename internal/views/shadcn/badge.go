package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type BadgeVariant string

const (
	BadgeDefault     BadgeVariant = "default"
	BadgeSecondary   BadgeVariant = "secondary"
	BadgeDestructive BadgeVariant = "destructive"
	BadgeOutline     BadgeVariant = "outline"
	BadgeSuccess     BadgeVariant = "success"
	BadgeWarning     BadgeVariant = "warning"
)

const badgeBaseClass = "inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"

var badgeVariantClasses = map[BadgeVariant]string{
	BadgeDefault:     "border-transparent bg-primary text-primary-foreground shadow hover:opacity-90",
	BadgeSecondary:   "border-transparent bg-secondary text-secondary-foreground hover:opacity-90",
	BadgeDestructive: "border-transparent bg-destructive text-primary-foreground shadow hover:opacity-90",
	BadgeOutline:     "border-border text-foreground",
	BadgeSuccess:     "border-transparent bg-chart-2 text-primary-foreground shadow hover:opacity-90",
	BadgeWarning:     "border-transparent bg-ring text-primary-foreground shadow hover:opacity-90",
}

type BadgeProps struct {
	Variant BadgeVariant
	Class   string
}

func Badge(props BadgeProps, children ...g.Node) g.Node {
	variant := props.Variant
	if variant == "" {
		variant = BadgeDefault
	}

	classes := Cn(
		badgeBaseClass,
		badgeVariantClasses[variant],
		props.Class,
	)

	return h.Span(
		h.Class(classes),
		g.Group(children),
	)
}

