package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Alert Variants
type AlertVariant string

const (
	AlertDefault     AlertVariant = "default"
	AlertDestructive AlertVariant = "destructive"
	AlertSuccess     AlertVariant = "success"
	AlertWarning     AlertVariant = "warning"
	AlertInfo        AlertVariant = "info"
)

var alertVariantClasses = map[AlertVariant]string{
	AlertDefault:     "bg-card border-border text-card-foreground [&>svg]:text-card-foreground",
	AlertDestructive: "bg-destructive/10 border-destructive/30 text-destructive [&>svg]:text-destructive",
	AlertSuccess:     "bg-chart-2/10 border-chart-2/30 text-chart-2 [&>svg]:text-chart-2",
	AlertWarning:     "bg-ring/10 border-ring/30 text-ring [&>svg]:text-ring",
	AlertInfo:        "bg-primary/10 border-primary/30 text-primary [&>svg]:text-primary",
}

// Alert
type AlertProps struct {
	Variant AlertVariant
	Class   string
}

func Alert(props AlertProps, children ...g.Node) g.Node {
	variant := props.Variant
	if variant == "" {
		variant = AlertDefault
	}

	classes := Cn(
		"relative w-full rounded-lg border px-4 py-3 text-sm [&>svg+div]:translate-y-[-3px] [&>svg]:absolute [&>svg]:left-4 [&>svg]:top-4 [&>svg~*]:pl-7",
		alertVariantClasses[variant],
		props.Class,
	)

	return h.Div(
		h.Class(classes),
		Role("alert"),
		g.Group(children),
	)
}

func AlertTitle(children ...g.Node) g.Node {
	return h.H5(
		h.Class("mb-1 font-medium leading-none tracking-tight"),
		g.Group(children),
	)
}

func AlertDescription(children ...g.Node) g.Node {
	return h.Div(
		h.Class("text-sm [&_p]:leading-relaxed"),
		g.Group(children),
	)
}

