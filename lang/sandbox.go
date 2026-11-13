// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

// The paths of the tools in the runner images. The Dockerfiles in images/ put them there.
const (
	imageJUnit        = "/opt/algo/junit.jar"
	imageKotlinStdlib = "/opt/kotlinc/lib/kotlin-stdlib.jar"
)

// Sandbox runs the builds, the tests and the adapters in containers (spec 08.7). Each command gets a new container with no network, a read-only root, a memory limit, a CPU limit and a process limit. The build tree is the only writable mount.
type Sandbox struct {
	// Engine is the container command, such as docker or podman.
	Engine string
	// Images maps each language to its image, pinned by digest.
	Images map[string]string
	// Memory is the memory limit of one container, such as 2g.
	Memory string
	// CPUs is the CPU limit of one container, such as 2.
	CPUs string
	// Pids is the process limit of one container.
	Pids int

	mu      sync.Mutex
	names   []string
	pulled  map[string]error
	version map[string]string
}

// digestRef matches an image reference that is pinned by digest, or a local image ID. An image ID is a digest too, and the image workflow tests an image by its ID before it pushes it.
var digestRef = regexp.MustCompile(`^([a-z0-9./_-]+(:[A-Za-z0-9._-]+)?@)?sha256:[0-9a-f]{64}$`)

// LoadImages reads the image manifest: a map from a language to an image reference. Each reference must be pinned by digest, so that a tag that moves cannot change a result.
func LoadImages(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for l, ref := range m {
		if !digestRef.MatchString(ref) {
			return nil, fmt.Errorf("%s: the image of %s is not pinned by digest: %s", path, l, ref)
		}
	}
	return m, nil
}

// Image gives the image of the language or the framework. A framework with no image of its own uses the image of its language.
func (s *Sandbox) Image(lang string) (string, bool) {
	if ref, ok := s.Images[lang]; ok {
		return ref, true
	}
	ref, ok := s.Images[Frameworks[lang]]
	return ref, ok
}

// NewSandbox returns a sandbox with the default limits.
func NewSandbox(engine string, images map[string]string) *Sandbox {
	return &Sandbox{Engine: engine, Images: images, Memory: "2g", CPUs: "2", Pids: 512}
}

// Ready pulls the image of the language once. It returns a Skip when the manifest has no image for the language.
func (s *Sandbox) Ready(lang string) error {
	ref, ok := s.Image(lang)
	if !ok {
		return Skip{lang, "image"}
	}
	if _, err := exec.LookPath(s.Engine); err != nil {
		return fmt.Errorf("the sandbox needs %s: %w", s.Engine, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pulled == nil {
		s.pulled = map[string]error{}
	}
	if err, ok := s.pulled[ref]; ok {
		return err
	}
	var err error
	if exec.Command(s.Engine, "image", "inspect", ref).Run() != nil {
		if out, perr := exec.Command(s.Engine, "pull", "-q", ref).CombinedOutput(); perr != nil {
			err = fmt.Errorf("pull %s: %w: %s", ref, perr, strings.TrimSpace(string(out)))
		}
	}
	s.pulled[ref] = err
	return err
}

// Mount is a host directory that the container sees at the same path.
type Mount struct {
	Dir      string
	ReadOnly bool
}

// Argv returns the command that runs cmd in a new container for the language. The first mount is the work directory. A mount keeps its host path in the container, so that the paths in an error message are the same as in host mode and Tree.Rewrite works on them. HOME points into the work directory, so that the caches of one implementation stay between its commands and go away with the tree.
func (s *Sandbox) Argv(lang string, interactive bool, mounts []Mount, cmd []string) []string {
	name := "aik-" + randomHex(6)
	s.mu.Lock()
	s.names = append(s.names, name)
	s.mu.Unlock()
	work := mounts[0].Dir
	home := path.Join(filepath.ToSlash(work), ".algo-home") // a path in the Linux container, so always with /
	a := []string{s.Engine, "run", "--rm", "--name", name, "--label", "aik.sandbox=1"}
	if interactive {
		a = append(a, "-i")
	}
	a = append(a,
		"--network", "none",
		"--read-only",
		"--tmpfs", "/tmp:rw,exec,size=1g",
		"--memory", s.Memory, "--memory-swap", s.Memory,
		"--cpus", s.CPUs,
		"--pids-limit", fmt.Sprint(s.Pids),
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-w", work,
	)
	for _, m := range mounts {
		v := m.Dir + ":" + m.Dir
		if m.ReadOnly {
			v += ":ro"
		}
		a = append(a, "-v", v)
	}
	env := append([]string{"HOME=" + home, "CARGO_HOME=" + path.Join(home, "cargo"), "DOTNET_CLI_HOME=" + home, "TMPDIR=/tmp"}, testEnv...)
	for _, e := range env {
		a = append(a, "-e", e)
	}
	ref, _ := s.Image(lang)
	return append(append(a, ref), cmd...)
}

// Version returns the first line of the version output of the toolchain in the image.
func (s *Sandbox) Version(lang string, cmd []string) string {
	s.mu.Lock()
	if v, ok := s.version[lang]; ok {
		s.mu.Unlock()
		return v
	}
	s.mu.Unlock()
	v := ""
	if s.Ready(lang) == nil {
		args := []string{s.Engine, "run", "--rm", "--network", "none", "--read-only", "--tmpfs", "/tmp", "-e", "HOME=/tmp", "-e", "DOTNET_CLI_HOME=/tmp"}
		ref, _ := s.Image(lang)
		args = append(args, ref)
		if out, err := exec.Command(args[0], append(args[1:], cmd...)...).CombinedOutput(); err == nil {
			v = strings.TrimSpace(strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0])
		}
	}
	s.mu.Lock()
	if s.version == nil {
		s.version = map[string]string{}
	}
	s.version[lang] = v
	s.mu.Unlock()
	return v
}

// Reap removes the containers that are still there, such as the container of an adapter that was killed after a timeout. Killing the engine client does not stop its container.
func (s *Sandbox) Reap() {
	s.mu.Lock()
	names := s.names
	s.names = nil
	s.mu.Unlock()
	if len(names) == 0 {
		return
	}
	// A container that ended is gone already, because of --rm. The engine reports it as missing, and that is not an error here.
	_ = exec.Command(s.Engine, append([]string{"rm", "-f"}, names...)...).Run()
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
