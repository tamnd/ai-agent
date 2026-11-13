// SPDX-License-Identifier: Apache-2.0

// Package jsonl runs a child process that reads one JSON value per line on standard input and writes one per line on standard output. The adapters (spec 09.2) and the custom checkers (spec 09.3) use it.
package jsonl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ErrTimeout is the error of a reply that does not come in time. The process is killed.
var ErrTimeout = errors.New("no reply in time")

// Proc is a running child process.
type Proc struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	lines  chan []byte
	done   chan struct{}
	mu     sync.Mutex
	stderr bytes.Buffer
}

// Start starts argv in dir.
func Start(dir string, argv []string, env []string) (*Proc, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	p := &Proc{cmd: cmd, lines: make(chan []byte, 1), done: make(chan struct{})}
	cmd.Stderr = &lockedWriter{p: p}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p.in = in
	go func() {
		defer close(p.done)
		r := bufio.NewReaderSize(out, 1<<16)
		for {
			line, err := r.ReadBytes('\n')
			if len(bytes.TrimSpace(line)) > 0 {
				p.lines <- line
			}
			if err != nil {
				return
			}
		}
	}()
	return p, nil
}

type lockedWriter struct{ p *Proc }

func (w *lockedWriter) Write(b []byte) (int, error) {
	w.p.mu.Lock()
	defer w.p.mu.Unlock()
	if w.p.stderr.Len() < 1<<20 {
		w.p.stderr.Write(b)
	}
	return len(b), nil
}

// Stderr returns the last part of the standard error of the process.
func (p *Proc) Stderr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := strings.TrimSpace(p.stderr.String())
	if len(s) > 2000 {
		s = "…" + s[len(s)-2000:]
	}
	return s
}

// Call sends v as one line and waits for one line of reply.
func (p *Proc) Call(v any, timeout time.Duration) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if _, err := p.in.Write(append(b, '\n')); err != nil {
		return nil, p.died()
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case line := <-p.lines:
		return line, nil
	case <-p.done:
		select {
		case line := <-p.lines:
			return line, nil
		default:
		}
		return nil, p.died()
	case <-t.C:
		p.Kill()
		return nil, ErrTimeout
	}
}

func (p *Proc) died() error {
	err := p.cmd.Wait()
	msg := "the process stopped"
	if err != nil {
		msg = "the process stopped: " + err.Error()
	}
	if s := p.Stderr(); s != "" {
		msg += "\n" + s
	}
	return errors.New(msg)
}

// Kill stops the process.
func (p *Proc) Kill() {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	_ = p.cmd.Wait()
}

// Close sends bye, closes standard input and waits a short time for the process to stop.
func (p *Proc) Close() {
	b, _ := json.Marshal(map[string]string{"op": "bye"})
	_, _ = p.in.Write(append(b, '\n'))
	_ = p.in.Close()
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
	}
	p.Kill()
}

// Errorf is a helper for a reply that is not what the protocol says.
func Errorf(line []byte, format string, args ...any) error {
	return fmt.Errorf("%s: %s", fmt.Sprintf(format, args...), strings.TrimSpace(string(line)))
}
