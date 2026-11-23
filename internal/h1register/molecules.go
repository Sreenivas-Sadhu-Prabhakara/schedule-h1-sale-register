package h1register

import (
	"sort"
	"strings"
)

// ScheduleH1Seed is the ONE place in this program where Schedule H1 molecule
// names are written down. It is a starting point for data entry, not the
// Schedule.
//
// Schedule H1 to the Drugs and Cosmetics Rules, 1945 is amended by
// notification: molecules are added to it from time to time, so any list
// compiled on one day goes stale. Keep your own copy current against the
// Schedule as notified by the Ministry of Health and Family Welfare / CDSCO.
// The tool lets you add and remove names without rebuilding it, and the warning
// it raises is a prompt to check the pack and the Schedule, never a ruling.
//
// Seeded across the three groups the Schedule is built around: anti-TB drugs,
// certain antibiotics (fluoroquinolones, third-generation cephalosporins and
// carbapenems) and habit-forming molecules.
const ScheduleH1Seed = `
rifampicin
rifabutin
isoniazid
ethambutol
pyrazinamide
ethionamide
cycloserine
capreomycin
clofazimine
thiacetazone
levofloxacin
moxifloxacin
ofloxacin
norfloxacin
sparfloxacin
gatifloxacin
gemifloxacin
balofloxacin
prulifloxacin
cefixime
cefpodoxime
cefdinir
cefditoren
cefetamet
ceftibuten
ceftizoxime
ceftriaxone
cefotaxime
ceftazidime
cefoperazone
cefepime
cefpirome
meropenem
imipenem
ertapenem
doripenem
faropenem
alprazolam
chlordiazepoxide
diazepam
nitrazepam
midazolam
zolpidem
tramadol
codeine
buprenorphine
pentazocine
diphenoxylate
`

// SeedMolecules returns the seeded molecule names, lower-cased, sorted and
// de-duplicated.
func SeedMolecules() []string { return normaliseMolecules(strings.Fields(ScheduleH1Seed)) }

// MoleculeList is a Schedule H1 molecule list: the seed plus whatever the
// pharmacy has added, minus whatever it has removed. It holds no global state,
// so two lists never interfere and every match is reproducible from the list
// that made it.
type MoleculeList struct {
	names []string
}

// NewMoleculeList returns the seeded list.
func NewMoleculeList() MoleculeList { return MoleculeList{names: SeedMolecules()} }

// NewMoleculeListFrom returns a list holding exactly the given names, used when
// loading a list the pharmacy has already curated.
func NewMoleculeListFrom(names []string) MoleculeList {
	return MoleculeList{names: normaliseMolecules(names)}
}

// Names returns the molecule names in the list, sorted.
func (l MoleculeList) Names() []string {
	out := make([]string, len(l.names))
	copy(out, l.names)
	return out
}

// Len reports how many molecules the list holds.
func (l MoleculeList) Len() int { return len(l.names) }

// Add returns a copy of the list with the given name included. A blank or
// already-present name leaves the list unchanged.
func (l MoleculeList) Add(name string) MoleculeList {
	return NewMoleculeListFrom(append(l.Names(), name))
}

// Remove returns a copy of the list without the given name, so a pharmacy can
// drop a name this tool seeded but the Schedule does not carry.
func (l MoleculeList) Remove(name string) MoleculeList {
	name = strings.ToLower(strings.TrimSpace(name))
	kept := make([]string, 0, len(l.names))
	for _, n := range l.names {
		if n != name {
			kept = append(kept, n)
		}
	}
	return MoleculeList{names: kept}
}

// Has reports whether the exact molecule name is in the list.
func (l MoleculeList) Has(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, n := range l.names {
		if n == name {
			return true
		}
	}
	return false
}

// Match returns every molecule in the list that appears inside the given drug
// name, matched case-insensitively. Pharmacy stock is labelled by brand and by
// compound — "Rifampicin 450 (Rcinex)", "Levofloxacin 500 Tab" — so the match
// looks for the molecule anywhere in the string the pharmacist typed and
// returns what it found rather than a bare yes.
//
// Where one matched name sits inside a longer matched name, only the longer one
// is reported: "levofloxacin 500" is a levofloxacin supply, not also an
// ofloxacin one.
func (l MoleculeList) Match(drugName string) []string {
	hay := strings.ToLower(drugName)
	if strings.TrimSpace(hay) == "" {
		return nil
	}
	var found []string
	for _, n := range l.names {
		if strings.Contains(hay, n) {
			found = append(found, n)
		}
	}
	hits := make([]string, 0, len(found))
	for _, n := range found {
		if !subsumed(n, found) {
			hits = append(hits, n)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	return hits
}

// IsScheduleH1 reports whether the drug name looks like a Schedule H1 molecule
// according to this list.
func (l MoleculeList) IsScheduleH1(drugName string) bool { return len(l.Match(drugName)) > 0 }

// IsScheduleH1 reports whether the drug name looks like a Schedule H1 molecule
// according to the seeded list alone. Use the MoleculeList method where the
// pharmacy has curated its own list.
func IsScheduleH1(drugName string) bool { return NewMoleculeList().IsScheduleH1(drugName) }

// subsumed reports whether name is contained in some other, longer name in the
// set.
func subsumed(name string, all []string) bool {
	for _, other := range all {
		if len(other) > len(name) && strings.Contains(other, name) {
			return true
		}
	}
	return false
}

// normaliseMolecules lower-cases, trims, drops blanks, de-duplicates and sorts.
func normaliseMolecules(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		n := strings.ToLower(strings.TrimSpace(raw))
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
