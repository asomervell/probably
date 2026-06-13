package shadcn

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Checkbox
type CheckboxProps struct {
	Name     string
	ID       string
	Value    string
	Checked  bool
	Disabled bool
	Class    string
}

func Checkbox(props CheckboxProps) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name
	}

	classes := Cn(
		"peer h-4 w-4 shrink-0 rounded-sm border border-border bg-card shadow focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-indigo-500 disabled:cursor-not-allowed disabled:opacity-50 checked:bg-indigo-600 checked:border-indigo-600 checked:text-white",
		props.Class,
	)

	return h.Input(
		h.Type("checkbox"),
		h.ID(id),
		h.Class(classes),
		g.If(props.Name != "", h.Name(props.Name)),
		g.If(props.Value != "", h.Value(props.Value)),
		g.If(props.Checked, h.Checked()),
		g.If(props.Disabled, h.Disabled()),
	)
}

func CheckboxWithLabel(props CheckboxProps, label string, description ...string) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name
	}

	return h.Div(
		h.Class("flex items-start gap-3"),
		Checkbox(props),
		h.Div(
			h.Class("grid gap-1.5 leading-none"),
			h.Label(
				h.For(id),
				h.Class("text-sm font-medium leading-none text-foreground peer-disabled:cursor-not-allowed peer-disabled:opacity-70"),
				g.Text(label),
			),
			g.If(len(description) > 0,
				h.P(h.Class("text-sm text-muted-foreground"), g.Text(description[0])),
			),
		),
	)
}

// Radio Group
type RadioGroupProps struct {
	Name        string
	Class       string
	Orientation string // "horizontal" or "vertical" (default)
	Disabled    bool
}

func RadioGroup(props RadioGroupProps, children ...g.Node) g.Node {
	orientation := props.Orientation
	if orientation == "" {
		orientation = "vertical"
	}

	flexClass := "flex flex-col gap-2"
	if orientation == "horizontal" {
		flexClass = "flex flex-row gap-4"
	}

	classes := Cn(flexClass, props.Class)

	return h.Div(
		h.Class(classes),
		Role("radiogroup"),
		DataOrientation(orientation),
		g.Group(children),
	)
}

type RadioGroupItemProps struct {
	Name     string
	ID       string
	Value    string
	Checked  bool
	Disabled bool
	Class    string
}

func RadioGroupItem(props RadioGroupItemProps) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name + "-" + props.Value
	}

	classes := Cn(
		"peer h-4 w-4 shrink-0 rounded-full border border-border bg-card shadow focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-indigo-500 disabled:cursor-not-allowed disabled:opacity-50 checked:border-indigo-600 checked:bg-indigo-600",
		props.Class,
	)

	return h.Input(
		h.Type("radio"),
		h.ID(id),
		h.Class(classes),
		h.Name(props.Name),
		h.Value(props.Value),
		g.If(props.Checked, h.Checked()),
		g.If(props.Disabled, h.Disabled()),
	)
}

func RadioGroupItemWithLabel(props RadioGroupItemProps, label string, description ...string) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name + "-" + props.Value
	}
	props.ID = id

	return h.Div(
		h.Class("flex items-start gap-3"),
		RadioGroupItem(props),
		h.Div(
			h.Class("grid gap-1.5 leading-none"),
			h.Label(
				h.For(id),
				h.Class("text-sm font-medium leading-none text-foreground peer-disabled:cursor-not-allowed peer-disabled:opacity-70"),
				g.Text(label),
			),
			g.If(len(description) > 0,
				h.P(h.Class("text-sm text-muted-foreground"), g.Text(description[0])),
			),
		),
	)
}

// Switch
type SwitchProps struct {
	Name     string
	ID       string
	Checked  bool
	Disabled bool
	Class    string
	OnChange string // JavaScript handler
}

func Switch(props SwitchProps) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name
	}

	// Using a checkbox styled as a switch
	return h.Label(
		h.Class("relative inline-flex cursor-pointer items-center"),
		h.For(id),
		h.Input(
			h.Type("checkbox"),
			h.ID(id),
			h.Class("peer sr-only"),
			g.If(props.Name != "", h.Name(props.Name)),
			g.If(props.Checked, h.Checked()),
			g.If(props.Disabled, h.Disabled()),
			g.If(props.OnChange != "", g.Attr("onchange", props.OnChange)),
		),
		h.Span(
			h.Class(Cn(
				"peer h-5 w-9 rounded-full bg-muted after:absolute after:left-[2px] after:top-0.5 after:h-4 after:w-4 after:rounded-full after:bg-white after:shadow-sm after:transition-all peer-checked:bg-indigo-600 peer-checked:after:translate-x-full peer-focus-visible:ring-2 peer-focus-visible:ring-indigo-500 peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-background peer-disabled:cursor-not-allowed peer-disabled:opacity-50",
				props.Class,
			)),
		),
	)
}

func SwitchWithLabel(props SwitchProps, label string, description ...string) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name
	}
	props.ID = id

	return h.Div(
		h.Class("flex items-center justify-between gap-4"),
		h.Div(
			h.Class("grid gap-1.5 leading-none"),
			h.Label(
				h.For(id),
				h.Class("text-sm font-medium leading-none text-foreground peer-disabled:cursor-not-allowed peer-disabled:opacity-70"),
				g.Text(label),
			),
			g.If(len(description) > 0,
				h.P(h.Class("text-sm text-muted-foreground"), g.Text(description[0])),
			),
		),
		Switch(props),
	)
}

// Checkbox Group (utility)
// CheckboxGroupOption represents an option in a checkbox group
type CheckboxGroupOption struct {
	Value       string
	Label       string
	Description string
	Checked     bool
	Disabled    bool
}

func CheckboxGroup(name string, options []CheckboxGroupOption, class string) g.Node {
	items := make([]g.Node, len(options))
	for i, opt := range options {
		items[i] = CheckboxWithLabel(
			CheckboxProps{
				Name:     fmt.Sprintf("%s[]", name),
				ID:       fmt.Sprintf("%s-%d", name, i),
				Value:    opt.Value,
				Checked:  opt.Checked,
				Disabled: opt.Disabled,
			},
			opt.Label,
			opt.Description,
		)
	}

	return h.Div(
		h.Class(Cn("flex flex-col gap-3", class)),
		Role("group"),
		g.Group(items),
	)
}
