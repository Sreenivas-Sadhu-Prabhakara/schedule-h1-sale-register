package h1register

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DateLayout is the single date format used in the register, on screen, in the
// stored file and in CSV exports.
const DateLayout = "2006-01-02"

// Date is a calendar date with no clock and no time zone. The register records
// the day of supply, so a plain calendar date is the whole truth: it cannot
// drift across midnight or across zones, and retention arithmetic on it is
// exact.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// NewDate builds a date without validating it. Use ParseDate for untrusted
// input.
func NewDate(y int, m time.Month, d int) Date { return Date{Year: y, Month: m, Day: d} }

// Today returns the current local calendar date.
func Today() Date {
	t := time.Now()
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// ParseDate reads a YYYY-MM-DD date and rejects anything that is not a real
// calendar day, including 31 April and 29 February in a non-leap year. An
// empty string parses to the zero Date with no error, because a blank date of
// supply is a validation problem, not a parse problem.
func ParseDate(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, nil
	}
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("date %q is not a valid YYYY-MM-DD calendar date", s)
	}
	d := Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
	if d.String() != s {
		return Date{}, fmt.Errorf("date %q is not a valid YYYY-MM-DD calendar date", s)
	}
	return d, nil
}

// MustParseDate is ParseDate for literals that are known to be valid.
func MustParseDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

// IsZero reports whether the date is unset.
func (d Date) IsZero() bool { return d.Year == 0 && d.Month == 0 && d.Day == 0 }

// String renders the date as YYYY-MM-DD, or "" when unset.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// Time converts the date to midnight UTC, which is the anchor used for day
// counting.
func (d Date) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// Compare orders two dates: negative if d is earlier, 0 if equal, positive if
// d is later.
func (d Date) Compare(o Date) int {
	switch {
	case d.Year != o.Year:
		return d.Year - o.Year
	case d.Month != o.Month:
		return int(d.Month) - int(o.Month)
	default:
		return d.Day - o.Day
	}
}

// Before reports whether d falls earlier than o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// After reports whether d falls later than o.
func (d Date) After(o Date) bool { return d.Compare(o) > 0 }

// Equal reports whether both dates are the same day.
func (d Date) Equal(o Date) bool { return d.Compare(o) == 0 }

// DaysUntil counts whole days from d to o. It is negative when o is earlier.
func (d Date) DaysUntil(o Date) int {
	return int(o.Time().Sub(d.Time()).Hours() / 24)
}

// AddDays moves the date by n days, rolling over months and years.
func (d Date) AddDays(n int) Date {
	t := d.Time().AddDate(0, 0, n)
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// AddYears moves the date forward by n calendar years and clamps the day to
// the last day of the target month when the original day does not exist there.
// 29 February plus three years is therefore 28 February, not 1 March: the
// anniversary of a date is a real day in the target month, and the standard
// library's own AddDate would instead spill over into the following month.
func (d Date) AddYears(n int) Date {
	if d.IsZero() {
		return Date{}
	}
	y := d.Year + n
	day := d.Day
	if last := DaysInMonth(y, d.Month); day > last {
		day = last
	}
	return Date{Year: y, Month: d.Month, Day: day}
}

// DaysInMonth returns the number of days in the given month of the given year.
func DaysInMonth(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// MarshalJSON writes the date as a YYYY-MM-DD string, or "" when unset.
func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

// UnmarshalJSON reads a YYYY-MM-DD string and rejects impossible calendar
// days.
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	p, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = p
	return nil
}
