package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/schedule-h1-sale-register/internal/h1register"
)

func testEntry() h1register.Entry {
	return h1register.Entry{
		DateOfSupply:    h1register.MustParseDate("2026-04-10"),
		DrugName:        "Levofloxacin 500 mg tablet",
		Manufacturer:    "Shree Pharma Ltd",
		BatchNo:         "LVX-2261",
		Quantity:        "10 tablets",
		PrescriberName:  "Dr A Nair",
		PrescriberRegNo: "KMC/41287",
		PatientName:     "R Kulkarni",
		PatientAddress:  "14, 3rd Cross, Jayanagar, Bengaluru 560011",
		PharmacistName:  "S Rao",
	}
}

func newTestStore(t *testing.T) (*store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "register.json")
	st, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	return st, path
}

func TestStoreAssignsSerialsAndPersistsAtomically(t *testing.T) {
	st, path := newTestStore(t)

	for i := 1; i <= 3; i++ {
		e, err := st.addEntry(testEntry())
		if err != nil {
			t.Fatalf("addEntry %d: %v", i, err)
		}
		if e.Serial != i {
			t.Fatalf("entry %d got serial %d", i, e.Serial)
		}
	}

	// The file on disk is the register, not a cache of it.
	reopened, err := openStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.count() != 3 {
		t.Fatalf("reopened register holds %d entries, want 3", reopened.count())
	}
	if got := h1register.NextSerial(reopened.data.Entries); got != 4 {
		t.Errorf("NextSerial after reopen = %d, want 4", got)
	}
	if c := h1register.CheckContinuity(reopened.data.Entries); !c.OK {
		t.Errorf("reopened register fails continuity: %s", c.Summary)
	}

	// Patient names and addresses live in this file, so it must not be
	// world-readable, and no temporary file may be left behind.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("register file mode is %o, want 600", perm)
	}
	leftovers, _ := filepath.Glob(path + ".tmp-*")
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestStoreRejectsIncompleteAndMisnumberedEntries(t *testing.T) {
	st, _ := newTestStore(t)

	bad := testEntry()
	bad.BatchNo = ""
	if _, err := st.addEntry(bad); err == nil {
		t.Fatal("addEntry accepted an entry with no batch number")
	} else if !strings.Contains(err.Error(), "batch number") {
		t.Errorf("error %q does not name the missing column", err)
	}
	if st.count() != 0 {
		t.Fatal("a refused entry was still written to the register")
	}

	if _, err := st.addEntry(testEntry()); err != nil {
		t.Fatalf("addEntry: %v", err)
	}
	stale := testEntry()
	stale.Serial = 1 // a re-submitted form still carrying the old serial
	if _, err := st.addEntry(stale); err == nil {
		t.Fatal("addEntry accepted a reused serial number")
	}
	if st.count() != 1 {
		t.Errorf("register holds %d entries, want 1", st.count())
	}
}

func TestStoreDeleteLastKeepsSerialsUnbroken(t *testing.T) {
	st, _ := newTestStore(t)
	for i := 0; i < 3; i++ {
		if _, err := st.addEntry(testEntry()); err != nil {
			t.Fatalf("addEntry: %v", err)
		}
	}
	removed, err := st.deleteLastEntry()
	if err != nil {
		t.Fatalf("deleteLastEntry: %v", err)
	}
	if removed.Serial != 3 {
		t.Errorf("removed serial %d, want 3", removed.Serial)
	}
	if c := h1register.CheckContinuity(st.data.Entries); !c.OK {
		t.Errorf("continuity broken after removing the last line: %s", c.Summary)
	}
	if got := h1register.NextSerial(st.data.Entries); got != 3 {
		t.Errorf("NextSerial = %d, want 3 so the serial is written once", got)
	}

	for i := 0; i < 2; i++ {
		if _, err := st.deleteLastEntry(); err != nil {
			t.Fatalf("deleteLastEntry: %v", err)
		}
	}
	if _, err := st.deleteLastEntry(); err == nil {
		t.Error("deleting from an empty register should fail")
	}
}

