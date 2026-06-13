package handlers

import (
	"net/http"
	"strconv"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	"github.com/google/uuid"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func (hdl *Handlers) TagsList(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get hierarchical tags
	tags, err := hdl.tags.GetHierarchy(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get usage counts
	counts, _ := hdl.tags.GetTagUsageCounts(r.Context(), ledger.ID)

	// Check if we just seeded tags
	seeded := r.URL.Query().Get("seeded") == "1"

	page := layouts.SettingsLayout("Tags", user.Email, "tags", user.ID.String(),
		shadcn.PageHeader("Tags", "Organize your transactions with tags",
			shadcn.ButtonAnchor(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
				"/tags/new",
				layouts.IconPlus(),
				g.Text("Add Tag"),
			),
		),

		// Success message after seeding
		g.If(seeded,
			shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertSuccess}, g.Text("Default tags have been added to your ledger.")),
		),

		// Seed tags section (only show if no tags exist)
		g.If(len(tags) == 0,
			h.Div(
				h.Class("mb-6"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardContentFull(
						h.Div(h.Class("text-center"),
							h.P(h.Class("text-muted-foreground mb-4"), g.Text("Get started quickly by adding default category tags (Income, Food & Drink, Transportation, Shopping, etc.)")),
							h.Form(
								h.Method("POST"),
								h.Action("/settings/seed-tags"),
								shadcn.Button(shadcn.ButtonProps{
									Variant: shadcn.ButtonDefault,
									Type:    "submit",
								},
									g.Text("Add Default Tags"),
								),
							),
						),
					),
				),
			),
		),

		shadcn.Card(shadcn.CardProps{},
			shadcn.CardContent(
				h.Div(
					h.Class("divide-y divide-border"),
					g.If(len(tags) == 0,
						shadcn.EmptyNoData("No tags yet", "Add default tags above or create your own.", nil),
					),
					g.Group(g.Map(tags, func(tag *models.Tag) g.Node {
						return renderTagRow(tag, counts, 0)
					})),
				),
			),
		),
	)

	renderHTML(w, page)
}

func renderTagRow(tag *models.Tag, counts map[uuid.UUID]int, depth int) g.Node {
	count := counts[tag.ID]

	return g.Group([]g.Node{
		h.Div(
			h.Class("flex items-center justify-between p-4 hover:bg-accent transition-colors"),
			h.Div(
				h.Class("flex items-center gap-3"),
				// Tree indent with visual connector
				g.If(depth > 0,
					h.Div(
						h.Class("flex items-center text-muted-foreground"),
						h.Style("padding-left: "+strconv.Itoa((depth-1)*28)+"px"),
						// Tree connector line
						h.Span(h.Class("text-muted-foreground mr-2"), g.Raw("└─")),
					),
				),
				// Color indicator
				h.Div(
					h.Class("w-4 h-4 rounded flex-shrink-0"),
					h.Style("background-color: "+tag.Color),
				),
				h.Div(
					h.P(h.Class("font-medium text-foreground"), g.Text(tag.Name)),
					h.P(h.Class("text-sm text-muted-foreground"), g.Text(strconv.Itoa(count)+" transactions")),
				),
			),
			h.Div(
				h.Class("flex items-center gap-2"),
				h.A(
					h.Href("/tags/"+tag.ID.String()+"/edit"),
					h.Class("text-muted-foreground hover:text-foreground"),
					layouts.IconEdit(),
				),
			),
		),
		// Render children
		g.Group(g.Map(tag.Children, func(child *models.Tag) g.Node {
			return renderTagRow(child, counts, depth+1)
		})),
	})
}

func (hdl *Handlers) TagsNew(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get existing tags for parent selection
	tags, _ := hdl.tags.GetByLedgerID(r.Context(), ledger.ID)

	page := layouts.SettingsLayout("New Tag", user.Email, "tags", user.ID.String(),
		shadcn.PageHeader("New Tag", "Create a new categorization tag"),
		renderTagForm(nil, tags, "/tags", "POST"),
	)

	renderHTML(w, page)
}

func (hdl *Handlers) TagsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tag := &models.Tag{
		LedgerID: ledger.ID,
		Name:     r.FormValue("name"),
		Color:    r.FormValue("color"),
	}

	if parentID := r.FormValue("parent_id"); parentID != "" {
		if id, err := uuid.Parse(parentID); err == nil {
			tag.ParentID = &id
		}
	}

	if err := hdl.tags.Create(r.Context(), tag); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/tags", http.StatusSeeOther)
}

