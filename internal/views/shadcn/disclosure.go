package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Accordion
type AccordionProps struct {
	ID          string
	Type        string // "single" or "multiple"
	Class       string
	Collapsible bool // For single type, allow all items to be closed
}

func Accordion(props AccordionProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "accordion"
	}

	accordionType := props.Type
	if accordionType == "" {
		accordionType = "single"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("divide-y divide-border", props.Class)),
		g.Attr("data-accordion", id),
		g.Attr("data-accordion-type", accordionType),
		g.Attr("data-collapsible", boolToString(props.Collapsible)),
		g.Group(children),
	)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func AccordionItem(accordionID, itemID string, open bool, children ...g.Node) g.Node {
	state := "closed"
	if open {
		state = "open"
	}

	return h.Div(
		h.ID(itemID),
		h.Class("border-b border-border"),
		g.Attr("data-accordion-item", itemID),
		DataState(state),
		g.Group(children),
	)
}

func AccordionTrigger(accordionID, itemID string, children ...g.Node) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("flex flex-1 items-center justify-between py-4 text-sm font-medium text-foreground transition-all hover:underline [&[data-state=open]>svg]:rotate-180 w-full text-left"),
		g.Attr("onclick", "toggleAccordionItem('"+accordionID+"', '"+itemID+"')"),
		AriaExpanded(false),
		AriaControls(itemID+"-content"),
		g.Group(children),
		h.Span(
			h.Class("shrink-0 text-muted-foreground transition-transform duration-200"),
			IconChevronDown(),
		),
	)
}

func AccordionContent(itemID string, open bool, children ...g.Node) g.Node {
	hiddenClass := "hidden"
	if open {
		hiddenClass = ""
	}

	return h.Div(
		h.ID(itemID+"-content"),
		h.Class(Cn("overflow-hidden text-sm text-muted-foreground", hiddenClass)),
		Role("region"),
		AriaLabelledBy(itemID+"-trigger"),
		h.Div(
			h.Class("pb-4 pt-0"),
			g.Group(children),
		),
	)
}

// AccordionScript returns the JavaScript for accordion functionality
func AccordionScript() g.Node {
	return g.Raw(`<script>
function toggleAccordionItem(accordionId, itemId) {
    const accordion = document.getElementById(accordionId);
    const item = document.getElementById(itemId);
    const content = document.getElementById(itemId + '-content');
    const trigger = item.querySelector('button');
    
    if (!accordion || !item || !content) return;
    
    const type = accordion.getAttribute('data-accordion-type');
    const collapsible = accordion.getAttribute('data-collapsible') === 'true';
    const isOpen = item.getAttribute('data-state') === 'open';
    
    // For single type, close all other items first
    if (type === 'single') {
        accordion.querySelectorAll('[data-accordion-item]').forEach(otherItem => {
            if (otherItem.id !== itemId) {
                otherItem.setAttribute('data-state', 'closed');
                const otherContent = document.getElementById(otherItem.id + '-content');
                const otherTrigger = otherItem.querySelector('button');
                if (otherContent) otherContent.classList.add('hidden');
                if (otherTrigger) otherTrigger.setAttribute('aria-expanded', 'false');
            }
        });
    }
    
    // Toggle current item
    if (isOpen && (type === 'multiple' || collapsible)) {
        item.setAttribute('data-state', 'closed');
        content.classList.add('hidden');
        if (trigger) trigger.setAttribute('aria-expanded', 'false');
    } else if (!isOpen) {
        item.setAttribute('data-state', 'open');
        content.classList.remove('hidden');
        if (trigger) trigger.setAttribute('aria-expanded', 'true');
    }
}

</script>`)
}

// SimpleAccordionItem is a helper for creating a complete accordion item
type SimpleAccordionItemData struct {
	ID      string
	Title   string
	Content g.Node
	Open    bool
}

// SimpleAccordion creates an accordion from a list of items
func SimpleAccordion(id string, items []SimpleAccordionItemData, accordionType string) g.Node {
	accordionItems := make([]g.Node, len(items))
	for i, item := range items {
		itemID := item.ID
		if itemID == "" {
			itemID = id + "-item-" + string(rune('0'+i))
		}
		accordionItems[i] = AccordionItem(id, itemID, item.Open,
			AccordionTrigger(id, itemID, g.Text(item.Title)),
			AccordionContent(itemID, item.Open, item.Content),
		)
	}

	return Accordion(AccordionProps{ID: id, Type: accordionType, Collapsible: true},
		g.Group(accordionItems),
	)
}

