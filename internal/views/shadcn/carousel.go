package shadcn

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Carousel
// CarouselOrientation represents the carousel orientation
type CarouselOrientation string

const (
	CarouselHorizontal CarouselOrientation = "horizontal"
	CarouselVertical   CarouselOrientation = "vertical"
)

type CarouselProps struct {
	ID          string
	Orientation CarouselOrientation
	Loop        bool
	AutoPlay    bool
	Interval    int // Autoplay interval in milliseconds
	Class       string
}

func Carousel(props CarouselProps, children ...g.Node) g.Node {
	id := props.ID
	if id == "" {
		id = "carousel"
	}

	orientation := props.Orientation
	if orientation == "" {
		orientation = CarouselHorizontal
	}

	interval := props.Interval
	if interval == 0 {
		interval = 5000
	}

	return h.Div(
		h.ID(id),
		h.Class(Cn("relative", props.Class)),
		g.Attr("data-carousel", id),
		g.Attr("data-orientation", string(orientation)),
		g.Attr("data-loop", boolToString(props.Loop)),
		g.If(props.AutoPlay, g.Attr("data-autoplay", fmt.Sprintf("%d", interval))),
		Role("region"),
		AriaLabel("carousel"),
		g.Group(children),
	)
}

func CarouselContent(carouselID string, children ...g.Node) g.Node {
	return h.Div(
		h.Class("overflow-hidden"),
		h.Div(
			h.ID(carouselID+"-content"),
			h.Class("flex transition-transform duration-300 ease-in-out"),
			g.Attr("data-carousel-content", carouselID),
			g.Group(children),
		),
	)
}

func CarouselItem(children ...g.Node) g.Node {
	return h.Div(
		h.Class("min-w-0 shrink-0 grow-0 basis-full pl-4"),
		Role("group"),
		g.Attr("aria-roledescription", "slide"),
		g.Attr("data-carousel-item", ""),
		g.Group(children),
	)
}

func CarouselItemMultiple(basis string, children ...g.Node) g.Node {
	basisClass := "basis-full"
	switch basis {
	case "1/2":
		basisClass = "basis-1/2"
	case "1/3":
		basisClass = "basis-1/3"
	case "1/4":
		basisClass = "basis-1/4"
	case "2/3":
		basisClass = "basis-2/3"
	case "3/4":
		basisClass = "basis-3/4"
	}

	return h.Div(
		h.Class(Cn("min-w-0 shrink-0 grow-0 pl-4", basisClass)),
		Role("group"),
		g.Attr("aria-roledescription", "slide"),
		g.Attr("data-carousel-item", ""),
		g.Group(children),
	)
}

func CarouselPrevious(carouselID string) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("absolute left-4 top-1/2 -translate-y-1/2 inline-flex items-center justify-center rounded-full w-8 h-8 border border-border bg-card shadow-sm hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed z-10"),
		g.Attr("onclick", "carouselPrev('"+carouselID+"')"),
		AriaLabel("Previous slide"),
		IconChevronLeft(),
	)
}

func CarouselNext(carouselID string) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("absolute right-4 top-1/2 -translate-y-1/2 inline-flex items-center justify-center rounded-full w-8 h-8 border border-border bg-card shadow-sm hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed z-10"),
		g.Attr("onclick", "carouselNext('"+carouselID+"')"),
		AriaLabel("Next slide"),
		IconChevronRight(),
	)
}

func CarouselDots(carouselID string, count int, currentIndex int) g.Node {
	dots := make([]g.Node, count)
	for i := 0; i < count; i++ {
		isActive := i == currentIndex
		dotClass := "w-2 h-2 rounded-full transition-colors bg-muted-foreground hover:bg-foreground"
		if isActive {
			dotClass = "w-2 h-2 rounded-full transition-colors bg-foreground"
		}
		attrs := []g.Node{
			h.Type("button"),
			h.Class(dotClass),
			g.Attr("onclick", fmt.Sprintf("carouselGoTo('%s', %d)", carouselID, i)),
			AriaLabel(fmt.Sprintf("Go to slide %d", i+1)),
		}
		if isActive {
			attrs = append(attrs, g.Attr("aria-current", "true"))
		}
		dots[i] = h.Button(attrs...)
	}

	return h.Div(
		h.ID(carouselID+"-dots"),
		h.Class("flex justify-center gap-2 mt-4"),
		g.Attr("data-carousel-dots", carouselID),
		g.Group(dots),
	)
}

