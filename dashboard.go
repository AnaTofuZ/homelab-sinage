package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
)

var tokyo = mustLocation("Asia/Tokyo")

type dashboardData struct {
	GeneratedAt string           `json:"generatedAt"`
	Weather     weatherData      `json:"weather"`
	Forecast    []forecastDay    `json:"forecast"`
	Events      []calendarEvent  `json:"events"`
	News        []newsItem       `json:"news"`
	Attendance  attendanceStatus `json:"attendance"`
	Warnings    []string         `json:"warnings"`
}

type weatherData struct {
	Place       string  `json:"place"`
	Temperature float64 `json:"temperature"`
	Apparent    float64 `json:"apparent"`
	Code        int     `json:"code"`
	Condition   string  `json:"condition"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Rain        int     `json:"rain"`
}

type forecastDay struct {
	Date      string  `json:"date"`
	Weekday   string  `json:"weekday"`
	Code      int     `json:"code"`
	Condition string  `json:"condition"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Rain      int     `json:"rain"`
}

type calendarEvent struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Time     string `json:"time"`
	Location string `json:"location"`
	AllDay   bool   `json:"allDay"`
}

type newsItem struct {
	Title     string `json:"title"`
	URL       string `json:"url"`
	Published string `json:"published"`
}

type attendanceStatus struct {
	Available        bool   `json:"available"`
	State            string `json:"state"`
	WorkedSeconds    int64  `json:"workedSeconds"`
	TargetSeconds    int64  `json:"targetSeconds"`
	DayDifference    int64  `json:"dayDifference"`
	MonthDifference  int64  `json:"monthDifference"`
	ProjectedSeconds int64  `json:"projectedSeconds"`
}

type dashboardService struct {
	client *http.Client
	mu     sync.Mutex
	cache  dashboardData
	at     time.Time
}

func newDashboardService(client *http.Client) *dashboardService {
	return &dashboardService{client: client}
}

func (s *dashboardService) dashboard(ctx context.Context) dashboardData {
	s.mu.Lock()
	if time.Since(s.at) < 5*time.Minute {
		cached := s.cache
		s.mu.Unlock()
		return cached
	}
	s.mu.Unlock()

	result := dashboardData{GeneratedAt: time.Now().Format(time.RFC3339), Forecast: []forecastDay{}, Events: []calendarEvent{}, News: []newsItem{}}
	var weatherErr, calendarErr, newsErr, attendanceErr error
	group, ctx := errgroup.WithContext(ctx)
	// Source errors are dashboard data, not reasons to cancel the other panels.
	group.Go(func() error { result.Weather, result.Forecast, weatherErr = s.fetchWeather(ctx); return nil })
	group.Go(func() error { result.Events, calendarErr = s.fetchCalendar(ctx); return nil })
	group.Go(func() error { result.News, newsErr = s.fetchNews(ctx); return nil })
	group.Go(func() error { result.Attendance, attendanceErr = s.fetchAttendance(ctx); return nil })
	_ = group.Wait()
	for label, err := range map[string]error{"天気": weatherErr, "カレンダー": calendarErr, "ニュース": newsErr, "勤怠": attendanceErr} {
		if err != nil {
			result.Warnings = append(result.Warnings, label+": "+err.Error())
		}
	}
	s.mu.Lock()
	// ponytail: one shared cache is enough for one tablet; split per source if refresh rates diverge.
	if weatherErr != nil && !s.at.IsZero() {
		result.Weather, result.Forecast = s.cache.Weather, s.cache.Forecast
	}
	if calendarErr != nil && !s.at.IsZero() {
		result.Events = s.cache.Events
	}
	if newsErr != nil && !s.at.IsZero() {
		result.News = s.cache.News
	}
	if attendanceErr != nil && !s.at.IsZero() {
		result.Attendance = s.cache.Attendance
	}
	s.cache, s.at = result, time.Now()
	s.mu.Unlock()
	return result
}

func (s *dashboardService) fetchWeather(ctx context.Context) (weatherData, []forecastDay, error) {
	const endpoint = "https://api.open-meteo.com/v1/forecast?latitude=35.6639&longitude=138.5683&timezone=Asia%2FTokyo&forecast_days=4&current=temperature_2m,apparent_temperature,weather_code&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max"
	var payload struct {
		Current struct {
			Temperature float64 `json:"temperature_2m"`
			Apparent    float64 `json:"apparent_temperature"`
			Code        int     `json:"weather_code"`
		} `json:"current"`
		Daily struct {
			Time []string  `json:"time"`
			Code []int     `json:"weather_code"`
			High []float64 `json:"temperature_2m_max"`
			Low  []float64 `json:"temperature_2m_min"`
			Rain []int     `json:"precipitation_probability_max"`
		} `json:"daily"`
	}
	if err := s.getJSON(ctx, endpoint, &payload); err != nil {
		return weatherData{}, nil, err
	}
	forecast := make([]forecastDay, 0, len(payload.Daily.Time))
	for i, date := range payload.Daily.Time {
		if i >= len(payload.Daily.Code) || i >= len(payload.Daily.High) || i >= len(payload.Daily.Low) || i >= len(payload.Daily.Rain) {
			break
		}
		day, _ := time.Parse("2006-01-02", date)
		forecast = append(forecast, forecastDay{date, []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}[day.Weekday()], payload.Daily.Code[i], weatherLabel(payload.Daily.Code[i]), payload.Daily.High[i], payload.Daily.Low[i], payload.Daily.Rain[i]})
	}
	w := weatherData{Place: "KOFU", Temperature: payload.Current.Temperature, Apparent: payload.Current.Apparent, Code: payload.Current.Code, Condition: weatherLabel(payload.Current.Code)}
	if len(forecast) > 0 {
		w.High, w.Low, w.Rain = forecast[0].High, forecast[0].Low, forecast[0].Rain
	}
	return w, forecast, nil
}

