// Command docgen builds the embedded help bundle for the web UI.
//
// It reads Vietnamese help articles from docs/help/*.md (YAML frontmatter +
// a strict Markdown subset), renders them to HTML at generation time (the
// browser never parses Markdown), and writes a single deterministic JSON
// bundle to web/static/help/help-index.json, which go:embed ships inside the
// binary.
//
// Run from the repo root:
//
//	go run ./tools/docgen          # regenerate the bundle (drift = warning)
//	go run ./tools/docgen -check   # no output written; exit 1 on any
//	                               # validation error or uncovered route/tab
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	helpDir = "docs/help"
	outFile = "web/static/help/help-index.json"
)

// sectionDefs fixes the section set and its display order.
var sectionDefs = []struct{ ID, Title string }{
	{"start", "Bắt đầu"},
	{"scan", "Quét mạng"},
	{"cameras", "Kho camera"},
	{"bulk", "Chỉnh hàng loạt"},
	{"dahua", "Riêng Dahua/KBVision"},
	{"import", "Nhập dữ liệu"},
	{"admin", "Cấu hình & vận hành"},
}

type frontmatter struct {
	ID       string   `yaml:"id"`
	Title    string   `yaml:"title"`
	Section  string   `yaml:"section"`
	Order    int      `yaml:"order"`
	Keywords []string `yaml:"keywords"`
	UI       string   `yaml:"ui"`
	Covers   []string `yaml:"covers"`
	Related  []string `yaml:"related"`
}

type article struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Section  string   `json:"section"`
	Order    int      `json:"order"`
	Keywords []string `json:"keywords"`
	UI       string   `json:"ui"`
	Related  []string `json:"related"`
	Snippet  string   `json:"snippet"`
	Text     string   `json:"text"`
	HTML     string   `json:"html"`

	covers []string
}

func main() {
	check := flag.Bool("check", false, "validate + drift-check only; write nothing, exit 1 on problems")
	flag.Parse()

	arts, errs := loadArticles(helpDir)
	errs = append(errs, validate(arts)...)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "docgen:", e)
		}
		os.Exit(1)
	}

	uncovered, err := driftCheck(arts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docgen: drift check:", err)
		os.Exit(1)
	}

	if *check {
		if len(uncovered) > 0 {
			fmt.Fprintln(os.Stderr, "docgen: các tính năng sau chưa có bài trợ giúp (thêm vào `covers:`/`ui:` của một bài, hoặc vào docs/help/coverage-ignore.txt):")
			for _, u := range uncovered {
				fmt.Fprintln(os.Stderr, "  -", u)
			}
			os.Exit(1)
		}
		fmt.Printf("docgen: OK — %d bài, mọi route/tab đều có bài trợ giúp\n", len(arts))
		return
	}

	for _, u := range uncovered {
		fmt.Fprintln(os.Stderr, "docgen: cảnh báo: chưa có bài trợ giúp cho", u)
	}
	if err := writeBundle(arts); err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
	fmt.Printf("docgen: đã ghi %s (%d bài)\n", outFile, len(arts))
}

func loadArticles(dir string) ([]*article, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{err}
	}
	var arts []*article
	var errs []error
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") ||
			strings.HasPrefix(name, "_") || name == "STYLE.md" {
			continue
		}
		a, err := parseArticle(filepath.Join(dir, name))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if a.ID != strings.TrimSuffix(name, ".md") {
			errs = append(errs, fmt.Errorf("%s: id %q khác tên file", name, a.ID))
		}
		arts = append(arts, a)
	}
	return arts, errs
}

func parseArticle(path string) (*article, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	body, fm, err := splitFrontmatter(string(raw))
	if err != nil {
		return nil, err
	}
	var m frontmatter
	if err := yaml.Unmarshal([]byte(fm), &m); err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	html, text, err := render(body)
	if err != nil {
		return nil, err
	}
	snippet := text
	if i := strings.IndexByte(snippet, '\n'); i > 0 {
		snippet = snippet[:i]
	}
	if r := []rune(snippet); len(r) > 160 {
		snippet = string(r[:160]) + "…"
	}
	return &article{
		ID: m.ID, Title: m.Title, Section: m.Section, Order: m.Order,
		Keywords: m.Keywords, UI: m.UI, Related: m.Related,
		Snippet: snippet, Text: text, HTML: html,
		covers: m.Covers,
	}, nil
}

func splitFrontmatter(src string) (body, fm string, err error) {
	src = strings.TrimPrefix(src, "\uFEFF")
	if !strings.HasPrefix(src, "---\n") {
		return "", "", fmt.Errorf("thiếu frontmatter (file phải bắt đầu bằng ---)")
	}
	rest := src[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", "", fmt.Errorf("frontmatter không đóng (thiếu --- thứ hai)")
	}
	return rest[end+5:], rest[:end], nil
}

func validate(arts []*article) []error {
	var errs []error
	valid := map[string]bool{}
	for _, s := range sectionDefs {
		valid[s.ID] = true
	}
	ids := map[string]bool{}
	for _, a := range arts {
		if a.ID == "" {
			errs = append(errs, fmt.Errorf("bài thiếu id"))
			continue
		}
		if ids[a.ID] {
			errs = append(errs, fmt.Errorf("%s: id trùng", a.ID))
		}
		ids[a.ID] = true
		if a.Title == "" {
			errs = append(errs, fmt.Errorf("%s: thiếu title", a.ID))
		}
		if !valid[a.Section] {
			errs = append(errs, fmt.Errorf("%s: section %q không hợp lệ", a.ID, a.Section))
		}
		if len(a.Keywords) == 0 {
			errs = append(errs, fmt.Errorf("%s: keywords rỗng", a.ID))
		}
		if a.UI != "" && !strings.HasPrefix(a.UI, "#") {
			errs = append(errs, fmt.Errorf("%s: ui %q phải bắt đầu bằng #", a.ID, a.UI))
		}
	}
	for _, a := range arts {
		for _, r := range a.Related {
			if !ids[r] {
				errs = append(errs, fmt.Errorf("%s: related %q không tồn tại", a.ID, r))
			}
		}
		// Cross-links #help/<id> inside the rendered HTML must resolve.
		for _, target := range helpLinks(a.HTML) {
			if !ids[target] {
				errs = append(errs, fmt.Errorf("%s: link #help/%s gãy", a.ID, target))
			}
		}
	}
	return errs
}

// helpLinks extracts <id> from every href="#help/<id>" in rendered HTML.
func helpLinks(html string) []string {
	var out []string
	const marker = `href="#help/`
	for h := html; ; {
		i := strings.Index(h, marker)
		if i < 0 {
			break
		}
		h = h[i+len(marker):]
		if j := strings.IndexByte(h, '"'); j > 0 {
			out = append(out, h[:j])
		}
	}
	return out
}

func writeBundle(arts []*article) error {
	secIdx := map[string]int{}
	for i, s := range sectionDefs {
		secIdx[s.ID] = i
	}
	sort.Slice(arts, func(i, j int) bool {
		a, b := arts[i], arts[j]
		if secIdx[a.Section] != secIdx[b.Section] {
			return secIdx[a.Section] < secIdx[b.Section]
		}
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.ID < b.ID
	})

	type section struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	out := struct {
		Sections []section  `json:"sections"`
		Articles []*article `json:"articles"`
	}{Articles: arts}
	for _, s := range sectionDefs {
		out.Sections = append(out.Sections, section{s.ID, s.Title})
	}

	buf, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outFile, buf, 0o644)
}
