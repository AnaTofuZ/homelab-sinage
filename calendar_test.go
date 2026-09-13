package main

import (
	"strings"
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"
)

func TestUpcomingCalendarEvents(t *testing.T) {
	calendar, err := ics.ParseCalendar(strings.NewReader("BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" +
		"BEGIN:VEVENT\r\nUID:all-day\r\nDTSTART;VALUE=DATE:20260906\r\nDTEND;VALUE=DATE:20260907\r\nSUMMARY:休暇\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:daily\r\nDTSTART;TZID=Asia/Tokyo:20260906T100000\r\nDTEND;TZID=Asia/Tokyo:20260906T103000\r\nRRULE:FREQ=DAILY;COUNT=3\r\nSUMMARY:朝会\r\nLOCATION:オンライン\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:daily\r\nRECURRENCE-ID;TZID=Asia/Tokyo:20260907T100000\r\nSTATUS:CANCELLED\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:tomorrow\r\nDTSTART;TZID=Asia/Tokyo:20260907T130000\r\nDTEND;TZID=Asia/Tokyo:20260907T140000\r\nSUMMARY:翌日の予定\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 6, 9, 0, 0, 0, tokyo)
	events := upcomingCalendarEvents(calendar, now)
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3: %#v", len(events), events)
	}
	if events[0].Title != "休暇" || events[0].Time != "終日" || events[0].EndTime != "" || events[0].Day != "today" {
		t.Errorf("unexpected all-day event: %#v", events[0])
	}
	if events[1].Title != "朝会" || events[1].Time != "10:00" || events[1].EndTime != "10:30" || events[1].Day != "today" || events[1].Location != "オンライン" {
		t.Errorf("unexpected recurring event: %#v", events[1])
	}
	if events[1].StartsAt != "2026-09-06T10:00:00+09:00" {
		t.Errorf("StartsAt = %q, want RFC3339 start time", events[1].StartsAt)
	}
	if events[2].Title != "翌日の予定" || events[2].Time != "13:00" || events[2].EndTime != "14:00" || events[2].Day != "tomorrow" {
		t.Errorf("unexpected tomorrow event: %#v", events[2])
	}
}
