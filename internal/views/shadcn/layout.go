package shadcn

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Separator
type SeparatorOrientation string

const (
	SeparatorHorizontal SeparatorOrientation = "horizontal"
	SeparatorVertical   SeparatorOrientation = "vertical"
)

type SeparatorProps struct {
	Orientation SeparatorOrientation
	Class       string
	Decorative  bool
}

func Separator(props SeparatorProps) g.Node {
	orientation := props.Orientation
	if orientation == "" {
		orientation = SeparatorHorizontal
	}

	baseClass := "shrink-0 bg-muted"
	orientationClass := ""
	if orientation == SeparatorHorizontal {
		orientationClass = "h-px w-full"
	} else {
		orientationClass = "h-full w-px"
	}

	attrs := []g.Node{
		h.Class(Cn(baseClass, orientationClass, props.Class)),
		DataOrientation(string(orientation)),
	}

	if props.Decorative {
		attrs = append(attrs, Role("none"))
	} else {
		attrs = append(attrs, Role("separator"))
		attrs = append(attrs, AriaOrientation(string(orientation)))
	}

	return h.Div(attrs...)
}

func SeparatorWithText(text string) g.Node {
	return h.Div(
		h.Class("relative"),
		h.Div(h.Class("absolute inset-0 flex items-center"),
			h.Span(h.Class("w-full border-t border-border")),
		),
		h.Div(h.Class("relative flex justify-center text-xs uppercase"),
			h.Span(h.Class("bg-background px-2 text-muted-foreground"), g.Text(text)),
		),
	)
}

// Aspect Ratio
type AspectRatioProps struct {
	Ratio float64 // width/height ratio (e.g., 16/9 = 1.777)
	Class string
}

func AspectRatio(props AspectRatioProps, children ...g.Node) g.Node {
	ratio := props.Ratio
	if ratio == 0 {
		ratio = 1 // Default to square
	}

	// Calculate padding-bottom percentage (100 / ratio)
	paddingPercent := 100 / ratio

	return h.Div(
		h.Class(Cn("relative w-full", props.Class)),
		h.Style(fmt.Sprintf("padding-bottom: %.4f%%", paddingPercent)),
		h.Div(
			h.Class("absolute inset-0"),
			g.Group(children),
		),
	)
}

// Common aspect ratios
const (
	AspectRatioSquare   = 1.0
	AspectRatioVideo    = 16.0 / 9.0
	AspectRatioPortrait = 3.0 / 4.0
	AspectRatioWide     = 21.0 / 9.0
	AspectRatioStandard = 4.0 / 3.0
)

func AspectRatioVideoContainer(children ...g.Node) g.Node {
	return h.Div(
		h.Class("aspect-video relative"),
		g.Group(children),
	)
}

func AspectRatioSquareContainer(children ...g.Node) g.Node {
	return h.Div(
		h.Class("aspect-square relative"),
		g.Group(children),
	)
}

// Scroll Area
type ScrollAreaProps struct {
	Class       string
	Orientation string // "vertical", "horizontal", or "both"
}

func ScrollArea(props ScrollAreaProps, children ...g.Node) g.Node {
	orientation := props.Orientation
	if orientation == "" {
		orientation = "vertical"
	}

	overflowClass := "overflow-y-auto"
	switch orientation {
	case "horizontal":
		overflowClass = "overflow-x-auto overflow-y-hidden"
	case "both":
		overflowClass = "overflow-auto"
	}

	// Custom scrollbar styling using Tailwind classes
	scrollbarClass := "[&::-webkit-scrollbar]:w-2 [&::-webkit-scrollbar-track]:bg-muted/50 [&::-webkit-scrollbar-thumb]:bg-muted [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:border-2 [&::-webkit-scrollbar-thumb]:border-transparent hover:[&::-webkit-scrollbar-thumb]:bg-border"

	return h.Div(
		h.Class(Cn("relative", overflowClass, scrollbarClass, props.Class)),
		g.Attr("data-orientation", orientation),
		g.Group(children),
	)
}

// ScrollAreaViewport is an alias for the scroll area content wrapper
func ScrollAreaViewport(children ...g.Node) g.Node {
	return h.Div(
		h.Class("h-full w-full"),
		g.Group(children),
	)
}

// Resizable
type ResizableProps struct {
	ID        string
	Direction string // "horizontal" or "vertical"
	Class     string
}

func ResizablePanelGroup(props ResizableProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "resizable"
	}

	direction := props.Direction
	if direction == "" {
		direction = "horizontal"
	}

	flexDirection := "flex-row"
	if direction == "vertical" {
		flexDirection = "flex-col"
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("flex h-full w-full", flexDirection, props.Class)),
		g.Attr("data-resizable", id),
		g.Attr("data-direction", direction),
		g.Group(children),
	)
}

func ResizablePanel(defaultSize int, minSize int, maxSize int, children ...g.Node) g.Node {
	style := fmt.Sprintf("flex: %d %d 0%%; min-width: %d%%; max-width: %d%%", defaultSize, defaultSize, minSize, maxSize)
	if maxSize == 0 {
		style = fmt.Sprintf("flex: %d %d 0%%; min-width: %d%%", defaultSize, defaultSize, minSize)
	}

	return h.Div(
		h.Class("overflow-hidden"),
		h.Style(style),
		g.Attr("data-panel", ""),
		g.Group(children),
	)
}