func TestStoreCorrectEntryKeepsTheSerial(t *testing.T) {
	st, _ := newTestStore(t)
	if _, err := st.addEntry(testEntry()); err != nil {
		t.Fatalf("addEntry: %v", err)
	}

	fix := testEntry()
	fix.Serial = 99 // ignored: a correction never renumbers a line
	fix.BatchNo = "LVX-2262"
	fix.Notes = "Batch number corrected against the pack"
	got, err := st.correctEntry(1, fix)
	if err != nil {
		t.Fatalf("correctEntry: %v", err)
	}
	if got.Serial != 1 || got.BatchNo != "LVX-2262" {
		t.Errorf("corrected entry = serial %d batch %q", got.Serial, got.BatchNo)
	}
	if _, err := st.correctEntry(42, testEntry()); err == nil {
		t.Error("correcting a line that does not exist should fail")
	}

	blank := testEntry()
	blank.PatientAddress = ""
	if _, err := st.correctEntry(1, blank); err == nil {
		t.Error("a correction that blanks a statutory column should be refused")
	}
}

func TestStoreMoleculeListIsUserExtendable(t *testing.T) {
	st, path := newTestStore(t)

	_, _, mol := st.snapshot()
	seeded := mol.Len()
	if mol.IsScheduleH1("Bedaquiline 100 mg") {
		t.Fatal("bedaquiline is seeded; pick another molecule for this test")
	}

	if _, err := st.updateMolecules("Bedaquiline", "", false); err != nil {
		t.Fatalf("updateMolecules: %v", err)
	}
	reopened, err := openStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	_, _, mol = reopened.snapshot()
	if !mol.IsScheduleH1("Tab Bedaquiline 100 mg") {
		t.Error("an added molecule did not survive a reload")
	}
	if mol.Len() != seeded+1 {
		t.Errorf("molecule list holds %d names, want %d", mol.Len(), seeded+1)
	}

	if _, err := st.updateMolecules("", "", true); err != nil {
		t.Fatalf("reset: %v", err)
	}
	_, _, mol = st.snapshot()
	if mol.Len() != seeded {
		t.Errorf("after reset the list holds %d names, want the seeded %d", mol.Len(), seeded)
	}
}

func TestAPIAddEntryAndState(t *testing.T) {
	st, _ := newTestStore(t)
	h := routes(st)

	body, err := json.Marshal(testEntry())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/entries", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/entries = %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/state?asOf=2026-04-10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/state = %d: %s", rec.Code, rec.Body.String())
	}
	var state stateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("state is not readable JSON: %v", err)
	}
	if state.Total != 1 || state.NextSerial != 2 {
		t.Errorf("Total,NextSerial = %d,%d want 1,2", state.Total, state.NextSerial)
	}
	if !state.Continuity.OK {
		t.Errorf("continuity not OK: %s", state.Continuity.Summary)
	}
	if state.RetentionYears != 3 {
		t.Errorf("RetentionYears = %d, want 3", state.RetentionYears)
	}
	if len(state.Entries) != 1 {
		t.Fatalf("state holds %d entries, want 1", len(state.Entries))
	}
	got := state.Entries[0]
	if got.Retention.Until.String() != "2029-04-10" || !got.Retention.MustRetain {
		t.Errorf("retention = %+v, want must retain until 2029-04-10", got.Retention)
	}
	if len(got.H1Match) != 1 || got.H1Match[0] != "levofloxacin" {
		t.Errorf("H1Match = %v, want [levofloxacin]", got.H1Match)
	}
	if len(state.Pages) != 1 || state.Pages[0].Of != 1 {
		t.Errorf("pages = %+v, want one page of one", state.Pages)
	}
}

