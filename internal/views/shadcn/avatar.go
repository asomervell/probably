package shadcn

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Avatar Sizes
type AvatarSize string

const (
	AvatarSizeXs AvatarSize = "xs"
	AvatarSizeSm AvatarSize = "sm"
	AvatarSizeMd AvatarSize = "md"
	AvatarSizeLg AvatarSize = "lg"
	AvatarSizeXl AvatarSize = "xl"
)

var avatarSizeClasses = map[AvatarSize]string{
	AvatarSizeXs: "h-6 w-6 text-xs",
	AvatarSizeSm: "h-8 w-8 text-sm",
	AvatarSizeMd: "h-10 w-10 text-base",
	AvatarSizeLg: "h-12 w-12 text-lg",
	AvatarSizeXl: "h-16 w-16 text-xl",
}

// Avatar
type AvatarProps struct {
	Size  AvatarSize
	Class string
}

func Avatar(props AvatarProps, children ...g.Node) g.Node {
	size := props.Size
	if size == "" {
		size = AvatarSizeMd
	}

	classes := Cn(
		"relative flex shrink-0 overflow-hidden rounded-full",
		avatarSizeClasses[size],
		props.Class,
	)

	return h.Span(
		h.Class(classes),
		g.Group(children),
	)
}

func AvatarImage(src, alt string) g.Node {
	return h.Img(
		h.Src(src),
		h.Alt(alt),
		h.Class("aspect-square h-full w-full object-cover"),
	)
}

func AvatarFallback(children ...g.Node) g.Node {
	return h.Span(
		h.Class("flex h-full w-full items-center justify-center rounded-full bg-muted text-foreground font-medium"),
		g.Group(children),
	)
}

func AvatarFallbackColored(color string, children ...g.Node) g.Node {
	return h.Span(
		h.Class("flex h-full w-full items-center justify-center rounded-full font-medium text-white"),
		h.Style("background-color: "+color),
		g.Group(children),
	)
}

// Avatar Helpers
func AvatarWithInitials(props AvatarProps, src, alt, initials string) g.Node {
	return Avatar(props,
		g.If(src != "",
			AvatarImage(src, alt),
		),
		g.If(src == "",
			AvatarFallback(g.Text(initials)),
		),
	)
}

func AvatarWithStatus(props AvatarProps, status string, children ...g.Node) g.Node {
	statusColor := "bg-muted-foreground"
	switch status {
	case "online":
		statusColor = "bg-green-500"
	case "offline":
		statusColor = "bg-muted-foreground"
	case "busy":
		statusColor = "bg-red-500"
	case "away":
		statusColor = "bg-amber-500"
	}

	size := props.Size
	if size == "" {
		size = AvatarSizeMd
	}

	dotSize := "h-2.5 w-2.5"
	if size == AvatarSizeLg || size == AvatarSizeXl {
		dotSize = "h-3 w-3"
	} else if size == AvatarSizeXs || size == AvatarSizeSm {
		dotSize = "h-2 w-2"
	}

	return h.Span(
		h.Class("relative inline-block"),
		Avatar(props, children...),
		h.Span(
			h.Class(Cn(
				"absolute bottom-0 right-0 block rounded-full ring-2 ring-background",
				statusColor,
				dotSize,
			)),
		),
	)
}

// Avatar Group
type AvatarGroupProps struct {
	Max   int
	Size  AvatarSize
	Class string
}

func AvatarGroup(props AvatarGroupProps, avatars []g.Node) g.Node {
	max := props.Max
	if max == 0 {
		max = 5
	}

	size := props.Size
	if size == "" {
		size = AvatarSizeMd
	}

	displayed := avatars
	overflow := 0
	if len(avatars) > max {
		displayed = avatars[:max]
		overflow = len(avatars) - max
	}

	nodes := make([]g.Node, len(displayed))
	for i, avatar := range displayed {
		nodes[i] = h.Div(
			h.Class("-ml-2 first:ml-0 ring-2 ring-background rounded-full"),
			avatar,
		)
	}

	if overflow > 0 {
		nodes = append(nodes, h.Div(
			h.Class("-ml-2 ring-2 ring-background rounded-full"),
			Avatar(AvatarProps{Size: size},
				AvatarFallback(g.Textf("+%d", overflow)),
			),
		))
	}

	return h.Div(
		h.Class(Cn("flex items-center", props.Class)),
		g.Group(nodes),
	)
}

// User Avatar Helper
type UserAvatarProps struct {
	Name   string
	Email  string
	Image  string
	Size   AvatarSize
	Status string
}

// GetInitials extracts initials from a name
func GetInitials(name string) string {
	if name == "" {
		return "?"
	}

	initials := ""
	words := make([]string, 0)
	current := ""

	for _, r := range name {
		if r == ' ' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		words = append(words, current)
	}

	if len(words) >= 2 {
		initials = string([]rune(words[0])[0]) + string([]rune(words[len(words)-1])[0])
	} else if len(words) == 1 && len(words[0]) >= 2 {
		initials = string([]rune(words[0])[:2])
	} else if len(words) == 1 {
		initials = string([]rune(words[0])[0])
	}

	return initials
}

func UserAvatar(props UserAvatarProps) g.Node {
	initials := GetInitials(props.Name)
	avatarProps := AvatarProps{Size: props.Size}

	if props.Status != "" {
		return AvatarWithStatus(avatarProps, props.Status,
			g.If(props.Image != "", AvatarImage(props.Image, props.Name)),
			g.If(props.Image == "", AvatarFallback(g.Text(initials))),
		)
	}

	return Avatar(avatarProps,
		g.If(props.Image != "", AvatarImage(props.Image, props.Name)),
		g.If(props.Image == "", AvatarFallback(g.Text(initials))),
	)
}

func UserAvatarWithName(props UserAvatarProps) g.Node {
	return h.Div(
		h.Class("flex items-center gap-3"),
		UserAvatar(props),
		h.Div(
			h.Class("flex flex-col"),
			h.Span(h.Class("text-sm font-medium text-foreground"), g.Text(props.Name)),
			g.If(props.Email != "",
				h.Span(h.Class("text-xs text-muted-foreground"), g.Text(props.Email)),
			),
		),
	)
}