func ResizableHandle(groupID string, withHandle bool) g.Node {
	return h.Div(
		h.Class("relative flex w-px items-center justify-center bg-muted after:absolute after:inset-y-0 after:left-1/2 after:w-1 after:-translate-x-1/2 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring focus-visible:ring-offset-1 cursor-col-resize hover:bg-border transition-colors"),
		g.Attr("data-resize-handle", groupID),
		g.If(withHandle,
			h.Div(
				h.Class("z-10 flex h-4 w-3 items-center justify-center rounded-sm border border-border bg-muted"),
				IconGripVertical(),
			),
		),
	)
}

// ResizableScript returns the JavaScript for resizable panels
func ResizableScript() g.Node {
	return g.Raw(`<script>
(function() {
    document.querySelectorAll('[data-resize-handle]').forEach(handle => {
        let isResizing = false;
        let startX = 0;
        let startWidths = [];
        
        handle.addEventListener('mousedown', function(e) {
            isResizing = true;
            startX = e.clientX;
            
            const group = handle.closest('[data-resizable]');
            const panels = group.querySelectorAll('[data-panel]');
            startWidths = Array.from(panels).map(p => p.offsetWidth);
            
            document.body.style.cursor = 'col-resize';
            document.body.style.userSelect = 'none';
        });
        
        document.addEventListener('mousemove', function(e) {
            if (!isResizing) return;
            
            const group = handle.closest('[data-resizable]');
            const panels = Array.from(group.querySelectorAll('[data-panel]'));
            const handleIndex = Array.from(group.children).indexOf(handle);
            const leftPanel = panels[Math.floor(handleIndex / 2)];
            const rightPanel = panels[Math.floor(handleIndex / 2) + 1];
            
            if (!leftPanel || !rightPanel) return;
            
            const diff = e.clientX - startX;
            const leftIndex = Math.floor(handleIndex / 2);
            const rightIndex = leftIndex + 1;
            
            const newLeftWidth = startWidths[leftIndex] + diff;
            const newRightWidth = startWidths[rightIndex] - diff;
            
            const totalWidth = group.offsetWidth;
            const leftPercent = (newLeftWidth / totalWidth) * 100;
            const rightPercent = (newRightWidth / totalWidth) * 100;
            
            if (leftPercent > 10 && rightPercent > 10) {
                leftPanel.style.flex = leftPercent + ' ' + leftPercent + ' 0%';
                rightPanel.style.flex = rightPercent + ' ' + rightPercent + ' 0%';
            }
        });
        
        document.addEventListener('mouseup', function() {
            if (isResizing) {
                isResizing = false;
                document.body.style.cursor = '';
                document.body.style.userSelect = '';
            }
        });
    });
})();
</script>`)
}

// Container / Layout Utilities
func Container(maxWidth string, children ...g.Node) g.Node {
	widthClass := "max-w-7xl"
	switch maxWidth {
	case "sm":
		widthClass = "max-w-sm"
	case "md":
		widthClass = "max-w-md"
	case "lg":
		widthClass = "max-w-lg"
	case "xl":
		widthClass = "max-w-xl"
	case "2xl":
		widthClass = "max-w-2xl"
	case "3xl":
		widthClass = "max-w-3xl"
	case "4xl":
		widthClass = "max-w-4xl"
	case "5xl":
		widthClass = "max-w-5xl"
	case "6xl":
		widthClass = "max-w-6xl"
	case "7xl":
		widthClass = "max-w-7xl"
	case "full":
		widthClass = "max-w-full"
	case "prose":
		widthClass = "max-w-prose"
	}

	return h.Div(
		h.Class(Cn("mx-auto w-full px-4 sm:px-6 lg:px-8", widthClass)),
		g.Group(children),
	)
}

func Stack(gap string, children ...g.Node) g.Node {
	gapClass := "gap-4"
	switch gap {
	case "0":
		gapClass = "gap-0"
	case "1":
		gapClass = "gap-1"
	case "2":
		gapClass = "gap-2"
	case "3":
		gapClass = "gap-3"
	case "4":
		gapClass = "gap-4"
	case "6":
		gapClass = "gap-6"
	case "8":
		gapClass = "gap-8"
	}

	return h.Div(
		h.Class(Cn("flex flex-col", gapClass)),
		g.Group(children),
	)
}

func HStack(gap string, children ...g.Node) g.Node {
	gapClass := "gap-4"
	switch gap {
	case "0":
		gapClass = "gap-0"
	case "1":
		gapClass = "gap-1"
	case "2":
		gapClass = "gap-2"
	case "3":
		gapClass = "gap-3"
	case "4":
		gapClass = "gap-4"
	case "6":
		gapClass = "gap-6"
	case "8":
		gapClass = "gap-8"
	}

	return h.Div(
		h.Class(Cn("flex flex-row items-center", gapClass)),
		g.Group(children),
	)
}

func Grid(cols int, gap string, children ...g.Node) g.Node {
	colsClass := "grid-cols-1"
	switch cols {
	case 2:
		colsClass = "grid-cols-2"
	case 3:
		colsClass = "grid-cols-3"
	case 4:
		colsClass = "grid-cols-4"
	case 5:
		colsClass = "grid-cols-5"
	case 6:
		colsClass = "grid-cols-6"
	case 12:
		colsClass = "grid-cols-12"
	}

	gapClass := "gap-4"
	switch gap {
	case "0":
		gapClass = "gap-0"
	case "2":
		gapClass = "gap-2"
	case "4":
		gapClass = "gap-4"
	case "6":
		gapClass = "gap-6"
	case "8":
		gapClass = "gap-8"
	}

	return h.Div(
		h.Class(Cn("grid", colsClass, gapClass)),
		g.Group(children),
	)
}

func Center(children ...g.Node) g.Node {
	return h.Div(
		h.Class("flex items-center justify-center"),
		g.Group(children),
	)
}

func Spacer() g.Node {
	return h.Div(h.Class("flex-1"))
}