func (s *dashboardService) fetchCalendar(ctx context.Context) ([]calendarEvent, error) {
	calendarID, credentials := os.Getenv("GOOGLE_CALENDAR_ID"), os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE")
	if calendarID == "" || credentials == "" {
		return []calendarEvent{}, errors.New("未設定")
	}
	secret, err := os.ReadFile(credentials)
	if err != nil {
		return nil, err
	}
	config, err := google.JWTConfigFromJSON(secret, "https://www.googleapis.com/auth/calendar.readonly")
	if err != nil {
		return nil, err
	}
	client := config.Client(ctx)
	now := time.Now().In(tokyo)
	end := time.Date(now.Year(), now.Month(), now.Day()+2, 0, 0, 0, 0, tokyo)
	query := url.Values{"singleEvents": {"true"}, "orderBy": {"startTime"}, "timeZone": {"Asia/Tokyo"}, "timeMin": {now.Format(time.RFC3339)}, "timeMax": {end.Format(time.RFC3339)}, "maxResults": {"12"}}
	endpoint := "https://www.googleapis.com/calendar/v3/calendars/" + url.PathEscape(calendarID) + "/events?" + query.Encode()
	var payload struct {
		Items []struct {
			ID, Summary, Location string
			Start                 struct{ Date, DateTime string }
		} `json:"items"`
	}
	if err := getJSONWith(ctx, client, endpoint, &payload); err != nil {
		return nil, err
	}
	events := make([]calendarEvent, 0, len(payload.Items))
	for _, item := range payload.Items {
		allDay := item.Start.DateTime == ""
		label := "終日"
		if !allDay {
			if parsed, err := time.Parse(time.RFC3339, item.Start.DateTime); err == nil {
				label = parsed.In(tokyo).Format("15:04")
			}
		}
		events = append(events, calendarEvent{ID: item.ID, Title: item.Summary, Time: label, Location: item.Location, AllDay: allDay})
	}
	return events, nil
}

func (s *dashboardService) fetchNews(ctx context.Context) ([]newsItem, error) {
	endpoint := os.Getenv("NEWS_FEED_URL")
	if endpoint == "" {
		endpoint = "https://www3.nhk.or.jp/rss/news/cat0.xml"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RSS HTTP %d", resp.StatusCode)
	}
	var feed struct {
		Channel struct {
			Items []struct {
				Title   string `xml:"title"`
				Link    string `xml:"link"`
				PubDate string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&feed); err != nil {
		return nil, err
	}
	items := make([]newsItem, 0, min(10, len(feed.Channel.Items)))
	for _, item := range feed.Channel.Items {
		published := ""
		if parsed, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
			published = parsed.In(tokyo).Format("15:04")
		}
		items = append(items, newsItem{strings.TrimSpace(item.Title), strings.TrimSpace(item.Link), published})
		if len(items) == 10 {
			break
		}
	}
	return items, nil
}

func (s *dashboardService) fetchAttendance(ctx context.Context) (attendanceStatus, error) {
	base := strings.TrimRight(os.Getenv("FLEX_TIMER_URL"), "/")
	if base == "" {
		return attendanceStatus{}, errors.New("未設定")
	}
	var status attendanceStatus
	if err := s.getJSON(ctx, base+"/v1/status", &status); err != nil {
		return status, err
	}
	status.Available = true
	return status, nil
}

func (s *dashboardService) attendanceAction(ctx context.Context, action string) (attendanceStatus, error) {
	base := strings.TrimRight(os.Getenv("FLEX_TIMER_URL"), "/")
	if base == "" {
		return attendanceStatus{}, errors.New("FLEX_TIMER_URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/actions/"+action, nil)
	if err != nil {
		return attendanceStatus{}, err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("FLEX_TIMER_TOKEN"))
	resp, err := s.client.Do(req)
	if err != nil {
		return attendanceStatus{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return attendanceStatus{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var status attendanceStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return status, err
	}
	status.Available = true
	s.mu.Lock()
	s.at = time.Time{}
	s.mu.Unlock()
	return status, nil
}

func (s *dashboardService) getJSON(ctx context.Context, endpoint string, target any) error {
	return getJSONWith(ctx, s.client, endpoint, target)
}

func getJSONWith(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(target)
}

func weatherLabel(code int) string {
	switch {
	case code == 0:
		return "快晴"
	case code <= 3:
		return "晴れ／曇り"
	case code <= 48:
		return "霧"
	case code <= 57:
		return "霧雨"
	case code <= 67:
		return "雨"
	case code <= 77:
		return "雪"
	case code <= 82:
		return "にわか雨"
	case code <= 86:
		return "にわか雪"
	default:
		return "雷雨"
	}
}

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return location
}
