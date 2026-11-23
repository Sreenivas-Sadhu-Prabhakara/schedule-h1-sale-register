package h1register

import (
	"strings"
	"testing"
)

func TestRetentionYearsIsThree(t *testing.T) {
	if RetentionYears != 3 {
		t.Fatalf("RetentionYears = %d, want 3 (the Schedule H1 retention period)", RetentionYears)
	}
}

func TestRetentionUntil(t *testing.T) {
	tests := []struct {
		name   string
		supply string
		want   string
	}{
		{"ordinary date", "2026-04-10", "2029-04-10"},
		{"start of year", "2026-01-01", "2029-01-01"},
		{"end of year", "2026-12-31", "2029-12-31"},
		{"29 February falls on 28 February three years on", "2024-02-29", "2027-02-28"},
		{"28 February of a leap year is unremarkable", "2024-02-28", "2027-02-28"},
		{"1 March after a leap day", "2024-03-01", "2027-03-01"},
		{"leap day three years before another leap year", "2020-02-29", "2023-02-28"},
		{"a leap year inside the period does not shift the day", "2027-06-15", "2030-06-15"},
		{"31 March keeps its day", "2026-03-31", "2029-03-31"},
		{"30 April keeps its day", "2026-04-30", "2029-04-30"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RetentionUntil(MustParseDate(tc.supply))
			if got.String() != tc.want {
				t.Errorf("RetentionUntil(%s) = %s, want %s", tc.supply, got, tc.want)
			}
		})
	}
}

func TestRetentionUntilIsCalendarNotFixedDays(t *testing.T) {
	// Three calendar years spanning a leap day is 1096 days, not 1095. A
	// fixed-day implementation would land on 2029-04-09 here.
	supply := MustParseDate("2026-04-10")
	until := RetentionUntil(supply)
	if days := supply.DaysUntil(until); days != 1096 {
		t.Fatalf("2026-04-10 to %s is %d days, want 1096 (2028 is a leap year)", until, days)
	}
}

func TestRetentionStatus(t *testing.T) {
	tests := []struct {
		name           string
		supply         string
		asOf           string
		wantStatus     string
		wantDaysLeft   int
		wantMustRetain bool
	}{
		{
			name: "the day of supply", supply: "2026-04-10", asOf: "2026-04-10",
			wantStatus: "Must retain", wantDaysLeft: 1096, wantMustRetain: true,
		},
		{
			name: "one year in", supply: "2026-04-10", asOf: "2027-04-10",
			wantStatus: "Must retain", wantDaysLeft: 731, wantMustRetain: true,
		},
		{
			name: "the day before the period ends", supply: "2026-04-10", asOf: "2029-04-09",
			wantStatus: "Must retain", wantDaysLeft: 1, wantMustRetain: true,
		},
		{
			name: "the last day of the period", supply: "2026-04-10", asOf: "2029-04-10",
			wantStatus: "Must retain", wantDaysLeft: 0, wantMustRetain: true,
		},
		{
			name: "the day after the period ends", supply: "2026-04-10", asOf: "2029-04-11",
			wantStatus: "May destroy", wantDaysLeft: -1, wantMustRetain: false,
		},
		{
			name: "long past", supply: "2019-07-01", asOf: "2026-11-24",
			wantStatus: "May destroy", wantDaysLeft: -1607, wantMustRetain: false,
		},
		{
			name: "leap-day supply on its last retention day", supply: "2024-02-29", asOf: "2027-02-28",
			wantStatus: "Must retain", wantDaysLeft: 0, wantMustRetain: true,
		},
		{
			name: "leap-day supply the day after", supply: "2024-02-29", asOf: "2027-03-01",
			wantStatus: "May destroy", wantDaysLeft: -1, wantMustRetain: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RetentionStatus(MustParseDate(tc.supply), MustParseDate(tc.asOf))
			if got.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tc.wantStatus)
			}
			if got.DaysLeft != tc.wantDaysLeft {
				t.Errorf("DaysLeft = %d, want %d", got.DaysLeft, tc.wantDaysLeft)
			}
			if got.MustRetain != tc.wantMustRetain {
				t.Errorf("MustRetain = %v, want %v", got.MustRetain, tc.wantMustRetain)
			}
			if got.MayDestroy == got.MustRetain {
				t.Errorf("MayDestroy (%v) must be the opposite of MustRetain (%v)", got.MayDestroy, got.MustRetain)
			}
			if got.Detail == "" {
				t.Error("Detail is empty")
			}
		})
	}
}

func TestRetentionStatusWithBlankDateKeepsTheRecord(t *testing.T) {
	got := RetentionStatus(Date{}, MustParseDate("2026-11-24"))
	if !got.MustRetain || got.MayDestroy {
		t.Fatalf("a blank date of supply must fall back to Must retain, got %+v", got)
	}
	if !strings.Contains(got.Detail, "blank") {
		t.Errorf("Detail = %q, want it to say the date is blank", got.Detail)
	}
}

func TestEntryRetentionUsesDateOfSupply(t *testing.T) {
	e := complete() // supplied 2026-04-10
	got := e.Retention(MustParseDate("2029-04-10"))
	if got.Until.String() != "2029-04-10" || !got.MustRetain || got.DaysLeft != 0 {
		t.Fatalf("Retention() = %+v, want last day of retention", got)
	}
}

func TestRetentionPeriodLabel(t *testing.T) {
	if got := RetentionPeriodLabel(); got != "3 years from the date of supply" {
		t.Errorf("RetentionPeriodLabel() = %q", got)
	}
}