func (hdl *Handlers) TagsEdit(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	tagID, ok := mustParamUUID(w, r, "id", "tag ID")
	if !ok {
		return
	}

	tag, err := hdl.tags.GetByID(r.Context(), tagID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Get all tags for parent selection (excluding this tag and its children)
	allTags, _ := hdl.tags.GetByLedgerID(r.Context(), tag.LedgerID)
	var availableParents []*models.Tag
	for _, t := range allTags {
		if t.ID != tag.ID {
			availableParents = append(availableParents, t)
		}
	}

	page := layouts.SettingsLayout("Edit Tag", user.Email, "tags", user.ID.String(),
		shadcn.PageHeader("Edit Tag", tag.Name),
		renderTagForm(tag, availableParents, "/tags/"+tag.ID.String(), "PUT"),

		// Delete section
		h.Div(
			h.Class("mt-6"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.H3(h.Class("text-lg font-medium text-destructive mb-2"), g.Text("Danger Zone")),
					h.P(h.Class("text-sm text-muted-foreground mb-4"), g.Text("Deleting this tag will remove it from all transactions.")),
					h.Form(
						h.Method("POST"),
						h.Action("/tags/"+tag.ID.String()),
						h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
						shadcn.Button(shadcn.ButtonProps{
							Variant: shadcn.ButtonDestructive,
							Type:    "submit",
						},
							g.Attr("onclick", "return confirm('Are you sure you want to delete this tag?')"),
							g.Text("Delete Tag"),
						),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

func (hdl *Handlers) TagsUpdate(w http.ResponseWriter, r *http.Request) {
	tagID, ok := mustParamUUID(w, r, "id", "tag ID")
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tag, err := hdl.tags.GetByID(r.Context(), tagID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	tag.Name = r.FormValue("name")
	tag.Color = r.FormValue("color")

	if parentID := r.FormValue("parent_id"); parentID != "" {
		if id, err := uuid.Parse(parentID); err == nil {
			tag.ParentID = &id
		}
	} else {
		tag.ParentID = nil
	}

	if err := hdl.tags.Update(r.Context(), tag); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/tags", http.StatusSeeOther)
}

func (hdl *Handlers) TagsDelete(w http.ResponseWriter, r *http.Request) {
	tagID, ok := mustParamUUID(w, r, "id", "tag ID")
	if !ok {
		return
	}

	if err := hdl.tags.Delete(r.Context(), tagID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/tags", http.StatusSeeOther)
}

func renderTagForm(tag *models.Tag, availableParents []*models.Tag, action, method string) g.Node {
	name := ""
	color := "#6366f1"

	if tag != nil {
		name = tag.Name
		color = tag.Color
	}

	parentOptions := []shadcn.SelectOption{{Value: "", Label: "No parent (top-level)"}}
	for _, p := range availableParents {
		parentOptions = append(parentOptions, shadcn.SelectOption{
			Value: p.ID.String(),
			Label: p.Name,
		})
	}

	// Predefined colors
	colors := []string{
		"#6366f1", // Indigo
		"#8b5cf6", // Violet
		"#ec4899", // Pink
		"#ef4444", // Red
		"#f97316", // Orange
		"#eab308", // Yellow
		"#22c55e", // Green
		"#14b8a6", // Teal
		"#06b6d4", // Cyan
		"#3b82f6", // Blue
		"#64748b", // Slate
	}

	return shadcn.Card(shadcn.CardProps{},
		shadcn.CardContentFull(
			h.Form(
				h.Method("POST"),
				h.Action(action),
				h.Class("space-y-4"),

				g.If(method == "PUT",
					h.Input(h.Type("hidden"), h.Name("_method"), h.Value("PUT")),
				),

				shadcn.FormField(shadcn.FormFieldProps{Name: "name"},
					shadcn.Label(shadcn.LabelProps{For: "name", Required: true},
						g.Text("Tag Name"),
					),
					shadcn.Input(shadcn.InputProps{
						Type:        "text",
						Name:        "name",
						Placeholder: "e.g., Groceries, Entertainment",
						Value:       name,
						Required:    true,
					}),
				),

				shadcn.FormField(shadcn.FormFieldProps{Name: "parent_id"},
					shadcn.Label(shadcn.LabelProps{For: "parent_id"},
						g.Text("Parent Tag"),
					),
					shadcn.NativeSelect(shadcn.NativeSelectProps{
						Name: "parent_id",
					}, parentOptions),
				),

				shadcn.FormField(shadcn.FormFieldProps{Name: "color"},
					shadcn.Label(shadcn.LabelProps{For: "color"},
						g.Text("Color"),
					),
					h.Input(
						h.Type("hidden"),
						h.Name("color"),
						h.ID("color-input"),
						h.Value(color),
					),
					h.Div(
						h.Class("flex flex-wrap gap-2"),
						g.Group(g.Map(colors, func(c string) g.Node {
							return h.Button(
								h.Type("button"),
								h.Class("w-8 h-8 rounded-lg transition-transform hover:scale-110 focus:outline-none focus:ring-2 focus:ring-white/50"),
								h.Style("background-color: "+c),
								g.Attr("onclick", "document.getElementById('color-input').value='"+c+"'; document.querySelectorAll('[data-color-btn]').forEach(b => b.classList.remove('ring-2', 'ring-white')); this.classList.add('ring-2', 'ring-white')"),
								g.Attr("data-color-btn", ""),
								g.If(c == color, h.Class("ring-2 ring-white")),
							)
						})),
					),
				),

				h.Div(
					h.Class("flex items-center gap-3 pt-4"),
					shadcn.Button(shadcn.ButtonProps{
						Variant: shadcn.ButtonDefault,
						Type:    "submit",
					},
						g.If(tag == nil, g.Text("Create Tag")),
						g.If(tag != nil, g.Text("Save Changes")),
					),
					h.A(
						h.Href("/tags"),
						h.Class("text-muted-foreground hover:text-foreground text-sm"),
						g.Text("Cancel"),
					),
				),
			),
		),
	)
}
