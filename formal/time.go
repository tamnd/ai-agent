// SPDX-License-Identifier: Apache-2.0

package formal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The limits of rule F10 (spec 11.13). A module that takes more than SlowModule to build gets a warning, and a module that takes more than TooSlowModule blocks the merge.
const (
	SlowModule    = 60 * time.Second
	TooSlowModule = 300 * time.Second
)

// ModuleTime is the time that one module of the package takes to build.
type ModuleTime struct {
	// Module is the module name, such as AI.Search.BinarySearch, and File its path in the package.
	Module string
	File   string
	// Took is the time of the module alone, without the load of its imports. Wall is the time of the whole lean process.
	Took time.Duration
	Wall time.Duration
}

// Time elaborates each module of the namespace AI again with the Lean profiler, and measures the time. Run it after the build, so that the modules that a module imports are built. The time of a module is the sum of the profiler times without import and initialization, because the load of Mathlib takes the same time for each module and is not the cost of its proofs. When the profiler gives no times, the time is the wall time. The result is sorted by module name.
func Time(ctx context.Context, o Options) ([]ModuleTime, error) {
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return nil, err
	}
	var files []string
	err = filepath.WalkDir(filepath.Join(dir, "AI"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".lean") {
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	out := make([]ModuleTime, 0, len(files))
	for _, f := range files {
		var buf bytes.Buffer
		q := o
		q.Log = &buf
		if o.Log != nil {
			q.Log = io.MultiWriter(o.Log, &buf)
		}
		start := time.Now()
		if err := q.run(ctx, dir, nil, "lake", "env", "lean", "-Dprofiler=true", f); err != nil {
			return nil, fmt.Errorf("lean %s: %w", f, err)
		}
		t := ModuleTime{Module: strings.ReplaceAll(strings.TrimSuffix(f, ".lean"), "/", "."), File: f, Wall: time.Since(start)}
		var ok bool
		if t.Took, ok = ownTime(buf.String()); !ok {
			t.Took = t.Wall
		}
		out = append(out, t)
	}
	return out, nil
}

// profileRE matches one line of the cumulative times of the Lean profiler, such as "\timport 8.46s" or "\ttype checking 1.04ms".
var profileRE = regexp.MustCompile(`^\t([a-zA-Z][a-zA-Z -]*?) ([0-9.]+)(s|ms|us|μs|ns)$`)

// ownTime adds the cumulative times of the profiler output, without import and initialization. It reports false when the output has no cumulative times.
func ownTime(log string) (time.Duration, bool) {
	_, after, found := strings.Cut(log, "cumulative profiling times:")
	if !found {
		return 0, false
	}
	var sum time.Duration
	for _, line := range strings.Split(after, "\n") {
		m := profileRE.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil || m[1] == "import" || m[1] == "initialization" {
			continue
		}
		unit := m[3]
		if unit == "μs" {
			unit = "us"
		}
		d, err := time.ParseDuration(m[2] + unit)
		if err == nil {
			sum += d
		}
	}
	return sum, true
}

// CheckTimes reports each module that is slower than SlowModule (rule F10). The finding is a warning up to TooSlowModule, and blocks above it.
func CheckTimes(ts []ModuleTime) []Finding {
	var out []Finding
	for _, t := range ts {
		if t.Took <= SlowModule {
			continue
		}
		f := Finding{Rule: "F10", Where: t.File, Severity: "warn", Msg: fmt.Sprintf("the module %s takes %s to build, more than %s; make the proofs faster", t.Module, t.Took.Round(time.Second), SlowModule)}
		if t.Took > TooSlowModule {
			f.Severity = "block"
			f.Msg = fmt.Sprintf("the module %s takes %s to build, more than %s", t.Module, t.Took.Round(time.Second), TooSlowModule)
		}
		out = append(out, f)
	}
	return out
}
