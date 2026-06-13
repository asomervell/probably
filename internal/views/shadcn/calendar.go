package shadcn

import (
	"fmt"
	"time"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Calendar
type CalendarProps struct {
	ID           string
	Name         string
	Selected     time.Time
	Month        time.Time // The month to display
	MinDate      time.Time
	MaxDate      time.Time
	DisabledDays []time.Weekday
	Class        string
}

func Calendar(props CalendarProps) g.Node {
	id := props.ID
	if id == "" {
		id = "calendar"
	}

	month := props.Month
	if month.IsZero() {
		month = time.Now()
	}

	// Get first day of month
	firstOfMonth := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	// Get last day of month
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	// Get the weekday of the first day (0 = Sunday)
	startWeekday := int(firstOfMonth.Weekday())

	// Previous month navigation
	prevMonth := firstOfMonth.AddDate(0, -1, 0)
	nextMonth := firstOfMonth.AddDate(0, 1, 0)

	// Build calendar grid
	weeks := buildCalendarWeeks(firstOfMonth, lastOfMonth, startWeekday, props)

	return h.Div(
		h.ID(id),
		h.Class(Cn("p-3", props.Class)),
		g.Attr("data-calendar", id),

		// Hidden input for form submission
		g.If(props.Name != "",
			h.Input(
				h.Type("hidden"),
				h.Name(props.Name),
				h.ID(id+"-value"),
				h.Value(formatDate(props.Selected)),
			),
		),

		// Month navigation
		h.Div(
			h.Class("flex items-center justify-between px-1"),
			h.Button(
				h.Type("button"),
				h.Class("inline-flex items-center justify-center rounded-md text-sm font-medium h-7 w-7 bg-transparent hover:bg-muted text-muted-foreground hover:text-foreground"),
				g.Attr("onclick", fmt.Sprintf("navigateCalendar('%s', '%d', '%d')", id, prevMonth.Year(), int(prevMonth.Month()))),
				IconChevronLeft(),
			),
			h.Div(
				h.Class("text-sm font-medium text-foreground"),
				g.Text(month.Format("January 2006")),
			),
			h.Button(
				h.Type("button"),
				h.Class("inline-flex items-center justify-center rounded-md text-sm font-medium h-7 w-7 bg-transparent hover:bg-muted text-muted-foreground hover:text-foreground"),
				g.Attr("onclick", fmt.Sprintf("navigateCalendar('%s', '%d', '%d')", id, nextMonth.Year(), int(nextMonth.Month()))),
				IconChevronRight(),
			),
		),

		// Calendar grid
		h.Table(
			h.Class("w-full border-collapse space-y-1 mt-4"),
			// Weekday headers
			h.THead(
				h.Tr(
					append([]g.Node{h.Class("flex")}, calendarWeekdayHeaders()...)...,
				),
			),
			// Calendar days
			h.TBody(
				h.Class(""),
				g.Group(weeks),
			),
		),
	)
}

func calendarWeekdayHeaders() []g.Node {
	weekdays := []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	headers := make([]g.Node, 7)
	for i, wd := range weekdays {
		headers[i] = h.Th(
			h.Class("text-muted-foreground rounded-md w-8 font-normal text-[0.8rem]"),
			g.Text(wd),
		)
	}
	return headers
}

func buildCalendarWeeks(firstOfMonth, lastOfMonth time.Time, startWeekday int, props CalendarProps) []g.Node {
	id := props.ID
	if id == "" {
		id = "calendar"
	}

	var weeks []g.Node
	var currentWeek []g.Node

	// Add empty cells for days before the first of the month
	for i := 0; i < startWeekday; i++ {
		currentWeek = append(currentWeek, h.Td(h.Class("p-0 text-center text-sm")))
	}

	// Add days of the month
	for day := 1; day <= lastOfMonth.Day(); day++ {
		currentDate := time.Date(firstOfMonth.Year(), firstOfMonth.Month(), day, 0, 0, 0, 0, time.UTC)

		isSelected := !props.Selected.IsZero() &&
			props.Selected.Year() == currentDate.Year() &&
			props.Selected.Month() == currentDate.Month() &&
			props.Selected.Day() == currentDate.Day()

		isToday := isDateToday(currentDate)
		isDisabled := isDateDisabled(currentDate, props)

		cellClass := "h-8 w-8 p-0 font-normal"
		buttonClass := "inline-flex items-center justify-center rounded-md text-sm h-8 w-8 p-0 font-normal"

		if isSelected {
			buttonClass += " bg-indigo-600 text-white hover:bg-indigo-500"
		} else if isToday {
			buttonClass += " bg-muted text-foreground"
		} else if isDisabled {
			buttonClass += " text-muted-foreground cursor-not-allowed"
		} else {
			buttonClass += " text-foreground hover:bg-muted"
		}

		dateStr := formatDate(currentDate)

		var button g.Node
		if isDisabled {
			button = h.Span(
				h.Class(buttonClass),
				g.Textf("%d", day),
			)
		} else {
			button = h.Button(
				h.Type("button"),
				h.Class(buttonClass),
				g.Attr("onclick", fmt.Sprintf("selectCalendarDate('%s', '%s')", id, dateStr)),
				g.Textf("%d", day),
			)
		}

		currentWeek = append(currentWeek, h.Td(
			h.Class(cellClass),
			button,
		))

		// Start new week on Sunday
		if (startWeekday+day)%7 == 0 {
			weekRow := []g.Node{h.Class("flex w-full mt-2")}
			weekRow = append(weekRow, currentWeek...)
			weeks = append(weeks, h.Tr(weekRow...))
			currentWeek = nil
		}
	}

	// Add remaining cells for the last week
	if len(currentWeek) > 0 {
		for len(currentWeek) < 7 {
			currentWeek = append(currentWeek, h.Td(h.Class("p-0 text-center text-sm")))
		}
		weekRow := []g.Node{h.Class("flex w-full mt-2")}
		weekRow = append(weekRow, currentWeek...)
		weeks = append(weeks, h.Tr(weekRow...))
	}

	return weeks
}

func isDateToday(date time.Time) bool {
	now := time.Now()
	return date.Year() == now.Year() &&
		date.Month() == now.Month() &&
		date.Day() == now.Day()
}

func isDateDisabled(date time.Time, props CalendarProps) bool {
	// Check min date
	if !props.MinDate.IsZero() && date.Before(props.MinDate) {
		return true
	}
	// Check max date
	if !props.MaxDate.IsZero() && date.After(props.MaxDate) {
		return true
	}
	// Check disabled weekdays
	for _, wd := range props.DisabledDays {
		if date.Weekday() == wd {
			return true
		}
	}
	return false
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// CalendarScript returns the JavaScript for calendar functionality
func CalendarScript() g.Node {
	return g.Raw(`<script>
function selectCalendarDate(calendarId, dateStr) {
    const hiddenInput = document.getElementById(calendarId + '-value');
    if (hiddenInput) {
        hiddenInput.value = dateStr;
        hiddenInput.dispatchEvent(new Event('change', { bubbles: true }));
    }
    
    // Update visual selection
    const calendar = document.getElementById(calendarId);
    calendar.querySelectorAll('button').forEach(btn => {
        btn.classList.remove('bg-indigo-600', 'text-white', 'hover:bg-indigo-500');
        if (!btn.classList.contains('bg-muted')) {
            btn.classList.add('text-foreground', 'hover:bg-muted');
        }
    });
    
    event.target.classList.remove('hover:bg-muted');
    event.target.classList.add('bg-indigo-600', 'text-white', 'hover:bg-indigo-500');
}


function navigateCalendar(calendarId, year, month) {
    // This requires HTMX or full page reload to update the calendar
    // For HTMX, you'd do something like:
    htmx.ajax('GET', window.location.pathname + '?calendar_year=' + year + '&calendar_month=' + month, {
        target: '#' + calendarId,
        swap: 'outerHTML'
    });
}

</script>`)
}

// Date Picker
type DatePickerProps struct {
	ID          string
	Name        string
	Placeholder string
	Value       time.Time
	MinDate     time.Time
	MaxDate     time.Time
	Format      string // Display format (default: "Jan 2, 2006")
	Class       string
	Disabled    bool
	Required    bool
}

func DatePicker(props DatePickerProps) g.Node {
	id := props.ID
	if id == "" {
		id = props.Name + "-datepicker"
	}

	placeholder := props.Placeholder
	if placeholder == "" {
		placeholder = "Pick a date"
	}

	format := props.Format
	if format == "" {
		format = "Jan 2, 2006"
	}

	displayValue := placeholder
	textClass := "text-muted-foreground"
	if !props.Value.IsZero() {
		displayValue = props.Value.Format(format)
		textClass = "text-foreground"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("relative", props.Class)),
		g.Attr("data-datepicker", id),

		// Hidden input for form submission
		h.Input(
			h.Type("hidden"),
			h.Name(props.Name),
			h.ID(id+"-value"),
			h.Value(formatDate(props.Value)),
			g.If(props.Required, h.Required()),
		),

		// Trigger button
		h.Button(
			h.Type("button"),
			h.Class("flex h-9 w-full items-center justify-start gap-2 whitespace-nowrap rounded-md border border-border bg-card px-3 py-2 text-sm shadow-sm ring-offset-background focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:cursor-not-allowed disabled:opacity-50"),
			g.If(props.Disabled, h.Disabled()),
			g.Attr("onclick", "toggleDatePicker('"+id+"')"),
			IconCalendar(),
			h.Span(h.Class(textClass), g.Text(displayValue)),
		),

		// Calendar popover
		h.Div(
			h.ID(id+"-calendar"),
			h.Class("absolute z-50 top-full left-0 mt-1 rounded-md border border-border bg-card shadow-lg hidden"),
			Calendar(CalendarProps{
				ID:       id + "-cal",
				Name:     "", // We handle the value ourselves
				Selected: props.Value,
				Month:    props.Value,
				MinDate:  props.MinDate,
				MaxDate:  props.MaxDate,
			}),
		),
	)
}

// DatePickerScript returns the JavaScript for date picker functionality
func DatePickerScript() g.Node {
	return g.Raw(`<script>
function toggleDatePicker(id) {
    const calendar = document.getElementById(id + '-calendar');
    if (!calendar) return;
    
    const isOpen = !calendar.classList.contains('hidden');
    
    // Close all other date pickers
    document.querySelectorAll('[data-datepicker]').forEach(dp => {
        const cal = document.getElementById(dp.id + '-calendar');
        if (cal && dp.id !== id) {
            cal.classList.add('hidden');
        }
    });
    
    if (isOpen) {
        calendar.classList.add('hidden');
    } else {
        calendar.classList.remove('hidden');
        // Close on click outside
        setTimeout(() => {
            document.addEventListener('click', function closeDatePicker(e) {
                const picker = document.getElementById(id);
                if (!picker || !picker.contains(e.target)) {
                    calendar.classList.add('hidden');
                    document.removeEventListener('click', closeDatePicker);
                }
            });
        }, 0);
    }
}


// Override calendar date selection for date picker
const originalSelectCalendarDate = window.selectCalendarDate;
window.selectCalendarDate = function(calendarId, dateStr) {
    // Check if this calendar is inside a date picker
    const calendar = document.getElementById(calendarId);
    const datepicker = calendar?.closest('[data-datepicker]');
    
    if (datepicker) {
        const id = datepicker.id;
        const hiddenInput = document.getElementById(id + '-value');
        const trigger = datepicker.querySelector('button');
        const displaySpan = trigger?.querySelector('span');
        
        if (hiddenInput) {
            hiddenInput.value = dateStr;
            hiddenInput.dispatchEvent(new Event('change', { bubbles: true }));
        }
        
        if (displaySpan) {
            const date = new Date(dateStr);
            displaySpan.textContent = date.toLocaleDateString('en-US', { 
                year: 'numeric', 
                month: 'short', 
                day: 'numeric' 
            });
            displaySpan.classList.remove('text-muted-foreground');
            displaySpan.classList.add('text-foreground');
        }
        
        // Close the date picker
        const calendarPopover = document.getElementById(id + '-calendar');
        if (calendarPopover) {
            calendarPopover.classList.add('hidden');
        }
    } else if (originalSelectCalendarDate) {
        originalSelectCalendarDate(calendarId, dateStr);
    }
};
</script>`)
}

// Date Range Picker
type DateRangePickerProps struct {
	ID          string
	StartName   string
	EndName     string
	Placeholder string
	StartDate   time.Time
	EndDate     time.Time
	MinDate     time.Time
	MaxDate     time.Time
	Class       string
	Disabled    bool
}

func DateRangePicker(props DateRangePickerProps) g.Node {
	id := props.ID
	if id == "" {
		id = "date-range-picker"
	}

	placeholder := props.Placeholder
	if placeholder == "" {
		placeholder = "Select date range"
	}

	displayValue := placeholder
	textClass := "text-muted-foreground"
	if !props.StartDate.IsZero() && !props.EndDate.IsZero() {
		displayValue = fmt.Sprintf("%s - %s",
			props.StartDate.Format("Jan 2, 2006"),
			props.EndDate.Format("Jan 2, 2006"),
		)
		textClass = "text-foreground"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("relative", props.Class)),
		g.Attr("data-daterangepicker", id),

		// Hidden inputs for form submission
		h.Input(
			h.Type("hidden"),
			h.Name(props.StartName),
			h.ID(id+"-start"),
			h.Value(formatDate(props.StartDate)),
		),
		h.Input(
			h.Type("hidden"),
			h.Name(props.EndName),
			h.ID(id+"-end"),
			h.Value(formatDate(props.EndDate)),
		),

		// Trigger button
		h.Button(
			h.Type("button"),
			h.Class("flex h-9 w-full items-center justify-start gap-2 whitespace-nowrap rounded-md border border-border bg-card px-3 py-2 text-sm shadow-sm ring-offset-background focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:cursor-not-allowed disabled:opacity-50"),
			g.If(props.Disabled, h.Disabled()),
			g.Attr("onclick", "toggleDateRangePicker('"+id+"')"),
			IconCalendar(),
			h.Span(h.Class(textClass), g.Text(displayValue)),
		),

		// Calendar popover (simplified - would need more complex implementation for true range selection)
		h.Div(
			h.ID(id+"-popover"),
			h.Class("absolute z-50 top-full left-0 mt-1 rounded-md border border-border bg-card shadow-lg p-4 hidden"),
			h.Div(h.Class("text-sm text-muted-foreground mb-2"), g.Text("Select start and end dates")),
			h.Div(
				h.Class("grid grid-cols-2 gap-4"),
				h.Div(
					h.Label(h.Class("text-xs text-muted-foreground"), g.Text("Start Date")),
					h.Input(
						h.Type("date"),
						h.Class("mt-1 flex h-9 w-full rounded-md border border-border bg-card px-3 py-2 text-sm text-card-foreground"),
						h.Value(formatDate(props.StartDate)),
						g.Attr("onchange", "updateDateRange('"+id+"', 'start', this.value)"),
					),
				),
				h.Div(
					h.Label(h.Class("text-xs text-muted-foreground"), g.Text("End Date")),
					h.Input(
						h.Type("date"),
						h.Class("mt-1 flex h-9 w-full rounded-md border border-border bg-card px-3 py-2 text-sm text-card-foreground"),
						h.Value(formatDate(props.EndDate)),
						g.Attr("onchange", "updateDateRange('"+id+"', 'end', this.value)"),
					),
				),
			),
		),
	)
}

// DateRangePickerScript returns the JavaScript for date range picker functionality
func DateRangePickerScript() g.Node {
	return g.Raw(`<script>
function toggleDateRangePicker(id) {
    const popover = document.getElementById(id + '-popover');
    if (popover) {
        popover.classList.toggle('hidden');
    }
}


function updateDateRange(id, which, value) {
    const hiddenInput = document.getElementById(id + '-' + which);
    if (hiddenInput) {
        hiddenInput.value = value;
    }
    
    // Update display
    const startInput = document.getElementById(id + '-start');
    const endInput = document.getElementById(id + '-end');
    const trigger = document.querySelector('#' + id + ' button span');
    
    if (startInput.value && endInput.value && trigger) {
        const start = new Date(startInput.value);
        const end = new Date(endInput.value);
        trigger.textContent = start.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }) + 
            ' - ' + end.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
        trigger.classList.remove('text-muted-foreground');
        trigger.classList.add('text-foreground');
    }
}

</script>`)
}
