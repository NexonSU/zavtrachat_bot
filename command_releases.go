package main

import (
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	ical "github.com/arran4/golang-ical"
)

func countRune(s string, r rune) int {
	count := 0
	for _, c := range s {
		if c == r {
			count++
		}
	}
	return count
}

// Send releases of 2 weeks on /releases
func Releases(bot *gotgbot.Bot, context *ext.Context) error {
	resp, err := httpClient().Get(Config.ReleasesUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("releases feed %v: %v", Config.ReleasesUrl, resp.Status)
	}
	cal, err := ical.ParseCalendar(resp.Body)
	if err != nil {
		return err
	}
	releases := releaseCalendarText(cal, time.Now())
	_, err = context.Message.Reply(bot, releases, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
	return err
}

// Skip malformed events before sorting or slicing dates. Calendar timestamps
// and all-day dates are both accepted; output remains one entry per title.
func releaseCalendarText(cal *ical.Calendar, now time.Time) string {
	type release struct {
		date time.Time
		name string
	}
	var entries []release
	for _, event := range cal.Events() {
		date, name := event.GetProperty(ical.ComponentPropertyDtStart), event.GetProperty(ical.ComponentPropertySummary)
		if date == nil || name == nil {
			continue
		}
		title := strings.TrimSpace(strings.Split(name.Value, " (")[0])
		if title == "" {
			continue
		}
		var parsed time.Time
		for _, layout := range []string{"20060102", "20060102T150405Z", "20060102T150405"} {
			value, err := time.ParseInLocation(layout, date.Value, now.Location())
			if err == nil {
				parsed = value
				break
			}
		}
		cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -2)
		if parsed.IsZero() || !parsed.After(cutoff) {
			continue
		}
		entries = append(entries, release{parsed, title})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].date.Before(entries[j].date) })
	seen := make(map[string]bool)
	lines := []string{}
	for _, entry := range entries {
		if seen[entry.name] {
			continue
		}
		seen[entry.name] = true
		lines = append(lines, fmt.Sprintf("%s: %s", entry.date.Format("02.01.2006"), html.EscapeString(entry.name)))
		if len(lines) == 20 {
			break
		}
	}
	if len(lines) == 0 {
		return "В календаре пока нет ближайших релизов."
	}
	return strings.Join(lines, "\n")
}
