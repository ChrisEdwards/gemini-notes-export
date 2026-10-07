package notes

import (
	"testing"
	"time"
)

func TestFileNameAndDateFolder(t *testing.T) {
	modified := time.Date(2026, 3, 4, 12, 0, 0, 0, time.Local)
	tests := []struct {
		drive, file, folder, title string
	}{
		{"Widget Guild - 2026/10/06 15:00 EDT - Notes by Gemini", "Widget Guild - 2026_10_06 15_00 EDT - Notes by Gemini.md", "2026-10-06", "Widget Guild"},
		{"Alice / Bob 1:1 - 2026/10/07 12:00 PDT - Notes by Gemini", "Alice _ Bob 1_1 - 2026_10_07 12_00 PDT - Notes by Gemini.md", "2026-10-07", "Alice / Bob 1:1"},
		{`Q&A <"x"|y?*> - 2026/01/02 09:00 CET - Notes by Gemini`, `Q&A __x__y___ - 2026_01_02 09_00 CET - Notes by Gemini.md`, "2026-01-02", `Q&A <"x"|y?*>`},
		{"Untitled meeting - Notes by Gemini", "Untitled meeting - Notes by Gemini.md", "2026-03-04", "Untitled meeting"},
	}
	for _, tt := range tests {
		if got := FileName(tt.drive); got != tt.file {
			t.Errorf("FileName(%q) = %q, want %q", tt.drive, got, tt.file)
		}
		if got := DateFolder(FileName(tt.drive), modified); got != tt.folder {
			t.Errorf("DateFolder(%q) = %q, want %q", tt.drive, got, tt.folder)
		}
		if got := TitleFromName(tt.drive); got != tt.title {
			t.Errorf("TitleFromName(%q) = %q, want %q", tt.drive, got, tt.title)
		}
	}
}
