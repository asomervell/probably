package shadcn

import (
	"fmt"
	"math"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Chart Types and Data
// ChartDataPoint represents a single data point
type ChartDataPoint struct {
	Label string
	Value float64
	Color string // Optional custom color
}

// ChartSeries represents a data series for multi-series charts
type ChartSeries struct {
	Name  string
	Color string
	Data  []float64
}

type ChartProps struct {
	ID     string
	Width  int
	Height int
	Class  string
}

// Default chart colors (indigo palette)
var defaultChartColors = []string{
	"#6366f1", // indigo-500
	"#8b5cf6", // violet-500
	"#ec4899", // pink-500
	"#f59e0b", // amber-500
	"#10b981", // emerald-500
	"#3b82f6", // blue-500
	"#ef4444", // red-500
	"#06b6d4", // cyan-500
}

func getChartColor(index int, custom string) string {
	if custom != "" {
		return custom
	}
	return defaultChartColors[index%len(defaultChartColors)]
}

// Area Chart
type AreaChartProps struct {
	ChartProps
	Labels     []string
	Series     []ChartSeries
	ShowGrid   bool
	ShowLegend bool
	Stacked    bool
	Smooth     bool
}

func AreaChart(props AreaChartProps) g.Node {
	width := props.Width
	if width == 0 {
		width = 400
	}
	height := props.Height
	if height == 0 {
		height = 200
	}

	padding := 40
	chartWidth := width - padding*2
	chartHeight := height - padding*2

	// Calculate max value
	maxVal := 0.0
	for _, series := range props.Series {
		for _, v := range series.Data {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal == 0 {
		maxVal = 100
	}

	// Build paths for each series
	var paths []g.Node
	for i, series := range props.Series {
		color := getChartColor(i, series.Color)
		path := buildAreaPath(series.Data, chartWidth, chartHeight, padding, maxVal, props.Smooth)

		paths = append(paths,
			g.Raw(fmt.Sprintf(`<path d="%s" fill="%s" fill-opacity="0.2" stroke="%s" stroke-width="2"/>`, path, color, color)),
		)
	}

	// Build grid lines
	var grid []g.Node
	if props.ShowGrid {
		for i := 0; i <= 4; i++ {
			y := padding + (chartHeight * i / 4)
			grid = append(grid, g.Raw(fmt.Sprintf(
				`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#3f3f46" stroke-dasharray="4"/>`,
				padding, y, width-padding, y,
			)))
		}
	}

	// Build X-axis labels
	var labels []g.Node
	if len(props.Labels) > 0 {
		for i, label := range props.Labels {
			var x int
			if len(props.Labels) == 1 {
				x = padding + chartWidth/2
			} else {
				step := chartWidth / (len(props.Labels) - 1)
				x = padding + (step * i)
			}
			labels = append(labels, g.Raw(fmt.Sprintf(
				`<text x="%d" y="%d" text-anchor="middle" class="fill-muted-foreground text-xs">%s</text>`,
				x, height-10, label,
			)))
		}
	}

	return h.Div(
		h.Class(Cn("relative", props.Class)),
		g.Raw(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height)),
		g.Group(grid),
		g.Group(paths),
		g.Group(labels),
		g.Raw(`</svg>`),
		g.If(props.ShowLegend, chartLegend(props.Series)),
	)
}

func buildAreaPath(data []float64, chartWidth, chartHeight, padding int, maxVal float64, smooth bool) string {
	if len(data) == 0 {
		return ""
	}
	if len(data) == 1 {
		y := padding + int(float64(chartHeight)*(1-data[0]/maxVal))
		return fmt.Sprintf("M %d %d L %d %d L %d %d L %d %d Z",
			padding, padding+chartHeight, padding, y, padding+chartWidth, y, padding+chartWidth, padding+chartHeight)
	}

	step := float64(chartWidth) / float64(len(data)-1)
	var path strings.Builder

	// Start path at bottom left
	path.WriteString(fmt.Sprintf("M %d %d ", padding, padding+chartHeight))

	// Line to first data point
	firstY := padding + int(float64(chartHeight)*(1-data[0]/maxVal))
	path.WriteString(fmt.Sprintf("L %d %d ", padding, firstY))

	// Draw line through all points
	for i, v := range data {
		x := padding + int(step*float64(i))
		y := padding + int(float64(chartHeight)*(1-v/maxVal))
		path.WriteString(fmt.Sprintf("L %d %d ", x, y))
	}

	// Close path back to bottom right, then bottom left
	path.WriteString(fmt.Sprintf("L %d %d ", padding+chartWidth, padding+chartHeight))
	path.WriteString("Z")

	return path.String()
}

// Bar Chart
type BarChartProps struct {
	ChartProps
	Data       []ChartDataPoint
	Horizontal bool
	ShowValues bool
}

func BarChart(props BarChartProps) g.Node {
	width := props.Width
	if width == 0 {
		width = 400
	}
	height := props.Height
	if height == 0 {
		height = 200
	}

	padding := 40
	chartWidth := width - padding*2
	chartHeight := height - padding*2

	// Calculate max value
	maxVal := 0.0
	for _, d := range props.Data {
		if d.Value > maxVal {
			maxVal = d.Value
		}
	}
	if maxVal == 0 {
		maxVal = 100
	}

	barGap := 8
	barWidth := (chartWidth - (len(props.Data)-1)*barGap) / len(props.Data)

	var bars []g.Node
	var labels []g.Node

	for i, d := range props.Data {
		color := getChartColor(i, d.Color)
		x := padding + i*(barWidth+barGap)
		barHeight := int(float64(chartHeight) * (d.Value / maxVal))
		y := padding + chartHeight - barHeight

		bars = append(bars, g.Raw(fmt.Sprintf(
			`<rect x="%d" y="%d" width="%d" height="%d" fill="%s" rx="4"/>`,
			x, y, barWidth, barHeight, color,
		)))

		// Label
		labels = append(labels, g.Raw(fmt.Sprintf(
			`<text x="%d" y="%d" text-anchor="middle" class="fill-muted-foreground text-xs">%s</text>`,
			x+barWidth/2, height-10, d.Label,
		)))

		// Value
		if props.ShowValues {
			bars = append(bars, g.Raw(fmt.Sprintf(
				`<text x="%d" y="%d" text-anchor="middle" class="fill-foreground text-xs">%.0f</text>`,
				x+barWidth/2, y-5, d.Value,
			)))
		}
	}

	return h.Div(
		h.Class(Cn("relative", props.Class)),
		g.Raw(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height)),
		g.Group(bars),
		g.Group(labels),
		g.Raw(`</svg>`),
	)
}

// Line Chart
type LineChartProps struct {
	ChartProps
	Labels      []string
	Series      []ChartSeries
	ShowGrid    bool
	ShowLegend  bool
	ShowDots    bool
	Smooth      bool
	ShowYAxis   bool                 // Show Y-axis labels
	YAxisFormat func(float64) string // Optional formatter for Y-axis values
}

func LineChart(props LineChartProps) g.Node {
	width := props.Width
	if width == 0 {
		width = 400
	}
	height := props.Height
	if height == 0 {
		height = 200
	}

	padding := 40
	leftPadding := padding
	if props.ShowYAxis {
		leftPadding = padding + 40 // Extra space for Y-axis labels
	}
	chartWidth := width - leftPadding - padding
	chartHeight := height - padding*2

	// Calculate max value
	maxVal := 0.0
	for _, series := range props.Series {
		for _, v := range series.Data {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal == 0 {
		maxVal = 100
	}

	// Build paths for each series
	var elements []g.Node
	for i, series := range props.Series {
		color := getChartColor(i, series.Color)
		path := buildLinePath(series.Data, chartWidth, chartHeight, leftPadding, padding, maxVal)

		elements = append(elements,
			g.Raw(fmt.Sprintf(`<path d="%s" fill="none" stroke="%s" stroke-width="2"/>`, path, color)),
		)

		// Add dots
		if props.ShowDots {
			if len(series.Data) == 1 {
				v := series.Data[0]
				x := leftPadding
				y := padding + int(float64(chartHeight)*(1-v/maxVal))
				elements = append(elements, g.Raw(fmt.Sprintf(
					`<circle cx="%d" cy="%d" r="4" fill="%s"/>`,
					x, y, color,
				)))
			} else {
				step := float64(chartWidth) / float64(len(series.Data)-1)
				for j, v := range series.Data {
					x := leftPadding + int(step*float64(j))
					y := padding + int(float64(chartHeight)*(1-v/maxVal))
					elements = append(elements, g.Raw(fmt.Sprintf(
						`<circle cx="%d" cy="%d" r="4" fill="%s"/>`,
						x, y, color,
					)))
				}
			}
		}
	}

	// Build grid lines and Y-axis labels
	var grid []g.Node
	if props.ShowGrid {
		for i := 0; i <= 4; i++ {
			y := padding + (chartHeight * i / 4)
			grid = append(grid, g.Raw(fmt.Sprintf(
				`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#3f3f46" stroke-dasharray="4"/>`,
				leftPadding, y, width-padding, y,
			)))

			// Add Y-axis labels if enabled
			if props.ShowYAxis {
				// Value at this grid line (top = maxVal, bottom = 0)
				value := maxVal * float64(4-i) / 4
				label := fmt.Sprintf("%.0f", value)
				if props.YAxisFormat != nil {
					label = props.YAxisFormat(value)
				}
				grid = append(grid, g.Raw(fmt.Sprintf(
					`<text x="%d" y="%d" text-anchor="end" dominant-baseline="middle" class="fill-muted-foreground text-xs">%s</text>`,
					leftPadding-8, y, label,
				)))
			}
		}
	}

	// Build X-axis labels
	var labels []g.Node
	if len(props.Labels) > 0 {
		for i, label := range props.Labels {
			var x int
			if len(props.Labels) == 1 {
				x = leftPadding + chartWidth/2
			} else {
				step := chartWidth / (len(props.Labels) - 1)
				x = leftPadding + (step * i)
			}
			labels = append(labels, g.Raw(fmt.Sprintf(
				`<text x="%d" y="%d" text-anchor="middle" class="fill-muted-foreground text-xs">%s</text>`,
				x, height-10, label,
			)))
		}
	}

	return h.Div(
		h.Class(Cn("relative", props.Class)),
		g.Raw(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height)),
		g.Group(grid),
		g.Group(elements),
		g.Group(labels),
		g.Raw(`</svg>`),
		g.If(props.ShowLegend, chartLegend(props.Series)),
	)
}

func buildLinePath(data []float64, chartWidth, chartHeight, leftPadding, topPadding int, maxVal float64) string {
	if len(data) == 0 {
		return ""
	}
	if len(data) == 1 {
		x := leftPadding
		y := topPadding + int(float64(chartHeight)*(1-data[0]/maxVal))
		return fmt.Sprintf("M %d %d L %d %d", x, y, x+1, y)
	}

	step := float64(chartWidth) / float64(len(data)-1)
	var path strings.Builder

	for i, v := range data {
		x := leftPadding + int(step*float64(i))
		y := topPadding + int(float64(chartHeight)*(1-v/maxVal))
		if i == 0 {
			path.WriteString(fmt.Sprintf("M %d %d ", x, y))
		} else {
			path.WriteString(fmt.Sprintf("L %d %d ", x, y))
		}
	}

	return path.String()
}

// Pie Chart
type PieChartProps struct {
	ChartProps
	Data       []ChartDataPoint
	ShowLegend bool
	Donut      bool
	DonutWidth int // For donut charts
}

func PieChart(props PieChartProps) g.Node {
	size := props.Width
	if size == 0 {
		size = 200
	}

	center := size / 2
	radius := (size - 20) / 2
	innerRadius := 0
	if props.Donut {
		innerRadius = props.DonutWidth
		if innerRadius == 0 {
			innerRadius = radius / 2
		}
	}

	// Calculate total
	total := 0.0
	for _, d := range props.Data {
		total += d.Value
	}

	var segments []g.Node
	startAngle := -90.0 // Start from top

	for i, d := range props.Data {
		if d.Value == 0 {
			continue
		}

		color := getChartColor(i, d.Color)
		sweepAngle := (d.Value / total) * 360

		path := buildArcPath(center, center, radius, innerRadius, startAngle, sweepAngle)
		segments = append(segments, g.Raw(fmt.Sprintf(
			`<path d="%s" fill="%s"/>`,
			path, color,
		)))

		startAngle += sweepAngle
	}

	return h.Div(
		h.Class(Cn("relative flex items-center gap-4", props.Class)),
		g.Raw(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d">`, size, size, size, size)),
		g.Group(segments),
		g.Raw(`</svg>`),
		g.If(props.ShowLegend, pieLegend(props.Data)),
	)
}

func buildArcPath(cx, cy, outerRadius, innerRadius int, startAngle, sweepAngle float64) string {
	// Convert angles to radians
	startRad := startAngle * math.Pi / 180
	endRad := (startAngle + sweepAngle) * math.Pi / 180

	// Calculate points
	x1 := float64(cx) + float64(outerRadius)*math.Cos(startRad)
	y1 := float64(cy) + float64(outerRadius)*math.Sin(startRad)
	x2 := float64(cx) + float64(outerRadius)*math.Cos(endRad)
	y2 := float64(cy) + float64(outerRadius)*math.Sin(endRad)

	largeArc := 0
	if sweepAngle > 180 {
		largeArc = 1
	}

	if innerRadius == 0 {
		// Pie slice
		return fmt.Sprintf("M %d %d L %.2f %.2f A %d %d 0 %d 1 %.2f %.2f Z",
			cx, cy, x1, y1, outerRadius, outerRadius, largeArc, x2, y2)
	}

	// Donut segment
	ix1 := float64(cx) + float64(innerRadius)*math.Cos(startRad)
	iy1 := float64(cy) + float64(innerRadius)*math.Sin(startRad)
	ix2 := float64(cx) + float64(innerRadius)*math.Cos(endRad)
	iy2 := float64(cy) + float64(innerRadius)*math.Sin(endRad)

	return fmt.Sprintf("M %.2f %.2f A %d %d 0 %d 1 %.2f %.2f L %.2f %.2f A %d %d 0 %d 0 %.2f %.2f Z",
		x1, y1, outerRadius, outerRadius, largeArc, x2, y2,
		ix2, iy2, innerRadius, innerRadius, largeArc, ix1, iy1)
}

// Radar Chart
type RadarChartProps struct {
	ChartProps
	Labels   []string
	Series   []ChartSeries
	MaxValue float64
}

func RadarChart(props RadarChartProps) g.Node {
	size := props.Width
	if size == 0 {
		size = 300
	}

	center := size / 2
	radius := (size - 60) / 2
	pointCount := len(props.Labels)

	maxVal := props.MaxValue
	if maxVal == 0 {
		maxVal = 100
	}

	// Build grid
	var grid []g.Node
	for level := 1; level <= 4; level++ {
		r := radius * level / 4
		points := buildPolygonPoints(center, center, r, pointCount)
		grid = append(grid, g.Raw(fmt.Sprintf(
			`<polygon points="%s" fill="none" stroke="#3f3f46"/>`,
			points,
		)))
	}

	// Build axis lines
	angleStep := 360.0 / float64(pointCount)
	for i := 0; i < pointCount; i++ {
		angle := (float64(i)*angleStep - 90) * math.Pi / 180
		x := float64(center) + float64(radius)*math.Cos(angle)
		y := float64(center) + float64(radius)*math.Sin(angle)
		grid = append(grid, g.Raw(fmt.Sprintf(
			`<line x1="%d" y1="%d" x2="%.2f" y2="%.2f" stroke="#3f3f46"/>`,
			center, center, x, y,
		)))
	}

	// Build data polygons
	var polygons []g.Node
	for i, series := range props.Series {
		color := getChartColor(i, series.Color)
		points := buildDataPolygonPoints(center, center, radius, series.Data, maxVal)
		polygons = append(polygons, g.Raw(fmt.Sprintf(
			`<polygon points="%s" fill="%s" fill-opacity="0.2" stroke="%s" stroke-width="2"/>`,
			points, color, color,
		)))
	}

	// Build labels
	var labels []g.Node
	for i, label := range props.Labels {
		angle := (float64(i)*angleStep - 90) * math.Pi / 180
		x := float64(center) + float64(radius+20)*math.Cos(angle)
		y := float64(center) + float64(radius+20)*math.Sin(angle)
		labels = append(labels, g.Raw(fmt.Sprintf(
			`<text x="%.2f" y="%.2f" text-anchor="middle" dominant-baseline="middle" class="fill-muted-foreground text-xs">%s</text>`,
			x, y, label,
		)))
	}

	return h.Div(
		h.Class(Cn("relative", props.Class)),
		g.Raw(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d">`, size, size, size, size)),
		g.Group(grid),
		g.Group(polygons),
		g.Group(labels),
		g.Raw(`</svg>`),
	)
}

func buildPolygonPoints(cx, cy, r, pointCount int) string {
	var points strings.Builder
	angleStep := 360.0 / float64(pointCount)
	for i := 0; i < pointCount; i++ {
		angle := (float64(i)*angleStep - 90) * math.Pi / 180
		x := float64(cx) + float64(r)*math.Cos(angle)
		y := float64(cy) + float64(r)*math.Sin(angle)
		if i > 0 {
			points.WriteString(" ")
		}
		points.WriteString(fmt.Sprintf("%.2f,%.2f", x, y))
	}
	return points.String()
}

func buildDataPolygonPoints(cx, cy, maxRadius int, data []float64, maxVal float64) string {
	var points strings.Builder
	angleStep := 360.0 / float64(len(data))
	for i, v := range data {
		r := float64(maxRadius) * (v / maxVal)
		angle := (float64(i)*angleStep - 90) * math.Pi / 180
		x := float64(cx) + r*math.Cos(angle)
		y := float64(cy) + r*math.Sin(angle)
		if i > 0 {
			points.WriteString(" ")
		}
		points.WriteString(fmt.Sprintf("%.2f,%.2f", x, y))
	}
	return points.String()
}

// Radial/Progress Chart
type RadialChartProps struct {
	ChartProps
	Value     float64
	Max       float64
	Label     string
	ShowValue bool
	Color     string
}

func RadialChart(props RadialChartProps) g.Node {
	size := props.Width
	if size == 0 {
		size = 120
	}

	center := size / 2
	radius := (size - 20) / 2
	strokeWidth := 12

	maxVal := props.Max
	if maxVal == 0 {
		maxVal = 100
	}

	percentage := props.Value / maxVal
	circumference := 2 * math.Pi * float64(radius)
	strokeDashoffset := circumference * (1 - percentage)

	color := props.Color
	if color == "" {
		color = "#6366f1"
	}

	return h.Div(
		h.Class(Cn("relative inline-flex items-center justify-center", props.Class)),
		g.Raw(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d" class="-rotate-90">`, size, size, size, size)),
		// Background circle
		g.Raw(fmt.Sprintf(
			`<circle cx="%d" cy="%d" r="%d" fill="none" stroke="#3f3f46" stroke-width="%d"/>`,
			center, center, radius, strokeWidth,
		)),
		// Progress circle
		g.Raw(fmt.Sprintf(
			`<circle cx="%d" cy="%d" r="%d" fill="none" stroke="%s" stroke-width="%d" stroke-linecap="round" stroke-dasharray="%.2f" stroke-dashoffset="%.2f"/>`,
			center, center, radius, color, strokeWidth, circumference, strokeDashoffset,
		)),
		g.Raw(`</svg>`),
		// Center label
		g.If(props.ShowValue || props.Label != "",
			h.Div(
				h.Class("absolute inset-0 flex flex-col items-center justify-center"),
				g.If(props.ShowValue,
					h.Span(h.Class("text-2xl font-bold text-foreground"), g.Textf("%.0f%%", percentage*100)),
				),
				g.If(props.Label != "",
					h.Span(h.Class("text-xs text-muted-foreground"), g.Text(props.Label)),
				),
			),
		),
	)
}

// Chart Legends
func chartLegend(series []ChartSeries) g.Node {
	items := make([]g.Node, len(series))
	for i, s := range series {
		color := getChartColor(i, s.Color)
		items[i] = h.Div(
			h.Class("flex items-center gap-2"),
			h.Span(
				h.Class("w-3 h-3 rounded-sm"),
				h.Style("background-color: "+color),
			),
			h.Span(h.Class("text-sm text-muted-foreground"), g.Text(s.Name)),
		)
	}
	return h.Div(
		h.Class("flex flex-wrap gap-4 mt-4 justify-center"),
		g.Group(items),
	)
}

func pieLegend(data []ChartDataPoint) g.Node {
	items := make([]g.Node, len(data))
	for i, d := range data {
		color := getChartColor(i, d.Color)
		items[i] = h.Div(
			h.Class("flex items-center gap-2"),
			h.Span(
				h.Class("w-3 h-3 rounded-sm"),
				h.Style("background-color: "+color),
			),
			h.Span(h.Class("text-sm text-muted-foreground"), g.Text(d.Label)),
		)
	}
	return h.Div(
		h.Class("flex flex-col gap-2"),
		g.Group(items),
	)
}

// Chart Container
// ChartContainer wraps a chart with a card and optional title
func ChartContainer(title, description string, chart g.Node) g.Node {
	return Card(CardProps{},
		CardHeader(
			CardTitle(g.Text(title)),
			g.If(description != "", CardDescription(g.Text(description))),
		),
		CardContent(chart),
	)
}
