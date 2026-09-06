package main

import (
	"context"
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
