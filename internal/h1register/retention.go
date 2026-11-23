package h1register

import "strconv"

// RetentionYears is the number of years a Schedule H1 sale record must be kept
// and be available for inspection, under the Schedule H1 condition to the
// Drugs and Cosmetics Rules, 1945 read with Rule 65(11A). It is declared once
// here; if the period is amended, change it in this one place.
const RetentionYears = 3

// Retention is the retention position of one register entry on a given day.
type Retention struct {
	// Until is the last day on which the record must still be held.
	Until Date `json:"until"`
	// MustRetain is true while the record is still within the retention
	// period.
	MustRetain bool `json:"mustRetain"`
	// MayDestroy is the complement of MustRetain.
	MayDestroy bool `json:"mayDestroy"`
	// DaysLeft counts the days from the day in question to Until. It is 0 on
	// the last day of retention and negative once the period has run out.
	DaysLeft int `json:"daysLeft"`
	// Status reads "Must retain" or "May destroy".
	Status string `json:"status"`
	// Detail spells the position out in words.
	Detail string `json:"detail"`
}

// RetentionUntil returns the last day on which a record of a supply made on
// dateOfSupply must still be held: the same day three calendar years later.
//
// The arithmetic is calendar arithmetic, not 1095 days, so leap years inside
// the period do not shift the date. Where the day does not exist in the target
// month — a supply on 29 February — the anniversary is the last day of that
// month, 28 February. Keeping the record one day longer costs nothing, so when
// the difference matters, hold it to 1 March.
func RetentionUntil(dateOfSupply Date) Date { return dateOfSupply.AddYears(RetentionYears) }

// RetentionStatus reports whether a record of a supply made on dateOfSupply
// must still be held as of the given day, and for how much longer.
func RetentionStatus(dateOfSupply, asOf Date) Retention {
	r := Retention{Until: RetentionUntil(dateOfSupply)}
	if dateOfSupply.IsZero() || asOf.IsZero() {
		r.MustRetain = true
		r.Status = "Must retain"
		r.Detail = "Date of supply is blank, so the retention period cannot be worked out. Keep the record."
		return r
	}
	r.DaysLeft = asOf.DaysUntil(r.Until)
	if r.DaysLeft >= 0 {
		r.MustRetain = true
		r.Status = "Must retain"
		r.Detail = "Must be held until " + r.Until.String() + " — " +
			plural(r.DaysLeft, "day", "days") + " left."
		if r.DaysLeft == 0 {
			r.Detail = "Last day of the retention period is today, " + r.Until.String() + "."
		}
		return r
	}
	r.MayDestroy = true
	r.Status = "May destroy"
	r.Detail = "Retention period ended on " + r.Until.String() + ", " +
		plural(-r.DaysLeft, "day", "days") + " ago."
	return r
}

// Retention reports the entry's retention position as of the given day.
func (e Entry) Retention(asOf Date) Retention {
	return RetentionStatus(e.DateOfSupply, asOf)
}

// RetentionPeriodLabel describes the retention rule in words, for the UI and
// the printed register header.
func RetentionPeriodLabel() string {
	return strconv.Itoa(RetentionYears) + " years from the date of supply"
}
