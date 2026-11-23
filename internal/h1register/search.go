package h1register

import (
	"sort"
	"strings"
)

// Query selects entries out of the register. Every text field matches
// case-insensitively on any part of the value; a blank field matches
// everything. From and To bound the date of supply inclusively, and a zero
// date leaves that end open.
type Query struct {
	Drug       string `json:"drug"`
	Batch      string `json:"batch"`
	Patient    string `json:"patient"`
	Prescriber string `json:"prescriber"`
	From       Date   `json:"from"`
	To         Date   `json:"to"`
}

// IsEmpty reports whether the query selects the whole register.
func (q Query) IsEmpty() bool {
	return strings.TrimSpace(q.Drug) == "" &&
		strings.TrimSpace(q.Batch) == "" &&
		strings.TrimSpace(q.Patient) == "" &&
		strings.TrimSpace(q.Prescriber) == "" &&
		q.From.IsZero() && q.To.IsZero()
}

// Matches reports whether one entry satisfies the query.
func (q Query) Matches(e Entry) bool {
	if !contains(e.DrugName, q.Drug) {
		return false
	}
	if !contains(e.BatchNo, q.Batch) {
		return false
	}
	if !contains(e.PatientName, q.Patient) {
		return false
	}
	// The prescriber is searched by name or by registration number, because
	// an inspector following up a prescription has one or the other.
	if p := strings.TrimSpace(q.Prescriber); p != "" &&
		!contains(e.PrescriberName, p) && !contains(e.PrescriberRegNo, p) {
		return false
	}
	if !q.From.IsZero() && e.DateOfSupply.Before(q.From) {
		return false
	}
	if !q.To.IsZero() && e.DateOfSupply.After(q.To) {
		return false
	}
	return true
}

// Filter returns the entries matching the query, in serial order.
func Filter(entries []Entry, q Query) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if q.Matches(e) {
			out = append(out, e)
		}
	}
	SortBySerial(out)
	return out
}

// SortBySerial puts entries in register order.
func SortBySerial(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Serial < entries[j].Serial })
}

// DefaultRowsPerPage is how many lines the printable register puts on one
// page. It is user-overridable in the UI; a bound register page typically
// holds a dozen or so lines, and inspectors read a page at a time.
const DefaultRowsPerPage = 15

// Page is one page of the printable register.
type Page struct {
	Number  int     `json:"number"`
	Of      int     `json:"of"`
	Entries []Entry `json:"entries"`
}

// Paginate splits entries into pages of rowsPerPage lines, in serial order.
// A non-positive rowsPerPage falls back to DefaultRowsPerPage. An empty
// register yields one empty page, so the printed register always has a page to
// hand over.
func Paginate(entries []Entry, rowsPerPage int) []Page {
	if rowsPerPage <= 0 {
		rowsPerPage = DefaultRowsPerPage
	}
	ordered := make([]Entry, len(entries))
	copy(ordered, entries)
	SortBySerial(ordered)

	if len(ordered) == 0 {
		return []Page{{Number: 1, Of: 1, Entries: []Entry{}}}
	}
	total := (len(ordered) + rowsPerPage - 1) / rowsPerPage
	pages := make([]Page, 0, total)
	for i := 0; i < len(ordered); i += rowsPerPage {
		end := min(i+rowsPerPage, len(ordered))
		pages = append(pages, Page{
			Number:  len(pages) + 1,
			Of:      total,
			Entries: ordered[i:end],
		})
	}
	return pages
}

// contains reports whether needle appears in haystack, case-insensitively. A
// blank needle matches.
func contains(haystack, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
