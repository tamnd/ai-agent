// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UserAgent is the User-Agent of every request of aik: a client name and the URL of its code, with no address of a
// person in it.
func UserAgent(version string) string {
	return "aik/" + strings.TrimPrefix(version, "v") + " (+https://github.com/tamnd/ai-agent)"
}

// Client fetches URLs politely (spec 13.4): it sends the User-Agent of aik, reads each host's robots.txt before the
// first request to the host and obeys it, waits at least Gap between two requests to one host (or the host's
// Crawl-delay when that is longer), and backs off when the host answers 429 or 503.
type Client struct {
	UA   string
	HTTP *http.Client
	Gap  time.Duration
	// Sleep and Now are time.Sleep and time.Now; tests replace them.
	Sleep func(time.Duration)
	Now   func() time.Time

	mu     sync.Mutex
	last   map[string]time.Time
	robots map[string]*robots
	// Requests counts the requests that the client sent, robots.txt included.
	Requests int
}

// NewClient makes a client with the User-Agent of version and a gap between requests.
func NewClient(version string, gap time.Duration) *Client {
	return &Client{UA: UserAgent(version), HTTP: &http.Client{Timeout: 10 * time.Minute}, Gap: gap, Sleep: time.Sleep, Now: time.Now}
}

// ErrRobots means that the robots.txt of the host does not let aik fetch the URL.
var ErrRobots = errors.New("robots.txt does not allow the URL")

// Get fetches a URL and returns its body, at most limit bytes.
func (c *Client) Get(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	rb, err := c.robotsFor(ctx, u)
	if err != nil {
		return nil, err
	}
	if !rb.allows(u.EscapedPath() + query(u)) {
		return nil, fmt.Errorf("%s: %w", rawURL, ErrRobots)
	}
	for try := 0; ; try++ {
		b, retry, err := c.get(ctx, u, limit, rb.delay)
		if err == nil || retry <= 0 || try >= 3 {
			return b, err
		}
		c.Sleep(retry)
	}
}

// Status sends a GET for a URL, as Get does, and returns the status code of the answer without reading the body: a
// link check (spec 14.9). A 429 or a 503 is tried again, as in Get.
func (c *Client) Status(ctx context.Context, rawURL string) (int, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, err
	}
	rb, err := c.robotsFor(ctx, u)
	if err != nil {
		return 0, err
	}
	if !rb.allows(u.EscapedPath() + query(u)) {
		return 0, fmt.Errorf("%s: %w", rawURL, ErrRobots)
	}
	for try := 0; ; try++ {
		c.wait(u.Host, rb.delay)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("User-Agent", c.UA)
		c.mu.Lock()
		c.Requests++
		c.mu.Unlock()
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return 0, err
		}
		_ = resp.Body.Close() // only the status is read
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable) && try < 3 {
			wait := 30 * time.Second
			if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s >= 0 {
				wait = min(time.Duration(s)*time.Second, 5*time.Minute)
			}
			c.Sleep(max(wait, c.Gap))
			continue
		}
		return resp.StatusCode, nil
	}
}

func query(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}
	return "?" + u.RawQuery
}

// get sends one request. On a 429 or a 503 it returns how long to wait before the next try.
func (c *Client) get(ctx context.Context, u *url.URL, limit int64, delay time.Duration) ([]byte, time.Duration, error) {
	c.wait(u.Host, delay)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.UA)
	c.mu.Lock()
	c.Requests++
	c.mu.Unlock()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		wait := 30 * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s >= 0 {
			wait = min(time.Duration(s)*time.Second, 5*time.Minute)
		}
		return nil, max(wait, c.Gap), fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, 0, err
	}
	if int64(len(b)) > limit {
		return nil, 0, fmt.Errorf("GET %s: the body is larger than %d MB", u, limit>>20)
	}
	return b, 0, nil
}