// CarouselScript returns the JavaScript for carousel functionality
func CarouselScript() g.Node {
	return g.Raw(`<script>
(function() {
    const carousels = {};
    
    document.querySelectorAll('[data-carousel]').forEach(carousel => {
        const id = carousel.id;
        const content = document.getElementById(id + '-content');
        const items = content.querySelectorAll('[data-carousel-item]');
        const loop = carousel.getAttribute('data-loop') === 'true';
        const autoplay = carousel.getAttribute('data-autoplay');
        
        carousels[id] = {
            currentIndex: 0,
            itemCount: items.length,
            loop: loop
        };
        
        // Initialize autoplay
        if (autoplay) {
            setInterval(() => carouselNext(id), parseInt(autoplay));
        }
    });
    
    window.carouselPrev = function(id) {
        const state = carousels[id];
        if (!state) return;
        
        if (state.currentIndex > 0) {
            state.currentIndex--;
        } else if (state.loop) {
            state.currentIndex = state.itemCount - 1;
        }
        
        updateCarousel(id);
    };
    
    window.carouselNext = function(id) {
        const state = carousels[id];
        if (!state) return;
        
        if (state.currentIndex < state.itemCount - 1) {
            state.currentIndex++;
        } else if (state.loop) {
            state.currentIndex = 0;
        }
        
        updateCarousel(id);
    };
    
    window.carouselGoTo = function(id, index) {
        const state = carousels[id];
        if (!state) return;
        
        state.currentIndex = Math.max(0, Math.min(index, state.itemCount - 1));
        updateCarousel(id);
    };
    
    function updateCarousel(id) {
        const state = carousels[id];
        const content = document.getElementById(id + '-content');
        const dots = document.getElementById(id + '-dots');
        
        if (content) {
            const translateX = -state.currentIndex * 100;
            content.style.transform = 'translateX(' + translateX + '%)';
        }
        
        if (dots) {
            const dotButtons = dots.querySelectorAll('button');
            dotButtons.forEach((dot, i) => {
                const isActive = i === state.currentIndex;
                dot.classList.toggle('bg-foreground', isActive);
                dot.classList.toggle('bg-muted-foreground', !isActive);
                dot.classList.toggle('hover:bg-foreground', !isActive);
                if (isActive) {
                    dot.setAttribute('aria-current', 'true');
                } else {
                    dot.removeAttribute('aria-current');
                }
            });
        }
    }
})();
</script>`)
}

// SimpleCarousel creates a complete carousel from a slice of items
func SimpleCarousel(id string, items []g.Node, showControls, showDots bool) g.Node {
	return Carousel(CarouselProps{ID: id},
		g.If(showControls, CarouselPrevious(id)),
		CarouselContent(id,
			g.Group(wrapInCarouselItems(items)),
		),
		g.If(showControls, CarouselNext(id)),
		g.If(showDots, CarouselDots(id, len(items), 0)),
	)
}

func wrapInCarouselItems(items []g.Node) []g.Node {
	wrapped := make([]g.Node, len(items))
	for i, item := range items {
		wrapped[i] = CarouselItem(item)
	}
	return wrapped
}

// ImageCarousel creates a carousel specifically for images
func ImageCarousel(id string, images []struct{ Src, Alt string }, showControls, showDots bool) g.Node {
	items := make([]g.Node, len(images))
	for i, img := range images {
		items[i] = h.Img(
			h.Src(img.Src),
			h.Alt(img.Alt),
			h.Class("w-full h-auto rounded-lg object-cover"),
		)
	}
	return SimpleCarousel(id, items, showControls, showDots)
}

// CardCarousel creates a carousel for displaying cards
func CardCarousel(id string, basis string, cards []g.Node, showControls bool) g.Node {
	items := make([]g.Node, len(cards))
	for i, card := range cards {
		items[i] = CarouselItemMultiple(basis, card)
	}

	return Carousel(CarouselProps{ID: id},
		g.If(showControls, CarouselPrevious(id)),
		h.Div(
			h.Class("overflow-hidden"),
			h.Div(
				h.ID(id+"-content"),
				h.Class("flex transition-transform duration-300 ease-in-out -ml-4"),
				g.Attr("data-carousel-content", id),
				g.Group(items),
			),
		),
		g.If(showControls, CarouselNext(id)),
	)
}
