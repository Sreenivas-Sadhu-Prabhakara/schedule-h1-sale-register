package h1register

import (
	"strconv"
	"strings"
	"testing"
)

// sample is a small register used by the search and pagination tests.
func sample() []Entry {
	rows := []struct {
		serial                                                  int
		date, drug, batch, patient, prescriber, prescriberRegNo string
	}{
		{1, "2026-04-01", "Rifampicin 450 mg capsule", "RIF-9001", "R Kulkarni", "Dr A Nair", "KMC/41287"},
		{2, "2026-04-03", "Levofloxacin 500 mg tablet", "LVX-2261", "S Fernandes", "Dr B Menon", "KMC/55120"},
		{3, "2026-04-18", "Alprazolam 0.25 mg tablet", "ALP-7733", "R Kulkarni", "Dr A Nair", "KMC/41287"},
		{4, "2026-05-02", "Cefixime 200 mg DT", "CFX-1180", "T Varghese", "Dr C Pillai", "TNMC/88041"},
		{5, "2026-05-20", "Tramadol 50 mg capsule", "TRM-4412", "S Fernandes", "Dr B Menon", "KMC/55120"},
	}
	out := make([]Entry, 0, len(rows))
	for _, r := range rows {
		e := complete()
		e.Serial = r.serial
		e.DateOfSupply = MustParseDate(r.date)
		e.DrugName = r.drug
		e.BatchNo = r.batch
		e.PatientName = r.patient
		e.PrescriberName = r.prescriber
		e.PrescriberRegNo = r.prescriberRegNo
		out = append(out, e)
	}
	return out
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name  string
		query Query
		want  []int
	}{
		{"empty query returns the whole register", Query{}, []int{1, 2, 3, 4, 5}},
		{"by drug, exact molecule", Query{Drug: "alprazolam"}, []int{3}},
		{"by drug, case-insensitive", Query{Drug: "RIFAMPICIN"}, []int{1}},
		{"by drug, partial", Query{Drug: "floxacin"}, []int{2}},
		{"by drug, no match", Query{Drug: "paracetamol"}, nil},
		{"by batch", Query{Batch: "LVX-2261"}, []int{2}},
		{"by batch, partial and case-insensitive", Query{Batch: "trm"}, []int{5}},
		{"by patient", Query{Patient: "Kulkarni"}, []int{1, 3}},
		{"by prescriber name", Query{Prescriber: "Menon"}, []int{2, 5}},
		{"by prescriber registration number", Query{Prescriber: "TNMC/88041"}, []int{4}},
		{"from date, inclusive", Query{From: MustParseDate("2026-05-02")}, []int{4, 5}},
		{"to date, inclusive", Query{To: MustParseDate("2026-04-03")}, []int{1, 2}},
		{
			"date range covering one month",
			Query{From: MustParseDate("2026-04-01"), To: MustParseDate("2026-04-30")},
			[]int{1, 2, 3},
		},
		{
			"date range of a single day",
			Query{From: MustParseDate("2026-04-18"), To: MustParseDate("2026-04-18")},
			[]int{3},
		},
		{
			"date range matching nothing",
			Query{From: MustParseDate("2026-06-01"), To: MustParseDate("2026-06-30")},
			nil,
		},
		{
			"drug and date range together",
			Query{Drug: "tramadol", From: MustParseDate("2026-05-01"), To: MustParseDate("2026-05-31")},
			[]int{5},
		},
		{
			"patient and prescriber together",
			Query{Patient: "Fernandes", Prescriber: "Menon"},
			[]int{2, 5},
		},
		{
			"contradictory filters return nothing",
			Query{Patient: "Kulkarni", Prescriber: "Pillai"},
			nil,
		},
		{"blank filters are ignored", Query{Drug: "  ", Patient: "\t"}, []int{1, 2, 3, 4, 5}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Filter(sample(), tc.query)
			if gotSerials := serialsOf(got); gotSerials != joinIntsPlain(tc.want) {
				t.Errorf("Filter() = [%s], want [%s]", gotSerials, joinIntsPlain(tc.want))
			}
		})
	}
}

