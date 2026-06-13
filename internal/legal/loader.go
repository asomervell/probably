package legal

import (
	"embed"
)

//go:embed policies/*.md
var policiesFS embed.FS

// ReadPolicy reads a policy file from the embedded filesystem
func ReadPolicy(filename string) ([]byte, error) {
	return policiesFS.ReadFile("policies/" + filename)
}
