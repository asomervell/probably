package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Form
type FormProps struct {
	ID       string
	Action   string
	Method   string
	Class    string
	HxPost   string // HTMX post endpoint
	HxTarget string // HTMX target
	HxSwap   string // HTMX swap strategy
}

func Form(props FormProps, children ...g.Node) g.Node {
	method := props.Method
	if method == "" {
		method = "POST"
	}

	attrs := []g.Node{
		h.Class(Cn("space-y-6", props.Class)),
		h.Method(method),
	}

	if props.ID != "" {
		attrs = append(attrs, h.ID(props.ID))
	}
	if props.Action != "" {
		attrs = append(attrs, h.Action(props.Action))
	}

	// HTMX attributes
	if props.HxPost != "" {
		attrs = append(attrs, HxPost(props.HxPost))
	}
	if props.HxTarget != "" {
		attrs = append(attrs, HxTarget(props.HxTarget))
	}
	if props.HxSwap != "" {
		attrs = append(attrs, HxSwap(props.HxSwap))
	}

	return h.Form(
		append(attrs, g.Group(children))...,
	)
}

// Form Field
type FormFieldProps struct {
	Name  string
	Class string
}

func FormField(props FormFieldProps, children ...g.Node) g.Node {
	return h.Div(
		h.Class(Cn("space-y-2", props.Class)),
		g.Attr("data-form-field", props.Name),
		g.Group(children),
	)
}

func FormItem(children ...g.Node) g.Node {
	return h.Div(
		h.Class("space-y-2"),
		g.Group(children),
	)
}

func FormLabel(forAttr string, required bool, children ...g.Node) g.Node {
	return Label(LabelProps{
		For:      forAttr,
		Required: required,
	}, children...)
}

// FormControl wraps the form input
func FormControl(children ...g.Node) g.Node {
	return h.Div(g.Group(children))
}

func FormDescription(children ...g.Node) g.Node {
	return h.P(
		h.Class("text-[0.8rem] text-muted-foreground"),
		g.Group(children),
	)
}

func FormMessage(message string, isError bool) g.Node {
	if message == "" {
		return nil
	}

	class := "text-[0.8rem] font-medium"
	if isError {
		class += " text-red-400"
	} else {
		class += " text-muted-foreground"
	}

	return h.P(
		h.Class(class),
		g.Text(message),
	)
}

func FormError(message string) g.Node {
	return FormMessage(message, true)
}

// Complete Form Field Helpers
type TextFieldProps struct {
	Name        string
	Label       string
	Type        string
	Placeholder string
	Value       string
	Description string
	Error       string
	Required    bool
	Disabled    bool
}

func TextField(props TextFieldProps) g.Node {
	fieldType := props.Type
	if fieldType == "" {
		fieldType = "text"
	}

	inputClass := ""
	if props.Error != "" {
		inputClass = "border-red-500 focus-visible:ring-red-500"
	}

	inputAttrs := []g.Node{}
	if props.Error != "" {
		inputAttrs = append(inputAttrs, AriaInvalid(true))
	}
	if props.Description != "" {
		inputAttrs = append(inputAttrs, AriaDescribedBy(props.Name+"-description"))
	}

	return FormItem(
		FormLabel(props.Name, props.Required, g.Text(props.Label)),
		FormControl(
			Input(InputProps{
				Type:        fieldType,
				Name:        props.Name,
				ID:          props.Name,
				Placeholder: props.Placeholder,
				Value:       props.Value,
				Required:    props.Required,
				Disabled:    props.Disabled,
				Class:       inputClass,
			}, inputAttrs...),
		),
		g.If(props.Description != "",
			h.P(h.ID(props.Name+"-description"), h.Class("text-[0.8rem] text-muted-foreground"), g.Text(props.Description)),
		),
		FormError(props.Error),
	)
}

type TextareaFieldProps struct {
	Name        string
	Label       string
	Placeholder string
	Value       string
	Description string
	Error       string
	Rows        int
	Required    bool
	Disabled    bool
}

func TextareaField(props TextareaFieldProps) g.Node {
	rows := props.Rows
	if rows == 0 {
		rows = 3
	}

	textareaClass := ""
	if props.Error != "" {
		textareaClass = "border-red-500 focus-visible:ring-red-500"
	}

	return FormItem(
		FormLabel(props.Name, props.Required, g.Text(props.Label)),
		FormControl(
			Textarea(TextareaProps{
				Name:        props.Name,
				ID:          props.Name,
				Placeholder: props.Placeholder,
				Value:       props.Value,
				Rows:        rows,
				Required:    props.Required,
				Disabled:    props.Disabled,
				Class:       textareaClass,
			}),
		),
		g.If(props.Description != "",
			h.P(h.Class("text-[0.8rem] text-muted-foreground"), g.Text(props.Description)),
		),
		FormError(props.Error),
	)
}

type SelectFieldProps struct {
	Name        string
	Label       string
	Placeholder string
	Options     []SelectOption
	Value       string
	Description string
	Error       string
	Required    bool
	Disabled    bool
}

