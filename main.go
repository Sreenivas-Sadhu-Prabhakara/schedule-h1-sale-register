// Command schedule-h1-sale-register keeps the separate register of Schedule H1
// drug supplies that a retail or hospital pharmacy must maintain under the
// Schedule H1 condition to the Drugs and Cosmetics Rules, 1945, read with
// Rule 65(11A).
//
// It serves a small web interface on the loopback address and stores the
// register in one JSON file on the same machine. It makes no outbound network
// calls of any kind: nothing is transmitted to any authority, and no part of
// the register leaves the computer it runs on.
package main

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sreenivas-sadhu-prabhakara/schedule-h1-sale-register/internal/h1register"
)

//go:embed web
var webFiles embed.FS

// fileVersion is the schema version written into the data file, so a future
// release can recognise and migrate an older file.
const fileVersion = 1

// maxBodyBytes caps a request body. One register line is a few hundred bytes.
const maxBodyBytes = 64 << 10

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to serve the register interface on")
	data := flag.String("data", "./schedule-h1-sale-register-data.json", "path to the register data file")
	flag.Parse()

	st, err := openStore(*data)
	if err != nil {
		log.Fatalf("cannot open register file %s: %v", *data, err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           routes(st),
		ReadHeaderTimeout: 10 * time.Second,
	}

	abs, err := filepath.Abs(*data)
	if err != nil {
		abs = *data
	}
	log.Printf("Schedule H1 sale register")
	log.Printf("register file: %s (%d entries)", abs, st.count())
	log.Printf("open http://%s/ in a browser — nothing is sent anywhere else", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// ---------------------------------------------------------------- data file

// pharmacy identifies the licensee on the printed register pages.
type pharmacy struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	LicenceNo  string `json:"licenceNo"`
	Pharmacist string `json:"pharmacist"`
}

// fileData is exactly what the data file holds.
type fileData struct {
	Version  int      `json:"version"`
	Pharmacy pharmacy `json:"pharmacy"`
	// Molecules is the pharmacy's own Schedule H1 molecule list. It is
	// absent until the list is first edited, at which point the whole
	// curated list is written out.
	Molecules []string           `json:"molecules,omitempty"`
	Entries   []h1register.Entry `json:"entries"`
}

// store holds the register and writes it to disk atomically. The register is
// personal health information, so the file is created readable only by its
// owner.
type store struct {
	mu   sync.RWMutex
	path string
	data fileData
}

func openStore(path string) (*store, error) {
	s := &store{path: path, data: fileData{Version: fileVersion, Entries: []h1register.Entry{}}}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return s, nil
	}
	var d fileData
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("the register file is not readable as JSON: %w", err)
	}
	if d.Entries == nil {
		d.Entries = []h1register.Entry{}
	}
	if d.Version == 0 {
		d.Version = fileVersion
	}
	s.data = d
	return s, nil
}

// save writes the register to a temporary file in the same directory, flushes
// it and renames it over the old file, so an interrupted write can never leave
// a half-written register behind.
func (s *store) save() error {
	s.data.Version = fileVersion
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}

func (s *store) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data.Entries)
}

// snapshot returns a copy of the register and the molecule list in force.
func (s *store) snapshot() ([]h1register.Entry, pharmacy, h1register.MoleculeList) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]h1register.Entry, len(s.data.Entries))
	copy(entries, s.data.Entries)
	return entries, s.data.Pharmacy, s.molecules()
}

// molecules returns the list in force: the pharmacy's own curated list once it
// has edited one, the seeded list until then. Callers hold the lock.
func (s *store) molecules() h1register.MoleculeList {
	if s.data.Molecules == nil {
		return h1register.NewMoleculeList()
	}
	return h1register.NewMoleculeListFrom(s.data.Molecules)
}

// addEntry validates the entry, gives it the next serial number and appends it.
// A submitted serial must be the one the register expects, which stops a
// double-submitted form from writing the same line twice.
func (s *store) addEntry(in h1register.Entry) (h1register.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := in.Trim()
	next := h1register.NextSerial(s.data.Entries)
	switch {
	case e.Serial == 0:
		e.Serial = next
	case e.Serial != next:
		return h1register.Entry{}, fmt.Errorf(
			"this entry must be serial %d, not %d — serial numbers run unbroken and are never reused", next, e.Serial)
	}
	if err := h1register.Validate(e); err != nil {
		return h1register.Entry{}, err
	}

	s.data.Entries = append(s.data.Entries, e)
	if err := s.save(); err != nil {
		s.data.Entries = s.data.Entries[:len(s.data.Entries)-1]
		return h1register.Entry{}, err
	}
	return e, nil
}