// wait sleeps until the gap since the last request to the host has passed.
func (c *Client) wait(host string, delay time.Duration) {
	gap := max(c.Gap, delay)
	c.mu.Lock()
	if c.last == nil {
		c.last = map[string]time.Time{}
	}
	now := c.Now()
	next := c.last[host].Add(gap)
	if next.Before(now) {
		next = now
	}
	c.last[host] = next
	c.mu.Unlock()
	if d := next.Sub(now); d > 0 {
		c.Sleep(d)
	}
}

func (c *Client) robotsFor(ctx context.Context, u *url.URL) (*robots, error) {
	key := u.Scheme + "://" + u.Host
	c.mu.Lock()
	rb := c.robots[key]
	c.mu.Unlock()
	if rb != nil {
		return rb, nil
	}
	rb, err := c.fetchRobots(ctx, key)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.robots == nil {
		c.robots = map[string]*robots{}
	}
	c.robots[key] = rb
	c.mu.Unlock()
	return rb, nil
}

// fetchRobots reads robots.txt as RFC 9309 says: a 4xx status means no rules, and a 5xx status or a failed request
// means that nothing may be fetched.
func (c *Client) fetchRobots(ctx context.Context, base string) (*robots, error) {
	u, _ := url.Parse(base + "/robots.txt")
	c.wait(u.Host, 0)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UA)
	c.mu.Lock()
	c.Requests++
	c.mu.Unlock()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("%s: %s, so the host may not be fetched", u, resp.Status)
	case resp.StatusCode >= 400:
		return &robots{}, nil
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%s: %s", u, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 500<<10))
	if err != nil {
		return nil, err
	}
	return parseRobots(b, "aik"), nil
}

// robots holds the rules of one robots.txt group.
type robots struct {
	rules []rule
	delay time.Duration
}

type rule struct {
	allow bool
	path  string
}

// parseRobots reads the group for the agent, or else the group for *. Lines of a group follow its user-agent lines.
func parseRobots(b []byte, agent string) *robots {
	type group struct {
		agents []string
		r      robots
	}
	var groups []*group
	var cur *group
	inAgents := false
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		switch k {
		case "user-agent":
			if !inAgents {
				cur = &group{}
				groups = append(groups, cur)
			}
			cur.agents = append(cur.agents, strings.ToLower(v))
			inAgents = true
			continue
		case "allow", "disallow":
			if cur != nil && v != "" {
				cur.r.rules = append(cur.r.rules, rule{allow: k == "allow", path: v})
			}
		case "crawl-delay":
			if f, err := strconv.ParseFloat(v, 64); err == nil && cur != nil {
				cur.r.delay = time.Duration(f * float64(time.Second))
			}
		}
		inAgents = false
	}
	var star *robots
	out := &robots{}
	found := false
	for _, g := range groups {
		for _, a := range g.agents {
			switch {
			case a == agent:
				out.rules, out.delay = append(out.rules, g.r.rules...), max(out.delay, g.r.delay)
				found = true
			case a == "*" && star == nil:
				r := g.r
				star = &r
			}
		}
	}
	if !found && star != nil {
		return star
	}
	return out
}

// allows tells if the path may be fetched: the longest matching rule wins, and an allow rule wins a tie.
func (r *robots) allows(path string) bool {
	best, allow := -1, true
	for _, ru := range r.rules {
		if n := matchLen(ru.path, path); n > best || n == best && ru.allow {
			best, allow = n, ru.allow
		}
	}
	return allow
}

// matchLen gives the length of the pattern when it matches the start of the path, and -1 when it does not. A
// pattern may hold * for any characters and end with $ for the end of the path.
func matchLen(pat, path string) int {
	anchored := strings.HasSuffix(pat, "$")
	p := strings.TrimSuffix(pat, "$")
	parts := strings.Split(p, "*")
	if !strings.HasPrefix(path, parts[0]) {
		return -1
	}
	at := len(parts[0])
	for _, part := range parts[1:] {
		i := strings.Index(path[at:], part)
		if i < 0 {
			return -1
		}
		at += i + len(part)
	}
	if anchored && at != len(path) && (len(parts) == 1 || !strings.HasSuffix(path, parts[len(parts)-1])) {
		return -1
	}
	return len(pat)
}
