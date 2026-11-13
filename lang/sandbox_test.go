// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

const pinned = "ghcr.io/tamnd/algo-runner-python@sha256:930e1abba7c4e8f96ed93e8012c35d174d7732752731bbd5e8e757d7ec6f3966"

func TestLoadImages(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	if err := os.WriteFile(good, []byte("python: "+pinned+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadImages(good)
	if err != nil || m["python"] != pinned {
		t.Fatalf("LoadImages = %v, %v", m, err)
	}
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("python: ghcr.io/tamnd/algo-runner-python:latest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadImages(bad); err == nil || !strings.Contains(err.Error(), "not pinned by digest") {
		t.Fatalf("a tag must not pass: %v", err)
	}
}

func TestSandboxArgv(t *testing.T) {
	s := NewSandbox("docker", map[string]string{"python": pinned})
	a := s.Argv("python", true, []Mount{{Dir: "/w/tree"}, {Dir: "/w/problem", ReadOnly: true}}, []string{"python3", "-m", "pytest"})
	line := strings.Join(a, " ")
	for _, want := range []string{"--network none", "--read-only", "--memory 2g", "--pids-limit 512", "--cap-drop ALL", "no-new-privileges", "-v /w/tree:/w/tree", "-v /w/problem:/w/problem:ro", "-w /w/tree", "HOME=/w/tree/.algo-home", " -i "} {
		if !strings.Contains(line, want) {
			t.Errorf("the command has no %q: %s", want, line)
		}
	}
	if i := slices.Index(a, pinned); i < 0 || !slices.Equal(a[i+1:], []string{"python3", "-m", "pytest"}) {
		t.Errorf("the image and the command are not at the end: %s", line)
	}
	if len(s.names) != 1 {
		t.Errorf("the sandbox keeps %d names, want 1", len(s.names))
	}
}

// TestSandboxNoNetwork runs a test that opens a socket in the Python image. The test must fail, because the container has no network. Set AIK_TEST_PYTHON_IMAGE to the image to run it.
func TestSandboxNoNetwork(t *testing.T) {
	ref := os.Getenv("AIK_TEST_PYTHON_IMAGE")
	if ref == "" {
		t.Skip("AIK_TEST_PYTHON_IMAGE is not set")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("no docker")
	}
	r := &Runner{Box: NewSandbox("docker", map[string]string{"python": ref}), TempDir: t.TempDir()}
	if err := r.Available(Recipes["python"]); err != nil {
		t.Fatal(err)
	}
	n := &node.Node{Slug: "net"}
	good := node.Impl{Lang: "python", Files: []node.File{
		{Name: "net.py", Content: "def one():\n    return 1\n"},
		{Name: "test_net.py", Test: true, Content: "from net import one\n\n\ndef test_one():\n    assert one() == 1\n"},
	}}
	if got := r.Test(n, good); !got.Pass {
		t.Fatalf("a test with no socket fails in the sandbox:\n%s", got.Output)
	}
	sock := node.Impl{Lang: "python", Files: []node.File{
		{Name: "net.py", Content: "import socket\n\n\ndef fetch():\n    with socket.create_connection((\"1.1.1.1\", 53), timeout=5):\n        return True\n"},
		{Name: "test_net.py", Test: true, Content: "from net import fetch\n\n\ndef test_fetch():\n    assert fetch()\n"},
	}}
	got := r.Test(n, sock)
	if got.Pass {
		t.Fatalf("a test that opens a socket passes in the sandbox:\n%s", got.Output)
	}
	if !strings.Contains(got.Output, "OSError") && !strings.Contains(got.Output, "unreachable") {
		t.Errorf("the test fails for another reason:\n%s", got.Output)
	}
}

func TestFrameworkImage(t *testing.T) {
	s := NewSandbox("docker", map[string]string{"python": "py@sha256:1", "jax": "jax@sha256:2"})
	for fw, want := range map[string]string{"numpy": "py@sha256:1", "pytorch": "py@sha256:1", "jax": "jax@sha256:2", "python": "py@sha256:1"} {
		if got, ok := s.Image(fw); !ok || got != want {
			t.Errorf("Image(%q) = %q, %v; want %q", fw, got, ok, want)
		}
	}
	if _, ok := s.Image("julia"); ok {
		t.Error("julia has an image")
	}
	if rc, ok := Recipes["pytorch"]; !ok || rc.ID != "pytorch" || rc.Run == nil {
		t.Errorf("the recipe of pytorch is %+v", rc)
	}
	if _, ok := adapters["numpy"]; !ok {
		t.Error("numpy has no adapter")
	}
}