// Collapsible
type CollapsibleProps struct {
	ID    string
	Open  bool
	Class string
}

func Collapsible(props CollapsibleProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "collapsible"
	}

	state := "closed"
	if props.Open {
		state = "open"
	}

	return h.Div(
		h.ID(id),
		h.Class(props.Class),
		g.Attr("data-collapsible", id),
		DataState(state),
		g.Group(children),
	)
}

func CollapsibleTrigger(collapsibleID string, children ...g.Node) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("flex items-center justify-between"),
		g.Attr("onclick", "toggleCollapsible('"+collapsibleID+"')"),
		AriaExpanded(false),
		AriaControls(collapsibleID+"-content"),
		g.Group(children),
	)
}

func CollapsibleContent(collapsibleID string, open bool, children ...g.Node) g.Node {
	hiddenClass := "hidden"
	if open {
		hiddenClass = ""
	}

	return h.Div(
		h.ID(collapsibleID+"-content"),
		h.Class(Cn("overflow-hidden", hiddenClass)),
		g.Attr("data-collapsible-content", collapsibleID),
		g.Group(children),
	)
}

// CollapsibleScript returns the JavaScript for collapsible functionality
func CollapsibleScript() g.Node {
	return g.Raw(`<script>
function toggleCollapsible(id) {
    const container = document.getElementById(id);
    const content = document.getElementById(id + '-content');
    const trigger = container.querySelector('button');
    
    if (!container || !content) return;
    
    const isOpen = container.getAttribute('data-state') === 'open';
    
    if (isOpen) {
        container.setAttribute('data-state', 'closed');
        content.classList.add('hidden');
        if (trigger) trigger.setAttribute('aria-expanded', 'false');
    } else {
        container.setAttribute('data-state', 'open');
        content.classList.remove('hidden');
        if (trigger) trigger.setAttribute('aria-expanded', 'true');
    }
}

</script>`)
}

func CollapsibleCard(id, title string, open bool, content g.Node) g.Node {
	state := "closed"
	contentClass := "px-4 pb-4 hidden"
	if open {
		state = "open"
		contentClass = "px-4 pb-4"
	}

	return Card(CardProps{},
		h.Div(
			h.ID(id),
			DataState(state),
			h.Div(
				h.Class("flex items-center justify-between p-4 cursor-pointer"),
				g.Attr("onclick", "toggleCollapsible('"+id+"')"),
				h.H3(h.Class("font-semibold text-foreground"), g.Text(title)),
				h.Span(
					h.Class("text-muted-foreground transition-transform duration-200 [&[data-state=open]]:rotate-180"),
					DataState(state),
					IconChevronDown(),
				),
			),
			h.Div(
				h.ID(id+"-content"),
				h.Class(contentClass),
				content,
			),
		),
	)
}

// Details/Summary (Native HTML collapsible)
func Details(open bool, summary, content g.Node) g.Node {
	attrs := []g.Node{
		h.Class("border-b border-border group"),
	}
	if open {
		attrs = append(attrs, g.Attr("open", ""))
	}

	return h.Details(
		append(attrs,
			h.Summary(
				h.Class("flex cursor-pointer items-center justify-between py-4 text-sm font-medium text-foreground hover:underline [&::-webkit-details-marker]:hidden"),
				summary,
				h.Span(
					h.Class("shrink-0 text-muted-foreground transition-transform duration-200 group-open:rotate-180"),
					IconChevronDown(),
				),
			),
			h.Div(
				h.Class("pb-4 text-sm text-muted-foreground"),
				content,
			),
		)...,
	)
}

func DetailsGroup(items []SimpleAccordionItemData) g.Node {
	nodes := make([]g.Node, len(items))
	for i, item := range items {
		nodes[i] = Details(item.Open, g.Text(item.Title), item.Content)
	}
	return h.Div(
		h.Class("divide-y divide-border"),
		g.Group(nodes),
	)
}
