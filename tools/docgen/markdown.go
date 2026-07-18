package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// render converts the strict Markdown subset to HTML plus a plain-text corpus
// for client-side search. Every text node is HTML-escaped and raw HTML input
// is rejected, so the browser can inject the result via innerHTML safely.
//
// Supported: ## / ### headings, paragraphs, - and 1. lists, > blockquotes,
// ``` code fences, | tables, **bold**, `code`, [text](href) links where href
// is #… (hash router) or http(s)://. Anything else is an error — malformed
// content must fail at generation time, not render wrong in the browser.
func render(src string) (htmlOut, textOut string, err error) {
	var hb, tb strings.Builder
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")

	flushPara := func(para []string) error {
		if len(para) == 0 {
			return nil
		}
		joined := strings.Join(para, " ")
		h, err := inline(joined)
		if err != nil {
			return err
		}
		hb.WriteString("<p>" + h + "</p>\n")
		tb.WriteString(plain(joined) + "\n")
		return nil
	}

	var para []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			if err := flushPara(para); err != nil {
				return "", "", fmt.Errorf("dòng %d: %w", i+1, err)
			}
			para = nil
			i++

		case strings.HasPrefix(trimmed, "### "), strings.HasPrefix(trimmed, "## "):
			if err := flushPara(para); err != nil {
				return "", "", fmt.Errorf("dòng %d: %w", i+1, err)
			}
			para = nil
			tag, text := "h2", strings.TrimPrefix(trimmed, "## ")
			if strings.HasPrefix(trimmed, "### ") {
				tag, text = "h3", strings.TrimPrefix(trimmed, "### ")
			}
			h, err := inline(text)
			if err != nil {
				return "", "", fmt.Errorf("dòng %d: %w", i+1, err)
			}
			hb.WriteString("<" + tag + ">" + h + "</" + tag + ">\n")
			tb.WriteString(plain(text) + "\n")
			i++

		case strings.HasPrefix(trimmed, "# "):
			return "", "", fmt.Errorf("dòng %d: chỉ dùng ## hoặc ### (tiêu đề bài nằm ở frontmatter)", i+1)

		case strings.HasPrefix(trimmed, "```"):
			if err := flushPara(para); err != nil {
				return "", "", fmt.Errorf("dòng %d: %w", i+1, err)
			}
			para = nil
			var code []string
			j := i + 1
			for ; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "```" {
					break
				}
				code = append(code, lines[j])
			}
			if j == len(lines) {
				return "", "", fmt.Errorf("dòng %d: code fence không đóng", i+1)
			}
			body := strings.Join(code, "\n")
			hb.WriteString("<pre><code>" + html.EscapeString(body) + "</code></pre>\n")
			tb.WriteString(body + "\n")
			i = j + 1

		case strings.HasPrefix(trimmed, "> "):
			if err := flushPara(para); err != nil {
				return "", "", fmt.Errorf("dòng %d: %w", i+1, err)
			}
			para = nil
			var quote []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "> ") {
				quote = append(quote, strings.TrimPrefix(strings.TrimSpace(lines[i]), "> "))
				i++
			}
			joined := strings.Join(quote, " ")
			h, err := inline(joined)
			if err != nil {
				return "", "", fmt.Errorf("dòng %d: %w", i+1, err)
			}
			hb.WriteString("<blockquote>" + h + "</blockquote>\n")
			tb.WriteString(plain(joined) + "\n")

		case strings.HasPrefix(trimmed, "- "):
			if err := flushPara(para); err != nil {
				return "", "", fmt.Errorf("dòng %d: %w", i+1, err)
			}
			para = nil
			hb.WriteString("<ul>\n")
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "- ") {
				item := strings.TrimPrefix(strings.TrimSpace(lines[i]), "- ")
				i = appendContinuations(lines, i+1, &item)
				h, err := inline(item)
				if err != nil {
					return "", "", fmt.Errorf("dòng %d: %w", i, err)
				}
				hb.WriteString("<li>" + h + "</li>\n")
				tb.WriteString(plain(item) + "\n")
			}
			hb.WriteString("</ul>\n")

		case orderedRe.MatchString(trimmed):
			if err := flushPara(para); err != nil {
				return "", "", fmt.Errorf("dòng %d: %w", i+1, err)
			}
			para = nil
			hb.WriteString("<ol>\n")
			for i < len(lines) && orderedRe.MatchString(strings.TrimSpace(lines[i])) {
				item := orderedRe.ReplaceAllString(strings.TrimSpace(lines[i]), "")
				i = appendContinuations(lines, i+1, &item)
				h, err := inline(item)
				if err != nil {
					return "", "", fmt.Errorf("dòng %d: %w", i, err)
				}
				hb.WriteString("<li>" + h + "</li>\n")
				tb.WriteString(plain(item) + "\n")
			}
			hb.WriteString("</ol>\n")

		case strings.HasPrefix(trimmed, "|"):
			if err := flushPara(para); err != nil {
				return "", "", fmt.Errorf("dòng %d: %w", i+1, err)
			}
			para = nil
			var rows []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
				rows = append(rows, strings.TrimSpace(lines[i]))
				i++
			}
			if len(rows) < 2 || !tableSepRe.MatchString(rows[1]) {
				return "", "", fmt.Errorf("bảng cần dòng phân cách |---| ngay sau dòng tiêu đề")
			}
			h, t, err := renderTable(rows)
			if err != nil {
				return "", "", err
			}
			hb.WriteString(h)
			tb.WriteString(t)

		case strings.HasPrefix(trimmed, "<"):
			return "", "", fmt.Errorf("dòng %d: không cho phép HTML thô trong bài trợ giúp", i+1)

		default:
			para = append(para, trimmed)
			i++
		}
	}
	if err := flushPara(para); err != nil {
		return "", "", err
	}
	return hb.String(), strings.TrimSpace(tb.String()), nil
}

