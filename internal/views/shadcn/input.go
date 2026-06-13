package shadcn

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Label
type LabelProps struct {
	For      string
	Class    string
	Required bool
	Disabled bool
}

func Label(props LabelProps, children ...g.Node) g.Node {
	classes := Cn(
		"text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 text-foreground",
		props.Class,
	)

	attrs := []g.Node{
		h.Class(classes),
	}

	if props.For != "" {
		attrs = append(attrs, h.For(props.For))
	}

	if props.Disabled {
		attrs = append(attrs, h.Class("cursor-not-allowed opacity-70"))
	}

	content := []g.Node{g.Group(children)}
	if props.Required {
		content = append(content, h.Span(h.Class("text-destructive ml-1"), g.Text("*")))
	}

	return h.Label(
		append(attrs, g.Group(content))...,
	)
}

// Input
type InputProps struct {
	Type        string // text, email, password, number, tel, url, search, date, time, etc.
	Name        string
	ID          string
	Placeholder string
	Value       string
	Class       string
	Disabled    bool
	ReadOnly    bool
	Required    bool
	AutoFocus   bool
	Pattern     string
	Min         string
	Max         string
	Step        string
	MinLength   int
	MaxLength   int
}

const inputBaseClass = "flex h-9 w-full rounded-md border border-border bg-input px-3 py-1 text-sm text-foreground transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"

func Input(props InputProps, attrs ...g.Node) g.Node {
	inputType := props.Type
	if inputType == "" {
		inputType = "text"
	}

	classes := Cn(inputBaseClass, props.Class)

	nodes := []g.Node{
		h.Type(inputType),
		h.Class(classes),
	}

	if props.Name != "" {
		nodes = append(nodes, h.Name(props.Name))
	}
	if props.ID != "" {
		nodes = append(nodes, h.ID(props.ID))
	}
	if props.Placeholder != "" {
		nodes = append(nodes, h.Placeholder(props.Placeholder))
	}
	if props.Value != "" {
		nodes = append(nodes, h.Value(props.Value))
	}
	if props.Disabled {
		nodes = append(nodes, h.Disabled())
	}
	if props.ReadOnly {
		nodes = append(nodes, h.ReadOnly())
	}
	if props.Required {
		nodes = append(nodes, h.Required())
	}
	if props.AutoFocus {
		nodes = append(nodes, h.AutoFocus())
	}
	if props.Pattern != "" {
		nodes = append(nodes, h.Pattern(props.Pattern))
	}
	if props.Min != "" {
		nodes = append(nodes, h.Min(props.Min))
	}
	if props.Max != "" {
		nodes = append(nodes, h.Max(props.Max))
	}
	if props.Step != "" {
		nodes = append(nodes, h.Step(props.Step))
	}
	if props.MinLength > 0 {
		nodes = append(nodes, g.Attr("minlength", fmt.Sprintf("%d", props.MinLength)))
	}
	if props.MaxLength > 0 {
		nodes = append(nodes, h.MaxLength(fmt.Sprintf("%d", props.MaxLength)))
	}

	nodes = append(nodes, g.Group(attrs))

	return h.Input(nodes...)
}

// Textarea
type TextareaProps struct {
	Name        string
	ID          string
	Placeholder string
	Value       string
	Class       string
	Rows        int
	Disabled    bool
	ReadOnly    bool
	Required    bool
	MinLength   int
	MaxLength   int
}

const textareaBaseClass = "flex min-h-16 w-full rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"

func Textarea(props TextareaProps, attrs ...g.Node) g.Node {
	classes := Cn(textareaBaseClass, props.Class)

	nodes := []g.Node{
		h.Class(classes),
	}

	if props.Name != "" {
		nodes = append(nodes, h.Name(props.Name))
	}
	if props.ID != "" {
		nodes = append(nodes, h.ID(props.ID))
	}
	if props.Placeholder != "" {
		nodes = append(nodes, h.Placeholder(props.Placeholder))
	}
	if props.Rows > 0 {
		nodes = append(nodes, h.Rows(fmt.Sprintf("%d", props.Rows)))
	}
	if props.Disabled {
		nodes = append(nodes, h.Disabled())
	}
	if props.ReadOnly {
		nodes = append(nodes, h.ReadOnly())
	}
	if props.Required {
		nodes = append(nodes, h.Required())
	}
	if props.MinLength > 0 {
		nodes = append(nodes, g.Attr("minlength", fmt.Sprintf("%d", props.MinLength)))
	}
	if props.MaxLength > 0 {
		nodes = append(nodes, h.MaxLength(fmt.Sprintf("%d", props.MaxLength)))
	}

	nodes = append(nodes, g.Group(attrs))

	// Add the value as text content
	if props.Value != "" {
		nodes = append(nodes, g.Text(props.Value))
	}

	return h.Textarea(nodes...)
}

// Input Group (with prefix/suffix)
type InputGroupProps struct {
	Class string
}

// InputGroup wraps an input with prefix and/or suffix elements
func InputGroup(props InputGroupProps, children ...g.Node) g.Node {
	classes := Cn(
		"flex h-9 w-full rounded-md border border-border bg-input text-sm shadow-sm focus-within:ring-1 focus-within:ring-ring",
		props.Class,
	)

	return h.Div(
		h.Class(classes),
		g.Group(children),
	)
}

func InputGroupPrefix(children ...g.Node) g.Node {
	return h.Span(
		h.Class("flex items-center px-3 text-muted-foreground border-r border-border bg-secondary/50 rounded-l-md"),
		g.Group(children),
	)
}

