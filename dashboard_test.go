package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWeatherLabel(t *testing.T) {
	tests := map[int]string{0: "快晴", 3: "晴れ／曇り", 45: "霧", 61: "雨", 75: "雪", 81: "にわか雨", 95: "雷雨"}
	for code, want := range tests {
		if got := weatherLabel(code); got != want {
			t.Errorf("weatherLabel(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestParseWeatherAlertsForKofu(t *testing.T) {
	const body = `[
		{"headlineText":"土砂災害に注意してください。","warning":{"class20Items":[{"areaCode":"1920100","kinds":[{"code":"29","status":"発表"}]}]}},
		{"headlineText":"解除します。","warning":{"class20Items":[{"areaCode":"1920100","kinds":[{"code":"15","status":"解除"}]}]}},
		{"headlineText":"別の地域です。","warning":{"class20Items":[{"areaCode":"1920200","kinds":[{"code":"03","status":"発表"}]}]}}
	]`
	var reports []jmaWarningReport
	if err := json.Unmarshal([]byte(body), &reports); err != nil {
		t.Fatal(err)
	}
	alerts := parseWeatherAlerts(reports)
	if len(alerts) != 1 || alerts[0].Title != "土砂災害注意報" || alerts[0].Level != "advisory" || alerts[0].Detail != "土砂災害に注意してください。" {
		t.Fatalf("unexpected alerts: %#v", alerts)
	}
}

func TestWeatherAlertKindLevels(t *testing.T) {
	for code, want := range map[string]string{"29": "advisory", "09": "warning", "49": "danger", "39": "emergency"} {
		if _, got := weatherAlertKind(code); got != want {
			t.Errorf("weatherAlertKind(%q) level = %q, want %q", code, got, want)
		}
	}
}

func TestFetchNewsIncludesRSSDescription(t *testing.T) {
	feed := `<?xml version="1.0"?><rss><channel><item><title>見出し</title><link>https://example.com/news</link><description>ニュースの要約です。</description><pubDate>Sun, 06 Sep 2026 15:14:26 +0900</pubDate></item></channel></rss>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feed))
	}))
	defer server.Close()
	t.Setenv("NEWS_FEED_URL", server.URL)

	items, err := newDashboardService(&http.Client{Timeout: time.Second}).fetchNews(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Summary != "ニュースの要約です。" {
		t.Fatalf("unexpected news: %#v", items)
	}
}
