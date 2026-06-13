package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Headings

func H1(children ...g.Node) g.Node {
	return h.H1(
		h.Class("scroll-m-20 text-4xl font-extrabold tracking-tight lg:text-5xl text-foreground"),
		g.Group(children),
	)
}

func H2(children ...g.Node) g.Node {
	return h.H2(
		h.Class("scroll-m-20 border-b border-border pb-2 text-3xl font-semibold tracking-tight first:mt-0 text-foreground"),
		g.Group(children),
	)
}

func H3(children ...g.Node) g.Node {
	return h.H3(
		h.Class("scroll-m-20 text-2xl font-semibold tracking-tight text-foreground"),
		g.Group(children),
	)
}

func H4(children ...g.Node) g.Node {
	return h.H4(
		h.Class("scroll-m-20 text-xl font-semibold tracking-tight text-foreground"),
		g.Group(children),
	)
}

// Body Text

func P(children ...g.Node) g.Node {
	return h.P(
		h.Class("leading-7 [&:not(:first-child)]:mt-6 text-foreground"),
		g.Group(children),
	)
}

func Lead(children ...g.Node) g.Node {
	return h.P(
		h.Class("text-xl text-muted-foreground"),
		g.Group(children),
	)
}

func Large(children ...g.Node) g.Node {
	return h.Div(
		h.Class("text-lg font-semibold text-foreground"),
		g.Group(children),
	)
}

func Small(children ...g.Node) g.Node {
	return h.Small(
		h.Class("text-sm font-medium leading-none text-foreground"),
		g.Group(children),
	)
}

func Muted(children ...g.Node) g.Node {
	return h.P(
		h.Class("text-sm text-muted-foreground"),
		g.Group(children),
	)
}

// Inline Elements

func InlineCode(children ...g.Node) g.Node {
	return h.Code(
		h.Class("relative rounded bg-muted px-[0.3rem] py-[0.2rem] font-mono text-sm text-foreground"),
		g.Group(children),
	)
}

func Strong(children ...g.Node) g.Node {
	return h.Strong(
		h.Class("font-semibold text-foreground"),
		g.Group(children),
	)
}

func Em(children ...g.Node) g.Node {
	return h.Em(
		h.Class("italic"),
		g.Group(children),
	)
}

func Link(href string, children ...g.Node) g.Node {
	return h.A(
		h.Href(href),
		h.Class("font-medium text-primary underline underline-offset-4 hover:text-primary/80"),
		g.Group(children),
	)
}

func ExternalLink(href string, children ...g.Node) g.Node {
	return h.A(
		h.Href(href),
		h.Target("_blank"),
		h.Rel("noopener noreferrer"),
		h.Class("font-medium text-primary underline underline-offset-4 hover:text-primary/80 inline-flex items-center gap-1"),
		g.Group(children),
		IconExternalLink(),
	)
}

// Block Elements

func Blockquote(children ...g.Node) g.Node {
	return h.BlockQuote(
		h.Class("mt-6 border-l-2 border-border pl-6 italic text-muted-foreground"),
		g.Group(children),
	)
}

// Lists

func Ul(children ...g.Node) g.Node {
	return h.Ul(
		h.Class("my-6 ml-6 list-disc [&>li]:mt-2 text-foreground"),
		g.Group(children),
	)
}

func Ol(children ...g.Node) g.Node {
	return h.Ol(
		h.Class("my-6 ml-6 list-decimal [&>li]:mt-2 text-foreground"),
		g.Group(children),
	)
}

func Li(children ...g.Node) g.Node {
	return h.Li(g.Group(children))
}

// Code Blocks

func Pre(children ...g.Node) g.Node {
	return h.Pre(
		h.Class("mb-4 mt-6 overflow-x-auto rounded-lg bg-card border border-border p-4"),
		g.Group(children),
	)
}

func CodeBlock(code string, language string) g.Node {
	return Pre(
		h.Code(
			h.Class("relative rounded font-mono text-sm text-foreground"),
			g.If(language != "", Data("language", language)),
			g.Text(code),
		),
	)
}

// Prose Container

func Prose(children ...g.Node) g.Node {
	return h.Div(
		h.Class("prose prose-invert prose-zinc max-w-none"),
		g.Group(children),
	)
}

func ProseSmall(children ...g.Node) g.Node {
	return h.Div(
		h.Class("prose prose-invert prose-zinc prose-sm max-w-none"),
		g.Group(children),
	)
}

// Descriptions

func DescriptionList(children ...g.Node) g.Node {
	return h.Dl(
		h.Class("divide-y divide-border"),
		g.Group(children),
	)
}

func DescriptionItem(term string, details ...g.Node) g.Node {
	return h.Div(
		h.Class("py-4 sm:grid sm:grid-cols-3 sm:gap-4"),
		h.Dt(h.Class("text-sm font-medium text-muted-foreground"), g.Text(term)),
		h.Dd(h.Class("mt-1 text-sm text-foreground sm:col-span-2 sm:mt-0"), g.Group(details)),
	)
}

// Utilities

func Truncate(children ...g.Node) g.Node {
	return h.Span(
		h.Class("truncate"),
		g.Group(children),
	)
}

func LineClamp(lines int, children ...g.Node) g.Node {
	var clampClass string
	switch lines {
	case 1:
		clampClass = "line-clamp-1"
	case 2:
		clampClass = "line-clamp-2"
	case 3:
		clampClass = "line-clamp-3"
	case 4:
		clampClass = "line-clamp-4"
	case 5:
		clampClass = "line-clamp-5"
	case 6:
		clampClass = "line-clamp-6"
	default:
		clampClass = "line-clamp-3"
	}
	return h.Span(
		h.Class(clampClass),
		g.Group(children),
	)
}
