package h1register

import (
	"sort"
	"strings"
	"testing"
)

func TestIsScheduleH1(t *testing.T) {
	tests := []struct {
		name string
		drug string
		want bool
	}{
		{"anti-TB, plain molecule", "Rifampicin", true},
		{"anti-TB, lower case", "isoniazid", true},
		{"anti-TB, upper case", "ETHAMBUTOL", true},
		{"anti-TB, mixed case in a pack name", "Pyrazinamide 750 mg Tab", true},
		{"fluoroquinolone inside a compound string", "Tab Levofloxacin 500 mg (10s)", true},
		{"fluoroquinolone with brand in brackets", "Moxifloxacin 400mg [Moxiford]", true},
		{"third-generation cephalosporin", "Cefixime 200 DT", true},
		{"cephalosporin in a combination", "Cefpodoxime 200 + Clavulanic acid 125", true},
		{"habit-forming molecule", "Alprazolam 0.25 mg", true},
		{"habit-forming molecule, dose first", "0.5 mg tramadol HCl SR", true},
		{"habit-forming sleep aid", "ZOLPIDEM TARTRATE 10MG", true},
		{"carbapenem", "Inj Meropenem 1 g vial", true},
		{"molecule with surrounding punctuation", "(cefpodoxime)", true},
		{"ordinary analgesic is not H1", "Paracetamol 650 mg", false},
		{"ordinary antibiotic is not seeded", "Amoxicillin 500 mg capsule", false},
		{"antacid is not H1", "Pantoprazole 40 mg", false},
		{"empty string", "", false},
		{"whitespace only", "   ", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsScheduleH1(tc.drug); got != tc.want {
				t.Errorf("IsScheduleH1(%q) = %v, want %v", tc.drug, got, tc.want)
			}
		})
	}
}

func TestMatchReportsWhatItFound(t *testing.T) {
	tests := []struct {
		name string
		drug string
		want []string
	}{
		{"single molecule", "Rifampicin 450", []string{"rifampicin"}},
		{
			name: "fixed-dose anti-TB combination lists each molecule",
			drug: "Rifampicin 150 + Isoniazid 75 + Pyrazinamide 400 + Ethambutol 275",
			want: []string{"ethambutol", "isoniazid", "pyrazinamide", "rifampicin"},
		},
		{
			name: "the longer molecule wins over the one inside it",
			drug: "Levofloxacin 500 mg",
			want: []string{"levofloxacin"},
		},
		{
			name: "a standalone short molecule is still reported",
			drug: "Ofloxacin 200 + Ornidazole 500",
			want: []string{"ofloxacin"},
		},
		{"nothing matches", "Paracetamol 650", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewMoleculeList().Match(tc.drug)
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("Match(%q) = %v, want %v", tc.drug, got, tc.want)
			}
		})
	}
}

func TestSeedListIsSaneAndSingleSourced(t *testing.T) {
	seed := SeedMolecules()
	if len(seed) < 30 {
		t.Fatalf("seed list holds %d molecules, expected a usable starting set", len(seed))
	}
	for _, n := range seed {
		if n != strings.ToLower(n) || strings.TrimSpace(n) != n {
			t.Errorf("seed entry %q is not trimmed and lower-cased", n)
		}
		if len(n) < 6 {
			t.Errorf("seed entry %q is short enough to match unrelated words by accident", n)
		}
	}
	if !sort.StringsAreSorted(seed) {
		t.Error("seed list is not sorted")
	}
	// No duplicates, and the three groups the Schedule is built around are
	// all represented.
	for i := 1; i < len(seed); i++ {
		if seed[i] == seed[i-1] {
			t.Errorf("seed list repeats %q", seed[i])
		}
	}
	for _, want := range []string{"rifampicin", "isoniazid", "levofloxacin", "cefixime", "alprazolam", "tramadol", "zolpidem"} {
		if !NewMoleculeList().Has(want) {
			t.Errorf("seed list is missing %q", want)
		}
	}
}

func TestMoleculeListIsUserExtendable(t *testing.T) {
	base := NewMoleculeList()
	if base.IsScheduleH1("Bedaquiline 100 mg") {
		t.Fatal("bedaquiline is not seeded; the test below would prove nothing")
	}

	extended := base.Add("Bedaquiline")
	if !extended.IsScheduleH1("Tab Bedaquiline 100 mg") {
		t.Error("an added molecule is not matched")
	}
	if !extended.Has("bedaquiline") {
		t.Error("Add() should store the name lower-cased")
	}
	if base.IsScheduleH1("Bedaquiline 100 mg") {
		t.Error("Add() must not mutate the list it was called on")
	}
	if extended.Len() != base.Len()+1 {
		t.Errorf("Len() = %d, want %d", extended.Len(), base.Len()+1)
	}

	tests := []struct {
		name    string
		add     string
		wantLen int
	}{
		{"blank name is ignored", "", base.Len()},
		{"whitespace name is ignored", "   ", base.Len()},
		{"already present name is ignored", "rifampicin", base.Len()},
		{"mixed case duplicate is ignored", "RIFAMPICIN", base.Len()},
		{"new name is added", "pretomanid", base.Len() + 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.Add(tc.add).Len(); got != tc.wantLen {
				t.Errorf("Add(%q) gives Len() = %d, want %d", tc.add, got, tc.wantLen)
			}
		})
	}
}

func TestMoleculeListRemove(t *testing.T) {
	l := NewMoleculeList()
	before := l.Len()
	l2 := l.Remove("Tramadol")
	if l2.Has("tramadol") {
		t.Error("Remove() did not drop the molecule")
	}
	if l2.IsScheduleH1("Tramadol 50 mg") {
		t.Error("a removed molecule still matches")
	}
	if l2.Len() != before-1 {
		t.Errorf("Len() = %d, want %d", l2.Len(), before-1)
	}
	if !l.Has("tramadol") {
		t.Error("Remove() must not mutate the list it was called on")
	}
	if got := l.Remove("not-in-the-list").Len(); got != before {
		t.Errorf("removing an absent name changed Len() to %d, want %d", got, before)
	}
}

func TestNewMoleculeListFromNormalises(t *testing.T) {
	l := NewMoleculeListFrom([]string{" Rifampicin ", "RIFAMPICIN", "", "  ", "cefixime"})
	want := []string{"cefixime", "rifampicin"}
	if strings.Join(l.Names(), ",") != strings.Join(want, ",") {
		t.Errorf("Names() = %v, want %v", l.Names(), want)
	}
}

func TestNamesReturnsACopy(t *testing.T) {
	l := NewMoleculeList()
	names := l.Names()
	names[0] = "tampered"
	if l.Names()[0] == "tampered" {
		t.Error("Names() exposed the internal slice")
	}
}
