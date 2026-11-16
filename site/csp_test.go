package site

import (
	"io/fs"
	"regexp"
	"testing"
)

// The site sends script-src 'self', so a browser blocks an inline event handler: a template must attach its
// handlers from a script file instead.
var inlineHandler = regexp.MustCompile(`\son[a-z]+\s*=\s*["']`)

func TestTemplatesHaveNoInlineHandlers(t *testing.T) {
	files, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil || len(files) == 0 {
		t.Fatalf("templates: %v", err)
	}
	for _, f := range files {
		b, err := fs.ReadFile(templateFS, f)
		if err != nil {
			t.Fatal(err)
		}
		if m := inlineHandler.Find(b); m != nil {
			t.Errorf("%s has an inline event handler %q", f, m)
		}
	}
}
