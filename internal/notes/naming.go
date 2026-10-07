package notes

import (
	"regexp"
	"strings"
	"time"
)

var unsafeChars = strings.NewReplacer("/", "_", ":", "_", "<", "_", ">", "_", `"`, "_", "|", "_", "?", "_", "*", "_")

// FileName turns a Drive doc name into the output file name.
func FileName(driveName string) string {
	return unsafeChars.Replace(driveName) + ".md"
}

var nameDateRe = regexp.MustCompile(`\d{4}_\d{2}_\d{2}`)

// DateFolder returns the YYYY-MM-DD folder for a sanitized file name: the first
// YYYY_MM_DD in it (the meeting date in the organizer's time zone), else the
// local date of modified.
func DateFolder(fileName string, modified time.Time) string {
	if m := nameDateRe.FindString(fileName); m != "" {
		return strings.ReplaceAll(m, "_", "-")
	}
	return modified.Local().Format(time.DateOnly)
}

var meetingNameRe = regexp.MustCompile(`^(.*?) - \d{4}/\d{2}/\d{2}\b`)

// TitleFromName derives a meeting title from a Drive name like
// "Team Sync - 2026/10/06 15:00 EDT - Notes by Gemini".
func TitleFromName(driveName string) string {
	if m := meetingNameRe.FindStringSubmatch(driveName); m != nil {
		return m[1]
	}
	return strings.TrimSuffix(driveName, " - Notes by Gemini")
}
