// Package lang runs the native tests of the implementations (spec 09.9).
//
// Each language has a recipe. A recipe copies the files of one implementation to a new temporary directory, writes the build files that the language needs, and runs the test command there. The recipe does not change the content tree.
//
// The tests run code from the content repository. Run them only in a sandbox, such as a CI runner or a container. The CLI asks for -unsafe-local outside CI.
package lang

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// Timeout is the longest time that the tests of one implementation can take.
var Timeout = 3 * time.Minute

// JUnitVersion is the version of the JUnit console launcher for Java and Kotlin.
const JUnitVersion = "1.13.4"

// Recipe runs the tests of one language.
type Recipe struct {
	ID string
	// Tools are the commands that the recipe needs. If one is missing, the language is skipped.
	Tools []string
	// Version prints the version of the toolchain.
	Version []string
	// Run prepares dir, which holds a copy of the files, and returns the commands to run in it.
	Run func(r *Runner, dir string, n *node.Node, im node.Impl) ([][]string, error)
}

// Recipes are the tier 1 languages (spec 08.2).
var Recipes = map[string]Recipe{
	"python":     {ID: "python", Tools: []string{"python3"}, Version: []string{"python3", "--version"}, Run: python},
	"go":         {ID: "go", Tools: []string{"go"}, Version: []string{"go", "version"}, Run: golang},
	"rust":       {ID: "rust", Tools: []string{"cargo"}, Version: []string{"rustc", "--version"}, Run: rust},
	"c":          {ID: "c", Tools: []string{"cc"}, Version: []string{"cc", "--version"}, Run: clang},
	"cpp":        {ID: "cpp", Tools: []string{"c++"}, Version: []string{"c++", "--version"}, Run: cpp},
	"javascript": {ID: "javascript", Tools: []string{"node"}, Version: []string{"node", "--version"}, Run: javascript},
	"typescript": {ID: "typescript", Tools: []string{"node"}, Version: []string{"node", "--version"}, Run: typescript},
	"java":       {ID: "java", Tools: []string{"javac", "java"}, Version: []string{"javac", "-version"}, Run: java},
	"kotlin":     {ID: "kotlin", Tools: []string{"kotlinc", "java"}, Version: []string{"kotlinc", "-version"}, Run: kotlin},
	"csharp":     {ID: "csharp", Tools: []string{"dotnet"}, Version: []string{"dotnet", "--version"}, Run: csharp},
}

// Runner runs tests and keeps the results.
type Runner struct {
	// Cache holds the downloaded jars. The default is the user cache directory.
	Cache string
	// Log gets one line for each implementation.
	Log func(format string, args ...any)
	// Keep keeps the temporary directories, for debugging.
	Keep bool
	// Box runs the commands in containers. A nil Box runs them on this machine.
	Box *Sandbox
	// TempDir holds the build trees. The default is the temporary directory of the system. A sandbox needs a directory that the container engine can mount.
	TempDir string
	// CaseTimeout is the longest time of one case. The default is the constant CaseTimeout. The mutation run uses a shorter time, because a mutant that loops forever is common.
	CaseTimeout time.Duration
	// Generate is set when the runner only writes a build tree (aik gen). The tree runs later in a runner image, so the tools of this machine are not checked.
	Generate bool
}

// Available tells if the language can run with this runner. In a sandbox, the image replaces the tools of this machine.
func (r *Runner) Available(rc Recipe) error {
	if r.Box != nil {
		return r.Box.Ready(rc.ID)
	}
	return rc.Available()
}

// ToolVersion returns the version of the toolchain that runs the tests of the language.
func (r *Runner) ToolVersion(rc Recipe) string {
	if r.Box != nil {
		return r.Box.Version(rc.ID, rc.Version)
	}
	return rc.ToolVersion()
}

// command returns the command c for the language, in dir. In a sandbox, it runs in a new container.
func (r *Runner) command(ctx context.Context, lang, dir string, c []string) *exec.Cmd {
	if r.Box != nil {
		c = r.Box.Argv(lang, false, []Mount{{Dir: dir}}, c)
	}
	cmd := exec.CommandContext(ctx, c[0], c[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), testEnv...)
	return cmd
}

func (r *Runner) reap() {
	if r.Box != nil {
		r.Box.Reap()
	}
}

// Skip is the error of a language whose tools are missing.
type Skip struct{ Lang, Tool string }

func (s Skip) Error() string { return fmt.Sprintf("%s: the tool %s is not installed", s.Lang, s.Tool) }

// Available tells if the tools of a recipe are installed.
func (rc Recipe) Available() error {
	for _, t := range rc.Tools {
		if _, err := exec.LookPath(t); err != nil {
			return Skip{rc.ID, t}
		}
	}
	// A tool can be a stub that only says how to install the real one, as /usr/bin/javac on macOS is.
	if len(rc.Version) > 0 {
		if err := exec.Command(rc.Version[0], rc.Version[1:]...).Run(); err != nil {
			return Skip{rc.ID, rc.Version[0]}
		}
	}
	return nil
}

