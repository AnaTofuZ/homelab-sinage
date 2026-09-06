package main

import "testing"

func TestWeatherLabel(t *testing.T) {
	tests := map[int]string{0: "快晴", 3: "晴れ／曇り", 45: "霧", 61: "雨", 75: "雪", 81: "にわか雨", 95: "雷雨"}
	for code, want := range tests {
		if got := weatherLabel(code); got != want {
			t.Errorf("weatherLabel(%d) = %q, want %q", code, got, want)
		}
	}
}
