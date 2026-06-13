package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/asomervell/probably/internal/blog"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/go-chi/chi/v5"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// BlogPost represents a single blog post (for backward compatibility)
type BlogPost struct {
	Slug        string
	Title       string
	Subtitle    string
	Date        time.Time
	ReadingTime string
	Content     []g.Node
}

// getBlogPosts loads posts from markdown files
func getBlogPosts() ([]BlogPost, error) {
	// Load from markdown files
	posts, err := blog.LoadPosts()
	if err != nil {
		return nil, fmt.Errorf("failed to load blog posts: %w", err)
	}

	// Convert blog.Post to BlogPost
	result := make([]BlogPost, len(posts))
	for i, p := range posts {
		result[i] = BlogPost{
			Slug:        p.Slug,
			Title:       p.Title,
			Subtitle:    p.Subtitle,
			Date:        p.Date,
			ReadingTime: p.ReadingTime,
			Content:     []g.Node{p.Content}, // Wrap in slice for compatibility
		}
	}
	return result, nil
}

// BlogIndex renders the blog listing page
func (hdl *Handlers) BlogIndex(w http.ResponseWriter, r *http.Request) {
	userEmail, phID := userContext(r)

	posts, err := getBlogPosts()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load blog posts: %v", err), http.StatusInternalServerError)
		return
	}

	page := layouts.MarketingLayout("Latest", userEmail, phID,
		renderBlogIndex(posts),
	)

	renderHTML(w, page)
}

// BlogPost renders a single blog post
func (hdl *Handlers) BlogPost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	// Try markdown loader first
	mdPost, err := blog.GetPostBySlug(slug)
	if err == nil && mdPost != nil {
		userEmail, phID := userContext(r)

		post := BlogPost{
			Slug:        mdPost.Slug,
			Title:       mdPost.Title,
			Subtitle:    mdPost.Subtitle,
			Date:        mdPost.Date,
			ReadingTime: mdPost.ReadingTime,
			Content:     []g.Node{mdPost.Content},
		}

		page := layouts.MarketingLayout(post.Title, userEmail, phID,
			renderBlogPostPage(&post),
		)

		renderHTML(w, page)
		return
	}

	// Fallback to hardcoded posts
	posts, err := getBlogPosts()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load blog posts: %v", err), http.StatusInternalServerError)
		return
	}

	var post *BlogPost
	for i := range posts {
		if posts[i].Slug == slug {
			post = &posts[i]
			break
		}
	}

	if post == nil {
		http.NotFound(w, r)
		return
	}

	userEmail, phID := userContext(r)

	page := layouts.MarketingLayout(post.Title, userEmail, phID,
		renderBlogPostPage(post),
	)

	renderHTML(w, page)
}

func renderBlogIndex(posts []BlogPost) g.Node {
	return h.Section(
		h.Class("pt-32 pb-24 bg-background"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("lg:grid lg:grid-cols-12 lg:gap-16"),
				// Main content - 8 columns, offset by 2 (1/6th indent)
				h.Div(
					h.Class("lg:col-span-8 lg:col-start-3"),
					// Header
					h.Div(
						h.Class("mb-12"),
						h.P(
							h.Class("text-sm font-medium text-primary mb-3"),
							g.Text("Latest"),
						),
						h.H1(
							h.Class("text-3xl font-semibold tracking-tight text-foreground sm:text-4xl mb-4"),
							g.Text("Updates and ideas"),
						),
						h.P(
							h.Class("text-base text-muted-foreground leading-relaxed max-w-xl"),
							g.Text("Thoughts on personal finance, AI, and building tools that actually help."),
						),
					),
					// Posts list
					h.Div(
						h.Class("divide-y divide-border/50"),
						g.Group(g.Map(posts, func(post BlogPost) g.Node {
							return h.Article(
								h.Class("group py-8 first:pt-0"),
								h.A(
									h.Href("/blog/"+post.Slug),
									h.Class("block"),
									h.Time(
										h.Class("text-sm text-muted-foreground tabular-nums"),
										g.Text(post.Date.Format("Jan 2, 2006")),
									),
									h.H2(
										h.Class("text-lg font-medium text-foreground group-hover:text-chart-3 transition-colors mt-2 mb-1"),
										g.Text(post.Title),
									),
									h.P(
										h.Class("text-sm text-muted-foreground"),
										g.Text(post.Subtitle),
									),
								),
							)
						})),
					),
				),
			),
		),
	)
}

func renderBlogPostPage(post *BlogPost) g.Node {
	return h.Article(
		h.Class("pt-32 pb-24 bg-background"),
		h.Div(
			h.Class("max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"),
			h.Div(
				h.Class("lg:grid lg:grid-cols-12 lg:gap-16"),
				// Main content - 8 columns, offset by 2 (1/6th indent)
				h.Div(
					h.Class("lg:col-span-8 lg:col-start-3"),
					// Back link
					h.A(
						h.Href("/blog"),
						h.Class("inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors mb-10"),
						g.Raw(`<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 12H5m7 7-7-7 7-7"/></svg>`),
						g.Text("Latest"),
					),
					// Header
					h.Header(
						h.Class("mb-10"),
						h.Time(
							h.Class("text-sm text-muted-foreground tabular-nums"),
							g.Text(post.Date.Format("Jan 2, 2006")),
						),
						h.H1(
							h.Class("text-2xl sm:text-3xl font-semibold tracking-tight text-foreground mt-3 mb-3"),
							g.Text(post.Title),
						),
						h.P(
							h.Class("text-base text-muted-foreground"),
							g.Text(post.Subtitle),
						),
					),
					// Content
					h.Div(
						h.Class("prose prose-p:text-muted-foreground prose-p:leading-relaxed prose-headings:text-foreground prose-headings:font-medium prose-a:text-primary prose-strong:text-foreground prose-strong:font-semibold max-w-none"),
						g.Group(post.Content),
					),
				),
			),
		),
	)
}
