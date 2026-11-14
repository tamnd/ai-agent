// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"errors"
	"reflect"
	"testing"
)

func TestOWASPFiles(t *testing.T) {
	files, err := OWASPFiles([]byte(`[
		{"name": "LLM00_Preface.md", "path": "2_0_vulns/LLM00_Preface.md", "type": "file"},
		{"name": "LLM02_SensitiveInformationDisclosure.md", "path": "2_0_vulns/LLM02_SensitiveInformationDisclosure.md", "type": "file"},
		{"name": "LLM01_PromptInjection.md", "path": "2_0_vulns/LLM01_PromptInjection.md", "type": "file"},
		{"name": "_template.md", "path": "2_0_vulns/_template.md", "type": "file"},
		{"name": "translations", "path": "2_0_vulns/translations", "type": "dir"}]`))
	want := []string{"2_0_vulns/LLM01_PromptInjection.md", "2_0_vulns/LLM02_SensitiveInformationDisclosure.md"}
	if err != nil || !reflect.DeepEqual(files, want) {
		t.Errorf("got %v, %v", files, err)
	}
	var e *ErrSchema
	for _, bad := range []string{`{"message": "Not Found"}`, `[]`, `[{"name": "README.md", "path": "README.md", "type": "file"}]`} {
		if _, err := OWASPFiles([]byte(bad)); !errors.As(err, &e) {
			t.Errorf("%s: %v", bad, err)
		}
	}
}

func TestOWASPRow(t *testing.T) {
	r25, r23 := OWASPReleases[0], OWASPReleases[1]
	a, err := OWASPRow(r25, "2_0_vulns/LLM01_PromptInjection.md", []byte("## LLM01:2025 Prompt Injection\n\n### Description\n\nA Prompt Injection Vulnerability occurs when...\n"), "b3:x")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "LLM01:2025" || a.Name != "Prompt Injection" || a.Kind != "risk" || !reflect.DeepEqual(a.SameAs, map[string]string{"owasp_llm_2025": "LLM01:2025"}) ||
		a.Facts["release"].V != "2025" || a.Facts["rank"].V != 1 || a.Snapshot != "b3:x" || a.Fields != nil ||
		a.URL != "https://github.com/OWASP/www-project-top-10-for-large-language-model-applications/blob/main/2_0_vulns/LLM01_PromptInjection.md" {
		t.Errorf("the 2025 entry is %+v", a)
	}
	b, err := OWASPRow(r23, "Archive/1_1_vulns/LLM01_PromptInjection.md", []byte("\ufeff## LLM01: Prompt Injection\n### Description\n"), "b3:y")
	if err != nil || b.ID != "LLM01:2023" || b.SameAs["owasp_llm_2023"] != "LLM01:2023" || b.Name != "Prompt Injection" || b.Facts["release"].V != "1.1" {
		t.Errorf("the 1.1 entry is %+v, %v", b, err)
	}
	c, err := OWASPRow(r23, "Archive/1_1_vulns/LLM10_ModelTheft.md", []byte("## LLM10: Model Theft\n"), "")
	if err != nil || c.ID != "LLM10:2023" || c.Facts["rank"].V != 10 {
		t.Errorf("the tenth entry is %+v, %v", c, err)
	}
	var e *ErrSchema
	for _, bad := range []struct{ p, body string }{
		{"2_0_vulns/LLM01_PromptInjection.md", "## Prompt Injection\n"},
		{"2_0_vulns/LLM01_PromptInjection.md", "## LLM02:2025 Sensitive Information Disclosure\n"},
		{"2_0_vulns/LLM01_PromptInjection.md", "## LLM01:2023 Prompt Injection\n"},
		{"2_0_vulns/README.md", "## LLM01:2025 Prompt Injection\n"},
	} {
		if _, err := OWASPRow(r25, bad.p, []byte(bad.body), ""); !errors.As(err, &e) {
			t.Errorf("%s %q: %v", bad.p, bad.body, err)
		}
	}
}
