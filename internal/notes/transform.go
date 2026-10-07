// Package notes turns Drive's markdown export of a "Notes by Gemini" doc into
// the summary and full markdown files, and names them.
package notes

import (
	"regexp"
	"strings"
	"time"
)

// Result is the rendered output for one doc.
type Result struct {
	Summary string
	Full    string
	Warning string // non-empty when the doc format was not recognized
}

var (
	headingRe   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	anchorRe    = regexp.MustCompile(`\s*\{#[^}]*\}\s*$`)
	imageRe     = regexp.MustCompile(`!\[[^\]]*\](?:\[[^\]]*\]|\([^)]*\))`)
	imageDefRe  = regexp.MustCompile(`^\[[^\]]+\]:\s*<data:`)
	linkRe      = regexp.MustCompile(`\[((?:\\.|[^\]\\])*)\]\([^)]*\)`)
	inviteRe    = regexp.MustCompile(`(~~)?\[((?:\\.|[^\]\\])*)\]\([^)]*\)(~~)?`)
	escapeRe    = regexp.MustCompile(`\\([-\[\].()#!+~>])`)
	dateLineRe  = regexp.MustCompile(`^[A-Z][a-z]{2,8} \d{1,2}, \d{4}$`)
	boilerplate = []string{
		"rate the new Quick notes tab",
		"You should review Gemini",
		"these specific notes",
		"This editable transcript was computer generated",
	}
)

// Transform converts one exported doc. fallbackTitle and fallbackDate are used
// when the doc lacks a title heading or date line.
func Transform(doc, fallbackTitle string, fallbackDate time.Time) Result {
	tabs := splitTabs(strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n"))
	fallbackDateLine := fallbackDate.Format("Jan 2, 2006")

	full, ok := tabs["Full notes"]
	if !ok {
		var all []string
		for _, l := range strings.Split(doc, "\n") {
			if l, keep := cleanLine(l); keep {
				all = append(all, l)
			}
		}
		return Result{
			Summary: render("## "+fallbackTitle, "", fallbackDateLine),
			Full:    render(all...),
			Warning: "no Full notes tab found; wrote cleaned doc as-is",
		}
	}

	var date, title, invited string
	var body []string
	inBody := false
	for _, l := range full {
		switch {
		case inBody:
			if strings.HasPrefix(l, "## ") {
				l = "#### " + l[3:] // stray heading inside a section, e.g. "## Aligned"
			}
			body = append(body, l)
		case strings.HasPrefix(l, "### "):
			inBody = true
			body = append(body, l)
		case date == "" && dateLineRe.MatchString(l):
			date = l
		case title == "" && strings.HasPrefix(l, "## "):
			title = l[3:]
		case strings.HasPrefix(l, "Invited:"):
			invited = l
		case strings.HasPrefix(l, "Attachments "), strings.HasPrefix(l, "Meeting records "):
		default:
			body = append(body, l)
		}
	}
	if title == "" {
		title = fallbackTitle
	}
	if date == "" {
		date = fallbackDateLine
	}

	var transcript []string
	seenTitle := false
	for _, l := range tabs["Transcript"] {
		if !seenTitle && dateLineRe.MatchString(l) {
			continue
		}
		seenTitle = seenTitle || strings.HasPrefix(l, "## ")
		transcript = append(transcript, l)
	}

	head := []string{"## " + title, "", date, "", invited, ""}
	fullLines := append(append(append(append([]string{}, head...), body...), ""), transcript...)
	return Result{
		Summary: render(append(head, section(body, "### Details")...)...),
		Full:    render(fullLines...),
	}
}

// splitTabs groups cleaned lines under their top-level "# " tab heading.
// Keys are "Quick notes", "Full notes", "Transcript" when recognized.
func splitTabs(lines []string) map[string][]string {
	tabs := map[string][]string{}
	cur := ""
	for _, raw := range lines {
		l, keep := cleanLine(raw)
		if !keep {
			continue
		}
		if strings.HasPrefix(l, "# ") {
			cur = l
			for _, name := range []string{"Quick notes", "Full notes", "Transcript"} {
				if strings.Contains(l, name) {
					cur = name
				}
			}
			continue
		}
		tabs[cur] = append(tabs[cur], l)
	}
	return tabs
}

// section returns the lines from the heading through the next "### " heading.
func section(lines []string, heading string) []string {
	for i, l := range lines {
		if l != heading {
			continue
		}
		end := i + 1
		for end < len(lines) && !strings.HasPrefix(lines[end], "### ") {
			end++
		}
		return lines[i:end]
	}
	return nil
}

// cleanLine applies the general cleanup to one raw line. keep is false for
// boilerplate, image definitions, and headings with no text.
func cleanLine(l string) (string, bool) {
	if imageDefRe.MatchString(l) {
		return "", false
	}
	for _, b := range boilerplate {
		if strings.Contains(l, b) {
			return "", false
		}
	}
	if strings.HasPrefix(l, "Invited ") {
		var names []string
		for _, m := range inviteRe.FindAllStringSubmatch(l, -1) {
			names = append(names, m[1]+escapeRe.ReplaceAllString(m[2], "$1")+m[3])
		}
		if len(names) > 0 {
			return "Invited: " + strings.Join(names, ", "), true
		}
	}
	l = imageRe.ReplaceAllString(l, "")
	l = linkRe.ReplaceAllString(l, "$1")
	if m := headingRe.FindStringSubmatch(l); m != nil {
		text := strings.TrimSpace(anchorRe.ReplaceAllString(strings.ReplaceAll(m[2], "**", ""), ""))
		if text == "" {
			return "", false
		}
		l = m[1] + " " + text
	}
	l = escapeRe.ReplaceAllString(l, "$1")
	return strings.TrimRight(l, " \t"), true
}

// render joins lines, collapsing blank runs to one and trimming the ends.
func render(lines ...string) string {
	var b strings.Builder
	blank := true // suppress leading blanks
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blank = true
			continue
		}
		if blank && b.Len() > 0 {
			b.WriteString("\n")
		}
		blank = false
		b.WriteString(l)
		b.WriteString("\n")
	}
	return b.String()
}