func SelectField(props SelectFieldProps) g.Node {
	// Mark the selected option
	options := make([]SelectOption, len(props.Options))
	for i, opt := range props.Options {
		options[i] = SelectOption{
			Value:    opt.Value,
			Label:    opt.Label,
			Disabled: opt.Disabled,
			Selected: opt.Value == props.Value,
		}
	}

	selectClass := ""
	if props.Error != "" {
		selectClass = "border-red-500 focus-visible:ring-red-500"
	}

	return FormItem(
		FormLabel(props.Name, props.Required, g.Text(props.Label)),
		FormControl(
			NativeSelect(NativeSelectProps{
				Name:        props.Name,
				ID:          props.Name,
				Placeholder: props.Placeholder,
				Disabled:    props.Disabled,
				Required:    props.Required,
				Class:       selectClass,
			}, options),
		),
		g.If(props.Description != "",
			h.P(h.Class("text-[0.8rem] text-muted-foreground"), g.Text(props.Description)),
		),
		FormError(props.Error),
	)
}

type CheckboxFieldProps struct {
	Name        string
	Label       string
	Description string
	Checked     bool
	Error       string
	Disabled    bool
}

func CheckboxField(props CheckboxFieldProps) g.Node {
	return FormItem(
		h.Div(
			h.Class("flex items-start gap-3"),
			FormControl(
				Checkbox(CheckboxProps{
					Name:     props.Name,
					ID:       props.Name,
					Checked:  props.Checked,
					Disabled: props.Disabled,
				}),
			),
			h.Div(
				h.Class("grid gap-1.5 leading-none"),
				h.Label(
					h.For(props.Name),
					h.Class("text-sm font-medium leading-none text-foreground peer-disabled:cursor-not-allowed peer-disabled:opacity-70"),
					g.Text(props.Label),
				),
				g.If(props.Description != "",
					h.P(h.Class("text-[0.8rem] text-muted-foreground"), g.Text(props.Description)),
				),
			),
		),
		FormError(props.Error),
	)
}

type SwitchFieldProps struct {
	Name        string
	Label       string
	Description string
	Checked     bool
	Error       string
	Disabled    bool
}

func SwitchField(props SwitchFieldProps) g.Node {
	return FormItem(
		h.Div(
			h.Class("flex items-center justify-between gap-4"),
			h.Div(
				h.Class("grid gap-1.5 leading-none"),
				h.Label(
					h.For(props.Name),
					h.Class("text-sm font-medium leading-none text-foreground"),
					g.Text(props.Label),
				),
				g.If(props.Description != "",
					h.P(h.Class("text-[0.8rem] text-muted-foreground"), g.Text(props.Description)),
				),
			),
			FormControl(
				Switch(SwitchProps{
					Name:     props.Name,
					ID:       props.Name,
					Checked:  props.Checked,
					Disabled: props.Disabled,
				}),
			),
		),
		FormError(props.Error),
	)
}

// Form Actions
func FormActions(children ...g.Node) g.Node {
	return h.Div(
		h.Class("flex items-center justify-end gap-4 pt-4"),
		g.Group(children),
	)
}

func FormSubmit(label string, loading bool) g.Node {
	return Button(ButtonProps{
		Type:     "submit",
		Variant:  ButtonDefault,
		Disabled: loading,
	},
		g.If(loading, Spinner(SpinnerProps{Size: SpinnerSizeSm})),
		g.Text(label),
	)
}

func FormCancel(href string) g.Node {
	return ButtonAnchor(ButtonProps{Variant: ButtonOutline}, href,
		g.Text("Cancel"),
	)
}

// HTMX Form Helpers
// HxForm creates a form with HTMX post and common settings
func HxForm(url, target string, children ...g.Node) g.Node {
	return Form(FormProps{
		HxPost:   url,
		HxTarget: target,
		HxSwap:   "outerHTML",
	}, children...)
}

// HxFormWithIndicator creates a form with loading indicator
func HxFormWithIndicator(url, target, indicatorID string, children ...g.Node) g.Node {
	return h.Form(
		h.Class("space-y-6"),
		h.Method("POST"),
		HxPost(url),
		HxTarget(target),
		HxSwap("outerHTML"),
		HxIndicator("#"+indicatorID),
		g.Group(children),
	)
}

func FormLoadingIndicator(id string) g.Node {
	return h.Div(
		h.ID(id),
		h.Class("htmx-indicator"),
		SpinnerWithText(SpinnerProps{Size: SpinnerSizeSm}, "Saving..."),
	)
}

// Form Validation Helpers
// ValidationErrors represents a map of field errors
type ValidationErrors map[string]string

// HasError checks if a field has an error
func (v ValidationErrors) HasError(field string) bool {
	_, ok := v[field]
	return ok
}

// GetError gets the error message for a field
func (v ValidationErrors) GetError(field string) string {
	return v[field]
}

func FormWithValidation(props FormProps, errors ValidationErrors, children ...g.Node) g.Node {
	// Inject error data attribute for client-side access
	return Form(props,
		g.If(len(errors) > 0,
			Alert(AlertProps{Variant: AlertDestructive, Class: "mb-4"},
				AlertTitle(g.Text("Please fix the following errors:")),
				AlertDescription(
					h.Ul(h.Class("list-disc list-inside"),
						g.Group(mapErrorsToNodes(errors)),
					),
				),
			),
		),
		g.Group(children),
	)
}

func mapErrorsToNodes(errors ValidationErrors) []g.Node {
	nodes := make([]g.Node, 0, len(errors))
	for _, msg := range errors {
		nodes = append(nodes, h.Li(g.Text(msg)))
	}
	return nodes
}