func InputGroupSuffix(children ...g.Node) g.Node {
	return h.Span(
		h.Class("flex items-center px-3 text-muted-foreground border-l border-border bg-secondary/50 rounded-r-md"),
		g.Group(children),
	)
}

func InputGroupInput(props InputProps, attrs ...g.Node) g.Node {
	props.Class = Cn(
		"flex-1 border-0 bg-transparent px-3 py-1 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none disabled:cursor-not-allowed disabled:opacity-50",
		props.Class,
	)

	inputType := props.Type
	if inputType == "" {
		inputType = "text"
	}

	nodes := []g.Node{
		h.Type(inputType),
		h.Class(props.Class),
	}

	if props.Name != "" {
		nodes = append(nodes, h.Name(props.Name))
	}
	if props.ID != "" {
		nodes = append(nodes, h.ID(props.ID))
	}
	if props.Placeholder != "" {
		nodes = append(nodes, h.Placeholder(props.Placeholder))
	}
	if props.Value != "" {
		nodes = append(nodes, h.Value(props.Value))
	}
	if props.Disabled {
		nodes = append(nodes, h.Disabled())
	}
	if props.Required {
		nodes = append(nodes, h.Required())
	}

	nodes = append(nodes, g.Group(attrs))

	return h.Input(nodes...)
}

// Input OTP
type InputOTPProps struct {
	Length   int
	Name     string
	ID       string
	Class    string
	Disabled bool
}

func InputOTP(props InputOTPProps) g.Node {
	length := props.Length
	if length == 0 {
		length = 6
	}

	id := props.ID
	if id == "" {
		id = "otp"
	}

	classes := Cn(
		"flex items-center gap-2",
		props.Class,
	)

	slots := make([]g.Node, length)
	for i := 0; i < length; i++ {
		slotID := fmt.Sprintf("%s-%d", id, i)
		slots[i] = h.Input(
			h.Type("text"),
			h.ID(slotID),
			h.Name(fmt.Sprintf("%s[%d]", props.Name, i)),
			h.MaxLength("1"),
			h.Class("flex h-10 w-10 items-center justify-center border border-border bg-input rounded-md text-center text-sm font-medium text-foreground focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50"),
			g.If(props.Disabled, h.Disabled()),
			g.Attr("pattern", "[0-9]"),
			g.Attr("inputmode", "numeric"),
			g.Attr("autocomplete", "one-time-code"),
			// Auto-advance to next input on entry
			g.Attr("oninput", fmt.Sprintf(`if(this.value.length===1){const next=document.getElementById('%s-%d');if(next)next.focus()}`, id, i+1)),
			// Go back on backspace
			g.Attr("onkeydown", fmt.Sprintf(`if(event.key==='Backspace'&&!this.value){const prev=document.getElementById('%s-%d');if(prev){prev.focus();prev.select()}}`, id, i-1)),
		)
	}

	// Hidden input to collect all values
	return h.Div(
		h.Class(classes),
		g.Attr("data-otp-container", id),
		g.Group(slots),
		h.Input(
			h.Type("hidden"),
			h.Name(props.Name),
			h.ID(id+"-value"),
		),
	)
}

func InputOTPGroup(children ...g.Node) g.Node {
	return h.Div(
		h.Class("flex items-center gap-2"),
		g.Group(children),
	)
}

func InputOTPSeparator() g.Node {
	return h.Div(
		h.Class("text-muted-foreground"),
		g.Text("–"),
	)
}

// Slider
type SliderProps struct {
	Name     string
	ID       string
	Min      float64
	Max      float64
	Step     float64
	Value    float64
	Class    string
	Disabled bool
}

func Slider(props SliderProps, attrs ...g.Node) g.Node {
	min := props.Min
	max := props.Max
	if max == 0 {
		max = 100
	}
	step := props.Step
	if step == 0 {
		step = 1
	}

	classes := Cn(
		"relative flex w-full touch-none select-none items-center",
		props.Class,
	)

	// Using a native range input styled to look like shadcn slider
	return h.Div(
		h.Class(classes),
		h.Input(
			h.Type("range"),
			h.Class("w-full h-1.5 rounded-full bg-secondary appearance-none cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-foreground [&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-ring [&::-webkit-slider-thumb]:cursor-pointer [&::-webkit-slider-thumb]:shadow [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:bg-foreground [&::-moz-range-thumb]:border-2 [&::-moz-range-thumb]:border-ring [&::-moz-range-thumb]:cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"),
			g.If(props.Name != "", h.Name(props.Name)),
			g.If(props.ID != "", h.ID(props.ID)),
			h.Min(fmt.Sprintf("%g", min)),
			h.Max(fmt.Sprintf("%g", max)),
			h.Step(fmt.Sprintf("%g", step)),
			h.Value(fmt.Sprintf("%g", props.Value)),
			g.If(props.Disabled, h.Disabled()),
			g.Group(attrs),
		),
	)
}

func SliderWithValue(props SliderProps) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name
	}
	valueID := id + "-value"

	return h.Div(
		h.Class("space-y-2"),
		h.Div(
			h.Class("flex items-center justify-between"),
			h.Span(h.ID(valueID), h.Class("text-sm text-muted-foreground"), g.Text(fmt.Sprintf("%g", props.Value))),
		),
		Slider(props,
			g.Attr("oninput", fmt.Sprintf("document.getElementById('%s').textContent=this.value", valueID)),
		),
	)
}