// ToolVersion returns the first line of the version output of the toolchain.
func (rc Recipe) ToolVersion() string {
	if len(rc.Version) == 0 {
		return ""
	}
	out, err := exec.Command(rc.Version[0], rc.Version[1:]...).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}

// Test runs the native tests of one implementation.
func (r *Runner) Test(n *node.Node, im node.Impl) status.Test {
	t := status.Test{Slug: n.Slug, Lang: im.Lang}
	if _, ok := Recipes[im.Lang]; !ok {
		t.Output = "no recipe for the language " + im.Lang
		return t
	}
	if !im.HasTests() {
		t.Output = "the implementation has no test file"
		return t
	}
	start := time.Now()
	dir, err := os.MkdirTemp(r.TempDir, "aik-test-"+im.Lang+"-")
	if err != nil {
		t.Output = err.Error()
		return t
	}
	if !r.Keep {
		defer os.RemoveAll(dir)
	}
	defer r.reap()
	tree, err := r.Gen(dir, n, im)
	if err != nil {
		t.Output = err.Error()
		t.Ms = time.Since(start).Milliseconds()
		return t
	}
	cmds := tree.Test
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	var out bytes.Buffer
	t.Pass = true
	for _, c := range cmds {
		cmd := r.command(ctx, im.Lang, dir, c)
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Pass = false
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				fmt.Fprintf(&out, "\nthe tests took more than %s\n", Timeout)
			}
			break
		}
	}
	t.Ms = time.Since(start).Milliseconds()
	t.Output = tail(tree.Rewrite(out.String()), 4000)
	return t
}

func tail(s string, k int) string {
	s = strings.TrimSpace(s)
	if len(s) <= k {
		return s
	}
	return "...\n" + s[len(s)-k:]
}

