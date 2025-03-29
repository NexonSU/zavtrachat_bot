package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	ical "github.com/arran4/golang-ical"
	tele "gopkg.in/telebot.v3"
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
func Releases(context tele.Context) error {
	var err error
	resp, err := http.Get("https://ical-videogames.onrender.com/calendar?platform=ps4&platform=ps5&platform=switch&platform=xbox_one&platform=xbox_series&region=pal")
	if err != nil {
		return err
	}
	cal, err := ical.ParseCalendar(resp.Body)
	if err != nil {
		return err
	}
	releases := ""
	twodaysago, _ := strconv.Atoi(time.Now().AddDate(0, 0, -2).Format("20060102"))
	events := cal.Events()
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].GetProperty(ical.ComponentPropertyDtStart).Value < events[j].GetProperty(ical.ComponentPropertyDtStart).Value
	})
	for _, element := range events {
		date := element.GetProperty(ical.ComponentPropertyDtStart).Value
		name := element.GetProperty(ical.ComponentPropertySummary).Value
		name = strings.Split(name, " (")[0]
		if strings.Contains(releases, name) {
			continue
		}
		dateint, _ := strconv.Atoi(date)
		if dateint > twodaysago && countRune(releases, '\n') < 20 {
			releases = fmt.Sprintf("%v\n%v.%v.%v: %v", releases, date[6:8], date[4:6], date[0:4], name)
		}
	}
	return context.Reply(releases)
}
