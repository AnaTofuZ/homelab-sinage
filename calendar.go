package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/teambition/rrule-go"
)

type icalEvent struct {
	ID, Title, Location string
	Start, End          time.Time
	AllDay              bool
}

func (s *dashboardService) fetchCalendar(ctx context.Context) ([]calendarEvent, error) {
	endpoint := strings.TrimSpace(os.Getenv("GOOGLE_CALENDAR_ICAL_URL"))
	if endpoint == "" {
		return []calendarEvent{}, errors.New("未設定")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("iCal URLが不正")
	}
	req.Header.Set("Accept", "text/calendar")
	resp, err := s.client.Do(req)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			return nil, fmt.Errorf("iCal取得失敗: %w", urlErr.Err)
		}
		return nil, errors.New("iCal取得失敗")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("iCal HTTP %d", resp.StatusCode)
	}
	calendar, err := ics.ParseCalendar(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("iCal解析失敗: %w", err)
	}
	return upcomingCalendarEvents(calendar, time.Now().In(tokyo)), nil
}

func upcomingCalendarEvents(calendar *ics.Calendar, now time.Time) []calendarEvent {
	windowEnd := time.Date(now.Year(), now.Month(), now.Day()+2, 0, 0, 0, 0, tokyo)
	overrides := map[string]*ics.VEvent{}
	regular := make([]*ics.VEvent, 0, len(calendar.Events()))
	for _, event := range calendar.Events() {
		if recurrenceID, err := event.GetRecurrenceID(); err == nil {
			overrides[recurrenceKey(event.Id(), recurrenceID)] = event
		} else {
			regular = append(regular, event)
		}
	}

	items := make([]icalEvent, 0, 12)
	for _, event := range regular {
		if isCancelled(event) {
			continue
		}
		start, end, allDay, err := icalEventTimes(event)
		if err != nil {
			continue
		}
		for _, occurrence := range eventOccurrences(event, start, end, now, windowEnd) {
			if !overlaps(occurrence, occurrence.Add(end.Sub(start)), now, windowEnd) {
				continue
			}
			if replacement, ok := overrides[recurrenceKey(event.Id(), occurrence)]; ok {
				delete(overrides, recurrenceKey(event.Id(), occurrence))
				if !isCancelled(replacement) {
					items = appendICalEvent(items, replacement, now, windowEnd)
				}
				continue
			}
			items = append(items, newICalEvent(event, occurrence, occurrence.Add(end.Sub(start)), allDay))
		}
	}
	for _, event := range overrides {
		if !isCancelled(event) {
			items = appendICalEvent(items, event, now, windowEnd)
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Start.Before(items[j].Start) })
	if len(items) > 12 {
		items = items[:12]
	}
	result := make([]calendarEvent, 0, len(items))
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, tokyo)
	for _, item := range items {
		start, end := "終日", ""
		if !item.AllDay {
			start = item.Start.In(tokyo).Format("15:04")
			if item.End.After(item.Start) {
				end = item.End.In(tokyo).Format("15:04")
			}
		}
		day := "today"
		if !item.Start.Before(tomorrow) {
			day = "tomorrow"
		}
		result = append(result, calendarEvent{ID: item.ID, Title: item.Title, StartsAt: item.Start.In(tokyo).Format(time.RFC3339), Time: start, EndTime: end, Day: day, Location: item.Location, AllDay: item.AllDay})
	}
	return result
}

func eventOccurrences(event *ics.VEvent, start, end, windowStart, windowEnd time.Time) []time.Time {
	rules, err := event.GetRRules()
	if err != nil || len(rules) == 0 {
		if overlaps(start, end, windowStart, windowEnd) {
			return []time.Time{start}
		}
		return nil
	}
	rule, err := rrule.StrToRRule(rules[0].String())
	if err != nil {
		return nil
	}
	set := &rrule.Set{}
	set.DTStart(start)
	set.RRule(rule)
	if dates, err := event.GetRDates(); err == nil {
		set.SetRDates(dates)
	}
	if dates, err := event.GetExDates(); err == nil {
		set.SetExDates(dates)
	}
	return set.Between(windowStart.Add(-end.Sub(start)), windowEnd, true)
}

func appendICalEvent(items []icalEvent, event *ics.VEvent, windowStart, windowEnd time.Time) []icalEvent {
	start, end, allDay, err := icalEventTimes(event)
	if err == nil && overlaps(start, end, windowStart, windowEnd) {
		return append(items, newICalEvent(event, start, end, allDay))
	}
	return items
}

func newICalEvent(event *ics.VEvent, start, end time.Time, allDay bool) icalEvent {
	title := propertyValue(event, ics.ComponentPropertySummary)
	if title == "" {
		title = "（無題）"
	}
	return icalEvent{
		ID:       fmt.Sprintf("%s/%d", event.Id(), start.Unix()),
		Title:    title,
		Location: propertyValue(event, ics.ComponentPropertyLocation),
		Start:    start,
		End:      end,
		AllDay:   allDay,
	}
}

func icalEventTimes(event *ics.VEvent) (time.Time, time.Time, bool, error) {
	startProperty := event.GetProperty(ics.ComponentPropertyDtStart)
	if startProperty == nil {
		return time.Time{}, time.Time{}, false, errors.New("DTSTARTがありません")
	}
	allDay := len(startProperty.Value) == len("20060102")
	if allDay {
		start, err := time.ParseInLocation("20060102", startProperty.Value, tokyo)
		if err != nil {
			return time.Time{}, time.Time{}, true, err
		}
		end := start.AddDate(0, 0, 1)
		if endProperty := event.GetProperty(ics.ComponentPropertyDtEnd); endProperty != nil {
			if parsed, err := time.ParseInLocation("20060102", endProperty.Value, tokyo); err == nil {
				end = parsed
			}
		}
		return start, end, true, nil
	}
	start, err := event.GetStartAt()
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	end, err := event.GetEndAt()
	if err != nil {
		end = start
	}
	return start, end, false, nil
}

func overlaps(start, end, windowStart, windowEnd time.Time) bool {
	if end.After(start) {
		return end.After(windowStart) && start.Before(windowEnd)
	}
	return !start.Before(windowStart) && start.Before(windowEnd)
}

func recurrenceKey(id string, at time.Time) string {
	return fmt.Sprintf("%s/%d", id, at.Unix())
}

func propertyValue(event *ics.VEvent, property ics.ComponentProperty) string {
	if value := event.GetProperty(property); value != nil {
		return strings.TrimSpace(value.Value)
	}
	return ""
}

func isCancelled(event *ics.VEvent) bool {
	return propertyValue(event, ics.ComponentPropertyStatus) == string(ics.ObjectStatusCancelled)
}
