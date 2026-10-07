package notes

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata")

func TestTransformGolden(t *testing.T) {
	date := time.Date(2026, 1, 5, 9, 0, 0, 0, time.Local)
	tests := []struct {
		name        string
		wantWarning bool
	}{
		{"quicknotes", false},
		{"decisions", false},
		{"notes-tab", false},
		{"no-full-notes", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, err := os.ReadFile(filepath.Join("testdata", tt.name+".md"))
			if err != nil {
				t.Fatal(err)
			}
			got := Transform(string(in), "Fallback Title", date)
			if (got.Warning != "") != tt.wantWarning {
				t.Errorf("warning = %q, want warning: %v", got.Warning, tt.wantWarning)
			}
			golden(t, tt.name+".summary.golden", got.Summary)
			golden(t, tt.name+".full.golden", got.Full)
		})
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// The specific rules, asserted directly so the goldens can't silently drift.
func TestTransformRules(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "quicknotes.md"))
	if err != nil {
		t.Fatal(err)
	}
	r := Transform(string(in), "Fallback Title", time.Now())
	for _, want := range []string{
		"## Widget Guild\n\nOct 6, 2026\n\nInvited: Alice Example, Bob Sample-Person, ~~Carol Test~~\n\n### Summary",
		"### Next steps",
		"- [ ] [Alice Example] Ship widgets",
		"- [ ] [The group] Review #widgets channel: Read it ~weekly.",
		"via the widget tracker.",
		"widgets (round ones) for Q4. Cost is 5+ units!",
		`Keep \*stars\* and \_underscores\_ escaped. Run ` + "`make`" + ` for $5 & up.`,
		"**Widget Planning**\n\nThe team planned",
		"opened with widgets (00:01:04).",
		"gadgets and > quoted things.",
		"## Widget Guild - Transcript\n\n### 00:01:04\n\n**Alice Example:** Hello everyone.\n\n**Bob",
		"### Transcription ended after 00:05:00\n",
	} {
		if !strings.Contains(r.Full, want) {
			t.Errorf("full missing %q", want)
		}
	}
	for _, bad := range []string{
		"Quick notes", "should not appear", "survey", "review Gemini", "Attachments", "Meeting records",
		"image", "data:", "{#", "**Summary**", "computer generated", "](", "\n\n\n", "  \n", "Oct 6, 2026\n\n## Widget Guild - Transcript",
	} {
		if strings.Contains(r.Full, bad) {
			t.Errorf("full contains %q", bad)
		}
	}
	if strings.Contains(r.Summary, "### Summary") || strings.Contains(r.Summary, "### Next steps") || !strings.Contains(r.Summary, "### Details") {
		t.Errorf("summary should hold only the Details section:\n%s", r.Summary)
	}
	if strings.Contains(r.Summary, "Transcript") {
		t.Errorf("summary contains transcript:\n%s", r.Summary)
	}
}

func TestTransformDemotesStrayHeadings(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "decisions.md"))
	if err != nil {
		t.Fatal(err)
	}
	r := Transform(string(in), "Fallback Title", time.Now())
	if !strings.HasPrefix(r.Full, "## Gadget Standup\n") {
		t.Errorf("title wrong:\n%s", r.Full)
	}
	if !strings.Contains(r.Full, "### Decisions\n\n#### Aligned\n") || strings.Contains(r.Full, "\n## Aligned") {
		t.Errorf("## Aligned not demoted:\n%s", r.Full)
	}
}

func TestTransformBlankTitleFallsBack(t *testing.T) {
	doc := "# **📝 Full notes**\n\nOct 6, 2026\n\n## \n\n### **Details**\n\n## Stray\n\nText.\n"
	r := Transform(doc, "Fallback Title", time.Now())
	if !strings.HasPrefix(r.Summary, "## Fallback Title\n\nOct 6, 2026\n\n### Details\n\n#### Stray\n") {
		t.Errorf("summary = %q", r.Summary)
	}
}