// correctEntry rewrites the columns of an existing line, keeping its serial
// number. In a bound register a correction is struck through and initialled
// rather than erased, so record what changed in the notes column.
func (s *store) correctEntry(serial int, in h1register.Entry) (h1register.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, e := range s.data.Entries {
		if e.Serial == serial {
			idx = i
			break
		}
	}
	if idx < 0 {
		return h1register.Entry{}, fmt.Errorf("there is no entry with serial %d in this register", serial)
	}
	e := in.Trim()
	e.Serial = serial
	if err := h1register.Validate(e); err != nil {
		return h1register.Entry{}, err
	}

	prev := s.data.Entries[idx]
	s.data.Entries[idx] = e
	if err := s.save(); err != nil {
		s.data.Entries[idx] = prev
		return h1register.Entry{}, err
	}
	return e, nil
}

// deleteLastEntry removes the highest-numbered line, which is the only removal
// that cannot leave a gap behind. Anything earlier stays: a missing serial in
// the middle of a register is an inspection failure, and this tool will not
// create one.
func (s *store) deleteLastEntry() (h1register.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.data.Entries) == 0 {
		return h1register.Entry{}, errors.New("the register is empty")
	}
	idx := 0
	for i, e := range s.data.Entries {
		if e.Serial > s.data.Entries[idx].Serial {
			idx = i
		}
	}
	removed := s.data.Entries[idx]
	kept := make([]h1register.Entry, 0, len(s.data.Entries)-1)
	kept = append(kept, s.data.Entries[:idx]...)
	kept = append(kept, s.data.Entries[idx+1:]...)

	prev := s.data.Entries
	s.data.Entries = kept
	if err := s.save(); err != nil {
		s.data.Entries = prev
		return h1register.Entry{}, err
	}
	return removed, nil
}

func (s *store) setPharmacy(p pharmacy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.Pharmacy
	s.data.Pharmacy = pharmacy{
		Name:       strings.TrimSpace(p.Name),
		Address:    strings.TrimSpace(p.Address),
		LicenceNo:  strings.TrimSpace(p.LicenceNo),
		Pharmacist: strings.TrimSpace(p.Pharmacist),
	}
	if err := s.save(); err != nil {
		s.data.Pharmacy = prev
		return err
	}
	return nil
}

// updateMolecules adds or removes a molecule name, or restores the seeded
// list.
func (s *store) updateMolecules(add, remove string, reset bool) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.data.Molecules
	switch {
	case reset:
		s.data.Molecules = nil
	case strings.TrimSpace(add) != "":
		s.data.Molecules = s.molecules().Add(add).Names()
	case strings.TrimSpace(remove) != "":
		s.data.Molecules = s.molecules().Remove(remove).Names()
	default:
		return s.molecules().Names(), nil
	}
	if err := s.save(); err != nil {
		s.data.Molecules = prev
		return nil, err
	}
	return s.molecules().Names(), nil
}

// ---------------------------------------------------------------- HTTP

func routes(st *store) http.Handler {
	mux := http.NewServeMux()

	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatalf("embedded interface files are missing: %v", err)
	}
	mux.Handle("GET /", http.FileServerFS(sub))

	mux.HandleFunc("GET /api/state", st.handleState)
	mux.HandleFunc("POST /api/entries", st.handleAddEntry)
	mux.HandleFunc("PUT /api/entries/{serial}", st.handleCorrectEntry)
	mux.HandleFunc("DELETE /api/entries/last", st.handleDeleteLast)
	mux.HandleFunc("GET /api/h1check", st.handleH1Check)
	mux.HandleFunc("POST /api/molecules", st.handleMolecules)
	mux.HandleFunc("POST /api/pharmacy", st.handlePharmacy)
	mux.HandleFunc("GET /api/export.csv", st.handleExportCSV)

	return mux
}