var (
	orderedRe  = regexp.MustCompile(`^\d+\.\s+`)
	tableSepRe = regexp.MustCompile(`^\|[\s:|-]+\|?$`)
	linkRe     = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
)

// appendContinuations folds hanging-indent wrap lines into the current list
// item: raw lines that start with whitespace and are not themselves a new
// list item. Returns the index of the next unconsumed line.
func appendContinuations(lines []string, i int, item *string) int {
	for i < len(lines) {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || raw == trimmed ||
			strings.HasPrefix(trimmed, "- ") || orderedRe.MatchString(trimmed) {
			break
		}
		*item += " " + trimmed
		i++
	}
	return i
}

func renderTable(rows []string) (string, string, error) {
	var hb, tb strings.Builder
	hb.WriteString(`<div class="help-table"><table>` + "\n")
	cells := func(row string) []string {
		row = strings.Trim(row, "|")
		parts := strings.Split(row, "|")
		for k := range parts {
			parts[k] = strings.TrimSpace(parts[k])
		}
		return parts
	}
	writeRow := func(cs []string, tag string) error {
		hb.WriteString("<tr>")
		for _, c := range cs {
			h, err := inline(c)
			if err != nil {
				return err
			}
			hb.WriteString("<" + tag + ">" + h + "</" + tag + ">")
			tb.WriteString(plain(c) + " ")
		}
		hb.WriteString("</tr>\n")
		tb.WriteString("\n")
		return nil
	}
	if err := writeRow(cells(rows[0]), "th"); err != nil {
		return "", "", err
	}
	for _, r := range rows[2:] {
		if err := writeRow(cells(r), "td"); err != nil {
			return "", "", err
		}
	}
	hb.WriteString("</table></div>\n")
	return hb.String(), tb.String(), nil
}

// inline renders one text run. Nesting order: [text](href) links first (their
// labels may contain bold/code), then **bold**, then `code` — so **`x`**
// works; a code span containing ** does not (generation error, rewrite it).
func inline(s string) (string, error) {
	var out strings.Builder
	for {
		m := linkRe.FindStringSubmatchIndex(s)
		if m == nil {
			break
		}
		pre, err := boldCode(s[:m[0]])
		if err != nil {
			return "", err
		}
		out.WriteString(pre)
		label := s[m[2]:m[3]]
		href := s[m[4]:m[5]]
		if !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
			return "", fmt.Errorf("link %q phải là #… hoặc http(s)://", href)
		}
		lab, err := boldCode(label)
		if err != nil {
			return "", err
		}
		attrs := ""
		if strings.HasPrefix(href, "http") {
			attrs = ` target="_blank" rel="noopener"`
		}
		out.WriteString(`<a href="` + html.EscapeString(href) + `"` + attrs + `>` + lab + `</a>`)
		s = s[m[1]:]
	}
	tail, err := boldCode(s)
	if err != nil {
		return "", err
	}
	out.WriteString(tail)
	return out.String(), nil
}

func boldCode(s string) (string, error) {
	parts := strings.Split(s, "**")
	if len(parts)%2 == 0 {
		return "", fmt.Errorf("dấu ** không đóng trong %q", s)
	}
	var out strings.Builder
	for i, p := range parts {
		h, err := code(p)
		if err != nil {
			return "", err
		}
		if i%2 == 1 {
			out.WriteString("<b>" + h + "</b>")
		} else {
			out.WriteString(h)
		}
	}
	return out.String(), nil
}

func code(s string) (string, error) {
	parts := strings.Split(s, "`")
	if len(parts)%2 == 0 {
		return "", fmt.Errorf("dấu ` không đóng trong %q", s)
	}
	var out strings.Builder
	for i, p := range parts {
		if i%2 == 1 {
			out.WriteString("<code>" + html.EscapeString(p) + "</code>")
		} else {
			out.WriteString(html.EscapeString(p))
		}
	}
	return out.String(), nil
}

// plain strips inline markup for the search corpus.
func plain(s string) string {
	s = linkRe.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return s
}
