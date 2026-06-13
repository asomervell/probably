package blog

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	g "maragu.dev/gomponents"
)

//go:embed posts/*.md
var postsFS embed.FS

// Post represents a single blog post
type Post struct {
	Slug        string
	Title       string
	Subtitle    string
	Date        time.Time
	ReadingTime string
	Content     g.Node // Rendered HTML as gomponents node
}

var (
	postsCache []Post
	postsDir   = "posts"
)

// markdownRenderer is configured goldmark instance for blog post rendering
var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM, // GitHub Flavored Markdown
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithUnsafe(), // Allow raw HTML in markdown
	),
)

// LoadPosts loads all blog posts from the embedded posts directory
func LoadPosts() ([]Post, error) {
	if postsCache != nil {
		return postsCache, nil
	}

	var posts []Post

	err := fs.WalkDir(postsFS, postsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		post, err := loadPostFromFile(path)
		if err != nil {
			return fmt.Errorf("failed to load post %s: %w", path, err)
		}

		posts = append(posts, post)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by date descending (newest first)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	postsCache = posts
	return posts, nil
}

// loadPostFromFile loads a single post from a markdown file
func loadPostFromFile(path string) (Post, error) {
	data, err := postsFS.ReadFile(path)
	if err != nil {
		return Post{}, err
	}

	// Parse frontmatter
	frontmatter, content, err := parseFrontmatter(string(data))
	if err != nil {
		return Post{}, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Extract metadata
	slug := frontmatter["slug"]
	if slug == "" {
		// Derive slug from filename
		// path is like "posts/foo.md", extract the base name
		parts := strings.Split(path, "/")
		base := parts[len(parts)-1]
		slug = strings.TrimSuffix(base, ".md")
	}

	title := frontmatter["title"]
	subtitle := frontmatter["subtitle"]
	readingTime := frontmatter["reading_time"]
	if readingTime == "" {
		readingTime = frontmatter["readingTime"] // Support camelCase too
	}

	// Parse date
	dateStr := frontmatter["date"]
	var date time.Time
	if dateStr != "" {
		var err error
		// Try RFC3339 first (2026-01-02T00:00:00Z)
		date, err = time.Parse(time.RFC3339, dateStr)
		if err != nil {
			// Try date-only format (2026-01-02)
			date, err = time.Parse("2006-01-02", dateStr)
			if err != nil {
				return Post{}, fmt.Errorf("invalid date format: %s", dateStr)
			}
		}
	} else {
		// Default to current time if no date in frontmatter (embedded files don't have mtime)
		date = time.Now()
	}

	// Render markdown to HTML
	var htmlBuf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(content), &htmlBuf); err != nil {
		return Post{}, fmt.Errorf("failed to render markdown: %w", err)
	}

	// Convert HTML to gomponents node
	htmlContent := htmlBuf.String()
	contentNode := g.Raw(htmlContent)

	return Post{
		Slug:        slug,
		Title:       title,
		Subtitle:    subtitle,
		Date:        date,
		ReadingTime: readingTime,
		Content:     contentNode,
	}, nil
}

// parseFrontmatter parses YAML-style frontmatter from markdown
// Supports both --- and --- delimiters
func parseFrontmatter(content string) (map[string]string, string, error) {
	frontmatter := make(map[string]string)

	// Check for frontmatter delimiter
	if !strings.HasPrefix(content, "---\n") {
		// No frontmatter, return empty map and full content
		return frontmatter, content, nil
	}

	// Find the end of frontmatter
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		// No closing delimiter, treat as no frontmatter
		return frontmatter, content, nil
	}

	// Extract frontmatter block
	fmBlock := content[4 : endIdx+4]
	body := content[endIdx+9:] // Skip "---\n" + frontmatter + "\n---\n"

	// Parse key-value pairs (simple YAML-like parsing)
	lines := strings.Split(fmBlock, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse "key: value" format
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			frontmatter[key] = value
		}
	}

	return frontmatter, body, nil
}

// GetPostBySlug returns a post by its slug
func GetPostBySlug(slug string) (*Post, error) {
	posts, err := LoadPosts()
	if err != nil {
		return nil, err
	}

	for i := range posts {
		if posts[i].Slug == slug {
			return &posts[i], nil
		}
	}

	return nil, fmt.Errorf("post not found: %s", slug)
}

