// Package spec holds the specification of the project. The site build publishes it at /spec/.
package spec

import "embed"

// Files holds the parts of the specification, one Markdown file for each part.
//
//go:embed *.md
var Files embed.FS