func TestFilterReturnsSerialOrderWhateverTheStorageOrder(t *testing.T) {
	entries := sample()
	entries[0], entries[4] = entries[4], entries[0]
	entries[1], entries[3] = entries[3], entries[1]
	if got := serialsOf(Filter(entries, Query{})); got != "1,2,3,4,5" {
		t.Errorf("Filter() = [%s], want serial order", got)
	}
}

func TestQueryIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		query Query
		want  bool
	}{
		{"zero value", Query{}, true},
		{"blank strings", Query{Drug: " ", Batch: "  ", Patient: "", Prescriber: "\t"}, true},
		{"a drug filter", Query{Drug: "cefixime"}, false},
		{"a from date", Query{From: MustParseDate("2026-01-01")}, false},
		{"a to date", Query{To: MustParseDate("2026-01-01")}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.query.IsEmpty(); got != tc.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPaginate(t *testing.T) {
	tests := []struct {
		name        string
		count       int
		rowsPerPage int
		wantPages   int
		wantLast    int // rows on the last page
	}{
		{"empty register still yields one page", 0, 15, 1, 0},
		{"one entry", 1, 15, 1, 1},
		{"exactly one full page", 15, 15, 1, 15},
		{"one row over a page", 16, 15, 2, 1},
		{"two full pages", 30, 15, 2, 15},
		{"small pages", 5, 2, 3, 1},
		{"non-positive rows per page falls back to the default", 20, 0, 2, 5},
		{"negative rows per page falls back to the default", 20, -3, 2, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entries := make([]Entry, 0, tc.count)
			for i := 1; i <= tc.count; i++ {
				e := complete()
				e.Serial = i
				entries = append(entries, e)
			}
			pages := Paginate(entries, tc.rowsPerPage)
			if len(pages) != tc.wantPages {
				t.Fatalf("Paginate() gave %d pages, want %d", len(pages), tc.wantPages)
			}
			if got := len(pages[len(pages)-1].Entries); got != tc.wantLast {
				t.Errorf("last page holds %d rows, want %d", got, tc.wantLast)
			}
			total := 0
			for i, p := range pages {
				if p.Number != i+1 {
					t.Errorf("page %d is numbered %d", i+1, p.Number)
				}
				if p.Of != tc.wantPages {
					t.Errorf("page %d says 'of %d', want 'of %d'", p.Number, p.Of, tc.wantPages)
				}
				total += len(p.Entries)
			}
			if total != tc.count {
				t.Errorf("pages hold %d entries in all, want %d", total, tc.count)
			}
		})
	}
}

func TestPaginatePutsEntriesInSerialOrderWithoutMutatingTheInput(t *testing.T) {
	entries := sample()
	entries[0], entries[4] = entries[4], entries[0]
	first := entries[0].Serial

	pages := Paginate(entries, 2)
	var got []string
	for _, p := range pages {
		for _, e := range p.Entries {
			got = append(got, strconv.Itoa(e.Serial))
		}
	}
	if strings.Join(got, ",") != "1,2,3,4,5" {
		t.Errorf("paginated order = [%s], want serial order", strings.Join(got, ","))
	}
	if entries[0].Serial != first {
		t.Error("Paginate() reordered the caller's slice")
	}
}

func serialsOf(entries []Entry) string {
	s := make([]string, 0, len(entries))
	for _, e := range entries {
		s = append(s, strconv.Itoa(e.Serial))
	}
	return strings.Join(s, ",")
}

func joinIntsPlain(ns []int) string {
	s := make([]string, 0, len(ns))
	for _, n := range ns {
		s = append(s, strconv.Itoa(n))
	}
	return strings.Join(s, ",")
}
