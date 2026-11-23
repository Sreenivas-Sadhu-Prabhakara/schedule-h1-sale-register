package h1register

import (
	"strings"
	"testing"
)

// serials builds a register holding just the given serial numbers.
func serials(ns ...int) []Entry {
	out := make([]Entry, 0, len(ns))
	for _, n := range ns {
		e := complete()
		e.Serial = n
		out = append(out, e)
	}
	return out
}

func TestNextSerial(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		want    int
	}{
		{"empty register starts at 1", nil, 1},
		{"after one entry", serials(1), 2},
		{"after a clean run", serials(1, 2, 3, 4), 5},
		{"out-of-order storage still uses the highest", serials(3, 1, 2), 4},
		{"a gap is not back-filled", serials(1, 2, 5), 6},
		{"a continuation volume keeps counting", serials(120, 121), 122},
		{"a duplicate does not rewind", serials(1, 2, 2), 3},
		{"an invalid serial is ignored", serials(0, -3), 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextSerial(tc.entries); got != tc.want {
				t.Errorf("NextSerial() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCheckContinuity(t *testing.T) {
	tests := []struct {
		name           string
		entries        []Entry
		wantOK         bool
		wantGaps       []int
		wantDuplicates []int
		wantInvalid    []int
		wantFirst      int
		wantLast       int
	}{
		{
			name:      "empty register is clean",
			entries:   nil,
			wantOK:    true,
			wantFirst: 0,
			wantLast:  0,
		},
		{
			name:      "clean run of five",
			entries:   serials(1, 2, 3, 4, 5),
			wantOK:    true,
			wantFirst: 1,
			wantLast:  5,
		},
		{
			name:      "clean run stored out of order",
			entries:   serials(4, 1, 5, 3, 2),
			wantOK:    true,
			wantFirst: 1,
			wantLast:  5,
		},
		{
			name:      "one gap",
			entries:   serials(1, 2, 4, 5),
			wantOK:    false,
			wantGaps:  []int{3},
			wantFirst: 1,
			wantLast:  5,
		},
		{
			name:      "several gaps are all reported",
			entries:   serials(1, 4, 7),
			wantOK:    false,
			wantGaps:  []int{2, 3, 5, 6},
			wantFirst: 1,
			wantLast:  7,
		},
		{
			name:           "one duplicate",
			entries:        serials(1, 2, 3, 3),
			wantOK:         false,
			wantDuplicates: []int{3},
			wantFirst:      1,
			wantLast:       3,
		},
		{
			name:           "a gap and a duplicate together",
			entries:        serials(1, 2, 2, 5),
			wantOK:         false,
			wantGaps:       []int{3, 4},
			wantDuplicates: []int{2},
			wantFirst:      1,
			wantLast:       5,
		},
		{
			name:        "non-positive serial is invalid",
			entries:     serials(1, 2, 0),
			wantOK:      false,
			wantInvalid: []int{0},
			wantFirst:   1,
			wantLast:    2,
		},
		{
			name:      "continuation volume starting at 120 is clean",
			entries:   serials(120, 121, 122),
			wantOK:    true,
			wantFirst: 120,
			wantLast:  122,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckContinuity(tc.entries)
			if got.OK != tc.wantOK {
				t.Errorf("OK = %v, want %v (summary: %s)", got.OK, tc.wantOK, got.Summary)
			}
			if !equalInts(got.Gaps, tc.wantGaps) {
				t.Errorf("Gaps = %v, want %v", got.Gaps, tc.wantGaps)
			}
			if !equalInts(got.Duplicates, tc.wantDuplicates) {
				t.Errorf("Duplicates = %v, want %v", got.Duplicates, tc.wantDuplicates)
			}
			if !equalInts(got.Invalid, tc.wantInvalid) {
				t.Errorf("Invalid = %v, want %v", got.Invalid, tc.wantInvalid)
			}
			if got.First != tc.wantFirst || got.Last != tc.wantLast {
				t.Errorf("First,Last = %d,%d, want %d,%d", got.First, got.Last, tc.wantFirst, tc.wantLast)
			}
			if got.Count != len(tc.entries) {
				t.Errorf("Count = %d, want %d", got.Count, len(tc.entries))
			}
			if got.Summary == "" {
				t.Error("Summary is empty; the banner has nothing to show")
			}
		})
	}
}

func TestContinuitySummarySaysItLoudly(t *testing.T) {
	tests := []struct {
		name     string
		entries  []Entry
		wantIn   []string
		wantNoIn []string
	}{
		{
			name:     "a gap is called an inspection failure and the serial is named",
			entries:  serials(1, 2, 4),
			wantIn:   []string{"broken", "3", "inspection failure"},
			wantNoIn: []string{"unbroken"},
		},
		{
			name:     "a duplicate is named",
			entries:  serials(1, 2, 2),
			wantIn:   []string{"used more than once", "2", "inspection failure"},
			wantNoIn: []string{"unbroken"},
		},
		{
			name:     "a clean run reads as unbroken",
			entries:  serials(1, 2, 3),
			wantIn:   []string{"unbroken", "1", "3"},
			wantNoIn: []string{"inspection failure"},
		},
		{
			name:    "an empty register says the next serial",
			entries: nil,
			wantIn:  []string{"empty", "serial 1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := CheckContinuity(tc.entries).Summary
			for _, w := range tc.wantIn {
				if !strings.Contains(s, w) {
					t.Errorf("summary %q does not mention %q", s, w)
				}
			}
			for _, w := range tc.wantNoIn {
				if strings.Contains(s, w) {
					t.Errorf("summary %q should not mention %q", s, w)
				}
			}
		})
	}
}

func TestStartsAtFirstSerial(t *testing.T) {
	if !CheckContinuity(serials(1, 2)).StartsAtFirstSerial {
		t.Error("a register starting at 1 should report StartsAtFirstSerial")
	}
	if CheckContinuity(serials(7, 8)).StartsAtFirstSerial {
		t.Error("a register starting at 7 should not report StartsAtFirstSerial")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