func files(im node.Impl, test bool, ext ...string) []string {
	var out []string
	for _, f := range im.Files {
		if f.Test != test {
			continue
		}
		for _, e := range ext {
			if strings.HasSuffix(f.Name, e) {
				out = append(out, f.Name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func all(im node.Impl, ext ...string) []string {
	return append(files(im, false, ext...), files(im, true, ext...)...)
}

func write(dir, name, content string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

func python(r *Runner, dir string, _ *node.Node, _ node.Impl) ([][]string, error) {
	// The runner image has pytest.
	if r.Box == nil && !r.Generate && exec.Command("python3", "-c", "import pytest").Run() != nil {
		return nil, Skip{"python", "pytest"}
	}
	return [][]string{{"python3", "-m", "pytest", "-q", "-p", "no:cacheprovider", "."}}, nil
}

func golang(_ *Runner, dir string, _ *node.Node, _ node.Impl) ([][]string, error) {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		if err := write(dir, "go.mod", "module example.com/impl\n\ngo 1.24\n"); err != nil {
			return nil, err
		}
	}
	return [][]string{{"go", "vet", "./..."}, {"go", "test", "-count=1", "./..."}}, nil
}

var nonIdent = regexp.MustCompile(`[^a-z0-9_]+`)

func crate(slug string) string {
	return "algo_" + nonIdent.ReplaceAllString(strings.ReplaceAll(slug, "-", "_"), "")
}

func rust(_ *Runner, dir string, n *node.Node, im node.Impl) ([][]string, error) {
	main := files(im, false, ".rs")
	if len(main) == 0 {
		return nil, errors.New("rust: no source file")
	}
	// A test file is an integration test that imports the crate. Cargo finds it by its [[test]] entry, so the file stays where the node has it.
	toml := fmt.Sprintf("[package]\nname = %q\nversion = \"0.0.0\"\nedition = \"2024\"\npublish = false\nautotests = false\n\n[lib]\npath = %q\n", crate(n.Slug), main[0])
	for _, t := range files(im, true, ".rs") {
		toml += fmt.Sprintf("\n[[test]]\nname = %q\npath = %q\n", stem(t), t)
	}
	toml += "\n[lints.rust]\nwarnings = \"deny\"\n"
	if err := write(dir, "Cargo.toml", toml); err != nil {
		return nil, err
	}
	return [][]string{{"cargo", "test", "--offline", "--quiet"}}, nil
}

var cFlags = []string{"-Wall", "-Wextra", "-Wpedantic", "-Werror", "-g", "-fsanitize=address,undefined", "-fno-omit-frame-pointer"}

func clang(_ *Runner, dir string, _ *node.Node, im node.Impl) ([][]string, error) {
	src := all(im, ".c")
	args := append([]string{"cc", "-std=c2x"}, cFlags...)
	args = append(append(args, src...), "-o", "test_main")
	return [][]string{args, {"./test_main"}}, nil
}

func cpp(_ *Runner, dir string, _ *node.Node, im node.Impl) ([][]string, error) {
	src := all(im, ".cpp", ".cc")
	args := append([]string{"c++", "-std=c++23"}, cFlags...)
	args = append(append(args, src...), "-o", "test_main")
	return [][]string{args, {"./test_main"}}, nil
}

func javascript(_ *Runner, dir string, _ *node.Node, im node.Impl) ([][]string, error) {
	if err := write(dir, "package.json", "{\"type\": \"module\", \"private\": true}\n"); err != nil {
		return nil, err
	}
	return [][]string{append([]string{"node", "--test"}, files(im, true, ".js", ".mjs")...)}, nil
}

func typescript(_ *Runner, dir string, _ *node.Node, im node.Impl) ([][]string, error) {
	if err := write(dir, "package.json", "{\"type\": \"module\", \"private\": true}\n"); err != nil {
		return nil, err
	}
	// Node 24 and later run TypeScript by type stripping. The code uses only erasable syntax.
	return [][]string{append([]string{"node", "--test"}, files(im, true, ".ts")...)}, nil
}

// junit returns the path of the JUnit console launcher, and downloads it once.
func (r *Runner) junit() (string, error) {
	if r.Box != nil {
		return imageJUnit, nil
	}
	cache := r.Cache
	if cache == "" {
		c, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		cache = filepath.Join(c, "aik")
	}
	name := "junit-platform-console-standalone-" + JUnitVersion + ".jar"
	path := filepath.Join(cache, name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return "", err
	}
	url := "https://repo1.maven.org/maven2/org/junit/platform/junit-platform-console-standalone/" + JUnitVersion + "/" + name
	resp, err := http.Get(url) //nolint:noctx // the URL is a constant
	if err != nil {
		return "", fmt.Errorf("download JUnit: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download JUnit: %s", resp.Status)
	}
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmp, path)
}

func java(r *Runner, dir string, _ *node.Node, im node.Impl) ([][]string, error) {
	jar, err := r.junit()
	if err != nil {
		return nil, err
	}
	args := append([]string{"javac", "-Xlint:all", "-Werror", "-d", "out", "-cp", jar}, all(im, ".java")...)
	run := []string{"java", "-jar", jar, "execute", "--class-path", "out", "--scan-class-path", "--disable-banner", "--details=summary", "--fail-if-no-tests"}
	return [][]string{args, run}, nil
}

// kotlinStdlib returns the path of kotlin-stdlib.jar of the kotlinc on the PATH, or in the image.
func (r *Runner) kotlinStdlib() (string, error) {
	if r.Box != nil {
		return imageKotlinStdlib, nil
	}
	kc, err := exec.LookPath("kotlinc")
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(kc); err == nil {
		kc = real
	}
	stdlib := filepath.Join(filepath.Dir(filepath.Dir(kc)), "lib", "kotlin-stdlib.jar")
	if _, err := os.Stat(stdlib); err != nil {
		return "", fmt.Errorf("kotlin: no kotlin-stdlib.jar next to kotlinc: %w", err)
	}
	return stdlib, nil
}

func kotlin(r *Runner, dir string, _ *node.Node, im node.Impl) ([][]string, error) {
	jar, err := r.junit()
	if err != nil {
		return nil, err
	}
	stdlib, err := r.kotlinStdlib()
	if err != nil {
		return nil, err
	}
	args := append([]string{"kotlinc", "-Werror", "-cp", jar, "-d", "out"}, all(im, ".kt")...)
	sep := string(os.PathListSeparator)
	if r.Box != nil {
		sep = ":"
	}
	cp := "out" + sep + stdlib
	run := []string{"java", "-jar", jar, "execute", "--class-path", cp, "--scan-class-path", "--disable-banner", "--details=summary", "--fail-if-no-tests"}
	return [][]string{args, run}, nil
}

const csproj = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net%s</TargetFramework>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <IsPackable>false</IsPackable>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.NET.Test.Sdk" Version="17.14.1" />
    <PackageReference Include="xunit" Version="2.9.3" />
    <PackageReference Include="xunit.runner.visualstudio" Version="3.1.4" />
  </ItemGroup>
</Project>
`

// dotnetMajor returns the major version of the .NET SDK, such as 10.
func (r *Runner) dotnetMajor() (string, error) {
	v := ""
	if r.Box != nil {
		if v = r.Box.Version("csharp", []string{"dotnet", "--version"}); v == "" {
			return "", fmt.Errorf("csharp: no dotnet in the image")
		}
	} else {
		out, err := exec.Command("dotnet", "--version").Output()
		if err != nil {
			return "", err
		}
		v = string(out)
	}
	major, _, _ := strings.Cut(strings.TrimSpace(v), ".")
	return major, nil
}

func csharp(r *Runner, dir string, _ *node.Node, _ node.Impl) ([][]string, error) {
	major, err := r.dotnetMajor()
	if err != nil {
		return nil, err
	}
	if err := write(dir, "Impl.Tests.csproj", fmt.Sprintf(csproj, major+".0")); err != nil {
		return nil, err
	}
	return [][]string{{"dotnet", "test", "--nologo", "-v", "q"}}, nil
}
