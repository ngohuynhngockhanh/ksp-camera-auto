package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	serverFile = "internal/server/server.go"
	appJSFile  = "web/static/app.js"
	ignoreFile = "docs/help/coverage-ignore.txt"
)

var (
	routeRe = regexp.MustCompile(`mux\.Handle(?:Func)?\(\s*"(/[^"]*)"`)
	hashRe  = regexp.MustCompile(`hash:\s*'([a-z][a-z0-9-]*)'`)
)

// driftCheck lists user-facing surfaces (HTTP routes registered in server.go
// and nav tabs in app.js) that no help article claims via covers:/ui: and
// that are not listed in coverage-ignore.txt. Read-only.
func driftCheck(arts []*article) ([]string, error) {
	var surfaces []string

	srv, err := os.ReadFile(serverFile)
	if err != nil {
		return nil, err
	}
	for _, m := range routeRe.FindAllStringSubmatch(string(srv), -1) {
		surfaces = append(surfaces, m[1])
	}

	appJS, err := os.ReadFile(appJSFile)
	if err != nil {
		return nil, err
	}
	nav := string(appJS)
	if i := strings.Index(nav, "const NAV_ITEMS"); i >= 0 {
		nav = nav[i:]
		if j := strings.Index(nav, "];"); j >= 0 {
			nav = nav[:j]
		}
	} else {
		return nil, fmt.Errorf("không tìm thấy NAV_ITEMS trong %s", appJSFile)
	}
	for _, m := range hashRe.FindAllStringSubmatch(nav, -1) {
		surfaces = append(surfaces, "#"+m[1])
	}

	covered := map[string]bool{}
	for _, a := range arts {
		for _, c := range a.covers {
			covered[c] = true
		}
		if a.UI != "" {
			covered[a.UI] = true
		}
	}
	if raw, err := os.ReadFile(ignoreFile); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			// Comments use //; a leading # is a nav-tab surface (e.g. #dashboard).
			if line != "" && !strings.HasPrefix(line, "//") {
				covered[line] = true
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	var uncovered []string
	seen := map[string]bool{}
	for _, s := range surfaces {
		if !covered[s] && !seen[s] {
			uncovered = append(uncovered, s)
			seen[s] = true
		}
	}
	return uncovered, nil
}
