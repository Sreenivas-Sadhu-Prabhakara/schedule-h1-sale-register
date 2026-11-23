package h1register

import (
	"sort"
	"strconv"
	"strings"
)

// FirstSerial is the serial number a fresh register starts at.
const FirstSerial = 1

// NextSerial returns the serial number the next entry must carry: one past the
// highest serial already used, or FirstSerial for an empty register. It never
// fills a gap, because reaching back into a gap would mean an entry made today
// sits in the register before entries made earlier.
func NextSerial(entries []Entry) int {
	next := FirstSerial
	for _, e := range entries {
		if e.Serial >= next {
			next = e.Serial + 1
		}
	}
	return next
}

// Continuity is the result of auditing the serial numbers of a register.
type Continuity struct {
	// OK is true only when the serials run unbroken, each used once, from
	// First to Last.
	OK bool `json:"ok"`
	// Count is the number of entries audited.
	Count int `json:"count"`
	// First and Last are the lowest and highest serials present, 0 for an
	// empty register.
	First int `json:"first"`
	Last  int `json:"last"`
	// Gaps lists the serials missing between First and Last.
	Gaps []int `json:"gaps"`
	// Duplicates lists the serials used by more than one entry.
	Duplicates []int `json:"duplicates"`
	// Invalid lists serials that are not positive numbers.
	Invalid []int `json:"invalid"`
	// StartsAtFirstSerial is false when the register's lowest serial is not
	// 1 — normal for a volume that continues an earlier one, worth saying
	// out loud for a volume that does not.
	StartsAtFirstSerial bool `json:"startsAtFirstSerial"`
	// Summary is a one-line reading of the audit, for the banner.
	Summary string `json:"summary"`
}

// CheckContinuity audits the serial numbers of a register for gaps, reuse and
// non-positive numbers.
//
// A register whose serials skip a number or use one twice cannot show an
// inspector that nothing was dispensed off the book, so this is reported
// loudly rather than quietly fixed: the result says exactly which serials are
// at fault. The audit looks at the set of serials, not at the order the entries
// happen to be stored in, so sorting or re-reading the file cannot change its
// verdict.
func CheckContinuity(entries []Entry) Continuity {
	c := Continuity{Count: len(entries), OK: true, StartsAtFirstSerial: true}
	if len(entries) == 0 {
		c.Summary = "Register is empty. The first entry will be serial 1."
		return c
	}

	seen := make(map[int]int, len(entries))
	for _, e := range entries {
		seen[e.Serial]++
	}

	for s, n := range seen {
		if s < FirstSerial {
			c.Invalid = append(c.Invalid, s)
			continue
		}
		if n > 1 {
			c.Duplicates = append(c.Duplicates, s)
		}
		if c.First == 0 || s < c.First {
			c.First = s
		}
		if s > c.Last {
			c.Last = s
		}
	}
	for s := c.First; s > 0 && s <= c.Last; s++ {
		if seen[s] == 0 {
			c.Gaps = append(c.Gaps, s)
		}
	}

	sort.Ints(c.Gaps)
	sort.Ints(c.Duplicates)
	sort.Ints(c.Invalid)
	c.StartsAtFirstSerial = c.First == FirstSerial
	c.OK = len(c.Gaps) == 0 && len(c.Duplicates) == 0 && len(c.Invalid) == 0
	c.Summary = continuitySummary(c)
	return c
}

// continuitySummary puts the audit into one sentence an inspector or a
// pharmacist can read at a glance.
func continuitySummary(c Continuity) string {
	if c.OK {
		return "Serials " + strconv.Itoa(c.First) + " to " + strconv.Itoa(c.Last) +
			" run unbroken across " + plural(c.Count, "entry", "entries") + "."
	}
	var parts []string
	if len(c.Gaps) > 0 {
		parts = append(parts, plural(len(c.Gaps), "serial", "serials")+" missing: "+joinInts(c.Gaps))
	}
	if len(c.Duplicates) > 0 {
		parts = append(parts, plural(len(c.Duplicates), "serial", "serials")+" used more than once: "+joinInts(c.Duplicates))
	}
	if len(c.Invalid) > 0 {
		parts = append(parts, "serials that are not positive numbers: "+joinInts(c.Invalid))
	}
	return "Serial numbering is broken — " + strings.Join(parts, "; ") +
		". A register with gaps or reused serials is an inspection failure; account for every one in writing."
}

// plural renders "1 entry" / "3 entries".
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// joinInts renders "4", "4 and 9", "4, 9 and 11".
func joinInts(ns []int) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = strconv.Itoa(n)
	}
	return joinList(s)
}