// entryView is one register line with everything the interface shows about it.
type entryView struct {
	h1register.Entry
	Retention h1register.Retention `json:"retention"`
	H1Match   []string             `json:"h1Match"`
}

// pageMeta is one page of the printable register, as a list of serials.
type pageMeta struct {
	Number  int   `json:"number"`
	Of      int   `json:"of"`
	Serials []int `json:"serials"`
}

type stateResponse struct {
	AsOf            h1register.Date       `json:"asOf"`
	Pharmacy        pharmacy              `json:"pharmacy"`
	Fields          []h1register.Field    `json:"fields"`
	NextSerial      int                   `json:"nextSerial"`
	Continuity      h1register.Continuity `json:"continuity"`
	RetentionYears  int                   `json:"retentionYears"`
	RetentionRule   string                `json:"retentionRule"`
	Molecules       []string              `json:"molecules"`
	MoleculesEdited bool                  `json:"moleculesEdited"`
	Total           int                   `json:"total"`
	Shown           int                   `json:"shown"`
	RowsPerPage     int                   `json:"rowsPerPage"`
	Filtered        bool                  `json:"filtered"`
	Pages           []pageMeta            `json:"pages"`
	Entries         []entryView           `json:"entries"`
	MustRetain      int                   `json:"mustRetain"`
	MayDestroy      int                   `json:"mayDestroy"`
}

func (s *store) handleState(w http.ResponseWriter, r *http.Request) {
	q, asOf, rows, err := readQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	all, ph, mol := s.snapshot()
	shown := h1register.Filter(all, q)

	resp := stateResponse{
		AsOf:           asOf,
		Pharmacy:       ph,
		Fields:         h1register.Fields,
		NextSerial:     h1register.NextSerial(all),
		Continuity:     h1register.CheckContinuity(all),
		RetentionYears: h1register.RetentionYears,
		RetentionRule:  h1register.RetentionPeriodLabel(),
		Molecules:      mol.Names(),
		Total:          len(all),
		Shown:          len(shown),
		RowsPerPage:    rows,
		Filtered:       !q.IsEmpty(),
		Entries:        make([]entryView, 0, len(shown)),
	}

	s.mu.RLock()
	resp.MoleculesEdited = s.data.Molecules != nil
	s.mu.RUnlock()

	for _, e := range shown {
		ret := e.Retention(asOf)
		if ret.MayDestroy {
			resp.MayDestroy++
		} else {
			resp.MustRetain++
		}
		resp.Entries = append(resp.Entries, entryView{Entry: e, Retention: ret, H1Match: mol.Match(e.DrugName)})
	}
	for _, p := range h1register.Paginate(shown, rows) {
		pm := pageMeta{Number: p.Number, Of: p.Of, Serials: make([]int, 0, len(p.Entries))}
		for _, e := range p.Entries {
			pm.Serials = append(pm.Serials, e.Serial)
		}
		resp.Pages = append(resp.Pages, pm)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *store) handleAddEntry(w http.ResponseWriter, r *http.Request) {
	var in h1register.Entry
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	e, err := s.addEntry(in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	all, _, mol := s.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"entry":      entryView{Entry: e, Retention: e.Retention(h1register.Today()), H1Match: mol.Match(e.DrugName)},
		"nextSerial": h1register.NextSerial(all),
		"continuity": h1register.CheckContinuity(all),
	})
}

func (s *store) handleCorrectEntry(w http.ResponseWriter, r *http.Request) {
	serial, err := strconv.Atoi(r.PathValue("serial"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "the serial number in the address is not a number", nil)
		return
	}
	var in h1register.Entry
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	e, err := s.correctEntry(serial, in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	_, _, mol := s.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"entry": entryView{Entry: e, Retention: e.Retention(h1register.Today()), H1Match: mol.Match(e.DrugName)},
	})
}

func (s *store) handleDeleteLast(w http.ResponseWriter, _ *http.Request) {
	e, err := s.deleteLastEntry()
	if err != nil {
		writeError(w, http.StatusConflict, err.Error(), nil)
		return
	}
	all, _, _ := s.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"removed":    e.Serial,
		"nextSerial": h1register.NextSerial(all),
		"continuity": h1register.CheckContinuity(all),
	})
}