func TestAPIRejectsIncompleteEntryWithTheMissingFields(t *testing.T) {
	st, _ := newTestStore(t)
	h := routes(st)

	bad := testEntry()
	bad.DrugName = ""
	bad.PatientName = ""
	body, err := json.Marshal(bad)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/entries", strings.NewReader(string(body))))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /api/entries = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Error       string   `json:"error"`
		Missing     []string `json:"missing"`
		MissingKeys []string `json:"missingKeys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not readable JSON: %v", err)
	}
	if len(out.Missing) != 2 || len(out.MissingKeys) != 2 {
		t.Fatalf("missing = %v / %v, want both columns named", out.Missing, out.MissingKeys)
	}
	if out.MissingKeys[0] != "drugName" || out.MissingKeys[1] != "patientName" {
		t.Errorf("missingKeys = %v, want [drugName patientName]", out.MissingKeys)
	}
}

func TestAPIH1Check(t *testing.T) {
	st, _ := newTestStore(t)
	h := routes(st)

	tests := []struct {
		drug     string
		wantIsH1 bool
	}{
		{"Rifampicin 450", true},
		{"Tab Alprazolam 0.25", true},
		{"Paracetamol 650", false},
		{"", false},
	}
	for _, tc := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/h1check?drug="+strings.ReplaceAll(tc.drug, " ", "+"), nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("h1check %q = %d", tc.drug, rec.Code)
		}
		var out struct {
			IsH1    bool     `json:"isH1"`
			Matches []string `json:"matches"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("h1check %q: %v", tc.drug, err)
		}
		if out.IsH1 != tc.wantIsH1 {
			t.Errorf("h1check(%q).isH1 = %v, want %v", tc.drug, out.IsH1, tc.wantIsH1)
		}
	}
}

func TestAPIExportCSVUsesTheStatutoryColumnOrder(t *testing.T) {
	st, _ := newTestStore(t)
	if _, err := st.addEntry(testEntry()); err != nil {
		t.Fatalf("addEntry: %v", err)
	}
	second := testEntry()
	second.DateOfSupply = h1register.MustParseDate("2026-05-02")
	second.DrugName = "Cefixime 200 mg DT"
	if _, err := st.addEntry(second); err != nil {
		t.Fatalf("addEntry: %v", err)
	}
	h := routes(st)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/export.csv?asOf=2026-05-02", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("export = %d", rec.Code)
	}
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("export has %d lines, want a header and two entries", len(lines))
	}
	if !strings.HasPrefix(lines[0], "Sl. No.,Date of supply,Name of drug,Manufacturer,Batch No.,Quantity,") {
		t.Errorf("header is not in statutory column order: %s", lines[0])
	}
	if !strings.HasSuffix(strings.TrimSpace(lines[0]), "Retain until,Retention status") {
		t.Errorf("header does not end with the retention columns: %s", lines[0])
	}
	if !strings.HasPrefix(lines[1], "1,2026-04-10,Levofloxacin 500 mg tablet,") {
		t.Errorf("first row = %s", lines[1])
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q", ct)
	}

	// A date range narrows the export and names itself in the filename, so
	// an inspector's copy says what period it covers.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/export.csv?from=2026-05-01&to=2026-05-31", nil))
	lines = strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("ranged export has %d lines, want a header and one entry", len(lines))
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "2026-05-01-to-2026-05-31") {
		t.Errorf("Content-Disposition = %q, want the date range in the filename", cd)
	}
}

func TestAPIRejectsABadDateRange(t *testing.T) {
	st, _ := newTestStore(t)
	h := routes(st)
	for _, url := range []string{
		"/api/state?from=2026-05-31&to=2026-05-01",
		"/api/state?from=not-a-date",
		"/api/state?rows=0",
		"/api/state?rows=999",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", url, rec.Code)
		}
	}
}

func TestEmbeddedInterfaceIsServed(t *testing.T) {
	st, _ := newTestStore(t)
	h := routes(st)
	for _, path := range []string{"/", "/assets/style.css", "/assets/app.js", "/assets/accent.css"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want the embedded file", path, rec.Code)
		}
	}
}

func TestOpenStoreOnAnUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(path, []byte("this is not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := openStore(path); err == nil {
		t.Fatal("openStore accepted a file that is not JSON; it would be overwritten on the next save")
	}

	// An empty file is treated as a new register rather than an error, so a
	// `touch`ed data file works.
	empty := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(empty, []byte("  \n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	st, err := openStore(empty)
	if err != nil {
		t.Fatalf("openStore on an empty file: %v", err)
	}
	if st.count() != 0 {
		t.Errorf("empty file gave %d entries", st.count())
	}
}
