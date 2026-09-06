package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

var tokyo = mustLocation("Asia/Tokyo")

type dashboardData struct {
	GeneratedAt string           `json:"generatedAt"`
	Weather     weatherData      `json:"weather"`
	Hourly      []hourlyWeather  `json:"hourly"`
	Alerts      []weatherAlert   `json:"alerts"`
	Forecast    []forecastDay    `json:"forecast"`
	Events      []calendarEvent  `json:"events"`
	News        []newsItem       `json:"news"`
	Attendance  attendanceStatus `json:"attendance"`
	Warnings    []string         `json:"warnings"`
}

type hourlyWeather struct {
	Time        string  `json:"time"`
	Code        int     `json:"code"`
	Condition   string  `json:"condition"`
	Temperature float64 `json:"temperature"`
	Rain        int     `json:"rain"`
	Precip      float64 `json:"precip"`
	Wind        float64 `json:"wind"`
}

type weatherAlert struct {
	Level  string `json:"level"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
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
	Summary   string `json:"summary"`
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

	result := dashboardData{GeneratedAt: time.Now().Format(time.RFC3339), Hourly: []hourlyWeather{}, Alerts: []weatherAlert{}, Forecast: []forecastDay{}, Events: []calendarEvent{}, News: []newsItem{}}
	var weatherErr, alertErr, calendarErr, newsErr, attendanceErr error
	group, ctx := errgroup.WithContext(ctx)
	// Source errors are dashboard data, not reasons to cancel the other panels.
	group.Go(func() error {
		result.Weather, result.Forecast, result.Hourly, weatherErr = s.fetchWeather(ctx)
		return nil
	})
	group.Go(func() error { result.Alerts, alertErr = s.fetchWeatherAlerts(ctx); return nil })
	group.Go(func() error { result.Events, calendarErr = s.fetchCalendar(ctx); return nil })
	group.Go(func() error { result.News, newsErr = s.fetchNews(ctx); return nil })
	group.Go(func() error { result.Attendance, attendanceErr = s.fetchAttendance(ctx); return nil })
	_ = group.Wait()
	for label, err := range map[string]error{"天気": weatherErr, "警報": alertErr, "カレンダー": calendarErr, "ニュース": newsErr, "勤怠": attendanceErr} {
		if err != nil {
			result.Warnings = append(result.Warnings, label+": "+err.Error())
		}
	}
	s.mu.Lock()
	// ponytail: one shared cache is enough for one tablet; split per source if refresh rates diverge.
	if weatherErr != nil && !s.at.IsZero() {
		result.Weather, result.Forecast, result.Hourly = s.cache.Weather, s.cache.Forecast, s.cache.Hourly
	}
	if alertErr != nil && !s.at.IsZero() {
		result.Alerts = s.cache.Alerts
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

func (s *dashboardService) fetchWeather(ctx context.Context) (weatherData, []forecastDay, []hourlyWeather, error) {
	const endpoint = "https://api.open-meteo.com/v1/forecast?latitude=35.6639&longitude=138.5683&timezone=Asia%2FTokyo&forecast_days=4&forecast_hours=12&current=temperature_2m,apparent_temperature,weather_code&hourly=temperature_2m,precipitation_probability,precipitation,weather_code,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max"
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
		Hourly struct {
			Time        []string  `json:"time"`
			Temperature []float64 `json:"temperature_2m"`
			Rain        []int     `json:"precipitation_probability"`
			Precip      []float64 `json:"precipitation"`
			Code        []int     `json:"weather_code"`
			Wind        []float64 `json:"wind_speed_10m"`
		} `json:"hourly"`
	}
	if err := s.getJSON(ctx, endpoint, &payload); err != nil {
		return weatherData{}, nil, nil, err
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
	hourly := make([]hourlyWeather, 0, 8)
	for i := 1; i < len(payload.Hourly.Time) && len(hourly) < 8; i++ {
		if i >= len(payload.Hourly.Code) || i >= len(payload.Hourly.Temperature) || i >= len(payload.Hourly.Rain) || i >= len(payload.Hourly.Precip) || i >= len(payload.Hourly.Wind) {
			break
		}
		hour, err := time.Parse("2006-01-02T15:04", payload.Hourly.Time[i])
		if err != nil {
			continue
		}
		hourly = append(hourly, hourlyWeather{hour.Format("15:04"), payload.Hourly.Code[i], weatherLabel(payload.Hourly.Code[i]), payload.Hourly.Temperature[i], payload.Hourly.Rain[i], payload.Hourly.Precip[i], payload.Hourly.Wind[i]})
	}
	return w, forecast, hourly, nil
}

type jmaWarningReport struct {
	Headline string `json:"headlineText"`
	Warning  struct {
		Items []struct {
			AreaCode string `json:"areaCode"`
			Kinds    []struct {
				Code   string `json:"code"`
				Status string `json:"status"`
			} `json:"kinds"`
		} `json:"class20Items"`
	} `json:"warning"`
}

func (s *dashboardService) fetchWeatherAlerts(ctx context.Context) ([]weatherAlert, error) {
	const endpoint = "https://www.jma.go.jp/bosai/warning/data/r8/190000.json"
	var reports []jmaWarningReport
	if err := s.getJSON(ctx, endpoint, &reports); err != nil {
		return nil, err
	}
	return parseWeatherAlerts(reports), nil
}

func parseWeatherAlerts(reports []jmaWarningReport) []weatherAlert {
	alerts := []weatherAlert{}
	seen := map[string]bool{}
	for _, report := range reports {
		for _, item := range report.Warning.Items {
			if item.AreaCode != "1920100" {
				continue
			}
			for _, kind := range item.Kinds {
				if kind.Code == "" || kind.Status == "解除" || kind.Status == "発表警報・注意報はなし" || seen[kind.Code] {
					continue
				}
				title, level := weatherAlertKind(kind.Code)
				alerts = append(alerts, weatherAlert{level, title, strings.TrimSpace(report.Headline)})
				seen[kind.Code] = true
			}
		}
	}
	return alerts
}

func weatherAlertKind(code string) (string, string) {
	names := map[string]string{
		"02": "暴風雪警報", "03": "大雨警報", "04": "洪水警報", "05": "暴風警報", "06": "大雪警報", "07": "波浪警報", "08": "高潮警報",
		"10": "大雨注意報", "12": "大雪注意報", "13": "風雪注意報", "14": "雷注意報", "15": "強風注意報", "16": "波浪注意報", "17": "融雪注意報", "18": "洪水注意報", "19": "高潮注意報", "20": "濃霧注意報", "21": "乾燥注意報", "22": "なだれ注意報", "23": "低温注意報", "24": "霜注意報", "25": "着氷注意報", "26": "着雪注意報",
		"32": "暴風雪特別警報", "33": "大雨特別警報", "35": "暴風特別警報", "36": "大雪特別警報", "37": "波浪特別警報", "38": "高潮特別警報",
	}
	level := "warning"
	if code >= "10" && code <= "26" {
		level = "advisory"
	}
	if code >= "32" && code <= "38" {
		level = "emergency"
	}
	if name := names[code]; name != "" {
		return name, level
	}
	return "気象警報・注意報", level
}

func (s *dashboardService) fetchNews(ctx context.Context) ([]newsItem, error) {
	endpoint := os.Getenv("NEWS_FEED_URL")
	if endpoint == "" {
		endpoint = "https://news.web.nhk/n-data/conf/na/rss/cat0.xml"
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
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				Description string `xml:"description"`
				PubDate     string `xml:"pubDate"`
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
		items = append(items, newsItem{
			Title:     strings.TrimSpace(item.Title),
			URL:       strings.TrimSpace(item.Link),
			Published: published,
			Summary:   strings.TrimSpace(item.Description),
		})
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
