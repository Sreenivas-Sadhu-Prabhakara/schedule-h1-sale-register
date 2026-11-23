package h1register

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"ordinary date", "2026-04-10", "2026-04-10", false},
		{"leap day in a leap year", "2024-02-29", "2024-02-29", false},
		{"last day of a long month", "2026-01-31", "2026-01-31", false},
		{"blank is the zero date, not an error", "", "", false},
		{"surrounding whitespace is trimmed", "  2026-04-10 ", "2026-04-10", false},
		{"leap day in a non-leap year is rejected", "2026-02-29", "", true},
		{"31 April is rejected", "2026-04-31", "", true},
		{"month 13 is rejected", "2026-13-01", "", true},
		{"day 0 is rejected", "2026-04-00", "", true},
		{"wrong separator is rejected", "10/04/2026", "", true},
		{"unpadded month is rejected", "2026-4-10", "", true},
		{"free text is rejected", "yesterday", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDate(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseDate(%q) = %v, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q) returned %v", tc.in, err)
			}
			if got.String() != tc.want {
				t.Errorf("ParseDate(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDateCompareAndOrder(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		wantSign int
	}{
		{"same day", "2026-04-10", "2026-04-10", 0},
		{"earlier day", "2026-04-09", "2026-04-10", -1},
		{"later day", "2026-04-11", "2026-04-10", 1},
		{"earlier month", "2026-03-31", "2026-04-01", -1},
		{"earlier year", "2025-12-31", "2026-01-01", -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, b := MustParseDate(tc.a), MustParseDate(tc.b)
			got := a.Compare(b)
			switch {
			case tc.wantSign == 0 && got != 0:
				t.Errorf("Compare = %d, want 0", got)
			case tc.wantSign < 0 && got >= 0:
				t.Errorf("Compare = %d, want negative", got)
			case tc.wantSign > 0 && got <= 0:
				t.Errorf("Compare = %d, want positive", got)
			}
			if a.Before(b) != (tc.wantSign < 0) {
				t.Errorf("Before = %v", a.Before(b))
			}
			if a.After(b) != (tc.wantSign > 0) {
				t.Errorf("After = %v", a.After(b))
			}
			if a.Equal(b) != (tc.wantSign == 0) {
				t.Errorf("Equal = %v", a.Equal(b))
			}
		})
	}
}

func TestDaysUntil(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want int
	}{
		{"same day", "2026-04-10", "2026-04-10", 0},
		{"next day", "2026-04-10", "2026-04-11", 1},
		{"backwards", "2026-04-11", "2026-04-10", -1},
		{"across February in a non-leap year", "2026-02-01", "2026-03-01", 28},
		{"across February in a leap year", "2024-02-01", "2024-03-01", 29},
		{"a non-leap year", "2026-01-01", "2027-01-01", 365},
		{"a leap year", "2024-01-01", "2025-01-01", 366},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MustParseDate(tc.from).DaysUntil(MustParseDate(tc.to))
			if got != tc.want {
				t.Errorf("DaysUntil = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestAddYearsClampsToRealDays(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"leap day to non-leap year clamps to the 28th", "2024-02-29", 1, "2025-02-28"},
		{"leap day to the next leap year keeps the 29th", "2024-02-29", 4, "2028-02-29"},
		{"zero years is a no-op", "2024-02-29", 0, "2024-02-29"},
		{"backwards from a leap day", "2024-02-29", -1, "2023-02-28"},
		{"31 January is safe", "2026-01-31", 3, "2029-01-31"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MustParseDate(tc.in).AddYears(tc.n)
			if got.String() != tc.want {
				t.Errorf("AddYears(%d) on %s = %s, want %s", tc.n, tc.in, got, tc.want)
			}
		})
	}
	if !(Date{}).AddYears(3).IsZero() {
		t.Error("AddYears on the zero date should stay zero")
	}
}

func TestAddYearsDiffersFromTimeAddDateOnALeapDay(t *testing.T) {
	// Proves the clamp is doing something: the standard library spills over
	// into March, which is why this package does its own year arithmetic.
	spill := time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC).AddDate(3, 0, 0)
	if got := spill.Format(DateLayout); got != "2027-03-01" {
		t.Fatalf("expected the standard library to normalise to 2027-03-01, got %s", got)
	}
	if got := MustParseDate("2024-02-29").AddYears(3).String(); got != "2027-02-28" {
		t.Fatalf("AddYears = %s, want the clamped 2027-02-28", got)
	}
}

func TestAddDays(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"into the next month", "2026-01-31", 1, "2026-02-01"},
		{"over a leap day", "2024-02-28", 1, "2024-02-29"},
		{"over a missing leap day", "2026-02-28", 1, "2026-03-01"},
		{"backwards over a year end", "2026-01-01", -1, "2025-12-31"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MustParseDate(tc.in).AddDays(tc.n).String(); got != tc.want {
				t.Errorf("AddDays(%d) on %s = %s, want %s", tc.n, tc.in, got, tc.want)
			}
		})
	}
}

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		y    int
		m    time.Month
		want int
	}{
		{2026, time.January, 31},
		{2026, time.February, 28},
		{2024, time.February, 29},
		{2000, time.February, 29},
		{1900, time.February, 28},
		{2026, time.April, 30},
		{2026, time.December, 31},
	}
	for _, tc := range tests {
		if got := DaysInMonth(tc.y, tc.m); got != tc.want {
			t.Errorf("DaysInMonth(%d, %v) = %d, want %d", tc.y, tc.m, got, tc.want)
		}
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	type wrapper struct {
		D Date `json:"d"`
	}
	for _, in := range []string{"2026-04-10", "2024-02-29", ""} {
		b, err := json.Marshal(wrapper{D: MustParseDate(in)})
		if err != nil {
			t.Fatalf("Marshal(%q): %v", in, err)
		}
		var out wrapper
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("Unmarshal(%s): %v", b, err)
		}
		if out.D.String() != in {
			t.Errorf("round trip of %q gave %q (json %s)", in, out.D, b)
		}
	}

	var out struct {
		D Date `json:"d"`
	}
	if err := json.Unmarshal([]byte(`{"d":"2026-02-30"}`), &out); err == nil {
		t.Error("Unmarshal accepted 30 February")
	}
}

func TestTodayIsAValidDate(t *testing.T) {
	d := Today()
	if d.IsZero() {
		t.Fatal("Today() is zero")
	}
	if _, err := ParseDate(d.String()); err != nil {
		t.Errorf("Today() renders %q which does not parse: %v", d, err)
	}
}