func (s *store) handleH1Check(w http.ResponseWriter, r *http.Request) {
	drug := r.URL.Query().Get("drug")
	_, _, mol := s.snapshot()
	matches := mol.Match(drug)
	writeJSON(w, http.StatusOK, map[string]any{
		"drug":    drug,
		"isH1":    len(matches) > 0,
		"matches": matches,
	})
}

func (s *store) handleMolecules(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Add    string `json:"add"`
		Remove string `json:"remove"`
		Reset  bool   `json:"reset"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	names, err := s.updateMolecules(in.Add, in.Remove, in.Reset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	s.mu.RLock()
	edited := s.data.Molecules != nil
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"molecules": names, "moleculesEdited": edited})
}

func (s *store) handlePharmacy(w http.ResponseWriter, r *http.Request) {
	var in pharmacy
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := s.setPharmacy(in); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	_, ph, _ := s.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{"pharmacy": ph})
}

func (s *store) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	q, asOf, _, err := readQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	all, _, _ := s.snapshot()
	rows := h1register.Filter(all, q)

	name := "schedule-h1-register"
	if !q.From.IsZero() || !q.To.IsZero() {
		name += "-" + rangeLabel(q.From, q.To)
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.csv"`)

	cw := csv.NewWriter(w)
	header := append(h1register.Headers(), "Retain until", "Retention status")
	if err := cw.Write(header); err != nil {
		return
	}
	for _, e := range rows {
		ret := e.Retention(asOf)
		if err := cw.Write(append(e.Row(), ret.Until.String(), ret.Status)); err != nil {
			return
		}
	}
	cw.Flush()
}

// ---------------------------------------------------------------- helpers

// readQuery reads the search, date-range, as-of and page-size parameters.
func readQuery(r *http.Request) (h1register.Query, h1register.Date, int, error) {
	v := r.URL.Query()
	q := h1register.Query{
		Drug:       v.Get("drug"),
		Batch:      v.Get("batch"),
		Patient:    v.Get("patient"),
		Prescriber: v.Get("prescriber"),
	}
	var err error
	if q.From, err = h1register.ParseDate(v.Get("from")); err != nil {
		return q, h1register.Date{}, 0, err
	}
	if q.To, err = h1register.ParseDate(v.Get("to")); err != nil {
		return q, h1register.Date{}, 0, err
	}
	if !q.From.IsZero() && !q.To.IsZero() && q.To.Before(q.From) {
		return q, h1register.Date{}, 0, errors.New("the end of the date range falls before its start")
	}

	asOf := h1register.Today()
	if s := v.Get("asOf"); strings.TrimSpace(s) != "" {
		if asOf, err = h1register.ParseDate(s); err != nil {
			return q, h1register.Date{}, 0, err
		}
	}

	rows := h1register.DefaultRowsPerPage
	if s := v.Get("rows"); strings.TrimSpace(s) != "" {
		n, convErr := strconv.Atoi(s)
		if convErr != nil || n < 1 || n > 200 {
			return q, asOf, 0, errors.New("rows per page must be a whole number between 1 and 200")
		}
		rows = n
	}
	return q, asOf, rows, nil
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("the request had no body")
		}
		return fmt.Errorf("the request body could not be read: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("could not write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string, extra map[string]any) {
	body := map[string]any{"error": msg}
	for k, v := range extra {
		body[k] = v
	}
	writeJSON(w, code, body)
}

// writeDomainError turns an incomplete entry into a response the form can use
// to mark the exact inputs that are blank.
func writeDomainError(w http.ResponseWriter, err error) {
	var inc *h1register.IncompleteEntryError
	if errors.As(err, &inc) {
		writeError(w, http.StatusUnprocessableEntity, err.Error(), map[string]any{
			"missing":     inc.Missing,
			"missingKeys": inc.MissingKeys,
		})
		return
	}
	writeError(w, http.StatusConflict, err.Error(), nil)
}

// rangeLabel names a date range for a download filename.
func rangeLabel(from, to h1register.Date) string {
	switch {
	case from.IsZero():
		return "upto-" + to.String()
	case to.IsZero():
		return "from-" + from.String()
	default:
		return from.String() + "-to-" + to.String()
	}
}
