package h1register

import (
	"errors"
	"strings"
	"testing"
)

// complete returns an entry with every statutory column filled in.
func complete() Entry {
	return Entry{
		Serial:          1,
		DateOfSupply:    MustParseDate("2026-04-10"),
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

func TestValidate(t *testing.T) {
	allLabels := make([]string, 0, len(Fields))
	for _, f := range Fields {
		allLabels = append(allLabels, f.Label)
	}

	tests := []struct {
		name        string
		entry       Entry
		wantMissing []string
	}{
		{
			name:        "all fields present",
			entry:       complete(),
			wantMissing: nil,
		},
		{
			name:        "empty entry names every statutory field",
			entry:       Entry{},
			wantMissing: allLabels,
		},
		{
			name: "one missing field names only that field",
			entry: func() Entry {
				e := complete()
				e.BatchNo = ""
				return e
			}(),
			wantMissing: []string{"batch number"},
		},
		{
			name: "whitespace counts as blank",
			entry: func() Entry {
				e := complete()
				e.PatientAddress = "   \t  "
				return e
			}(),
			wantMissing: []string{"address of the patient"},
		},
		{
			name: "serial zero is missing",
			entry: func() Entry {
				e := complete()
				e.Serial = 0
				return e
			}(),
			wantMissing: []string{"serial number"},
		},
		{
			name: "negative serial is missing",
			entry: func() Entry {
				e := complete()
				e.Serial = -4
				return e
			}(),
			wantMissing: []string{"serial number"},
		},
		{
			name: "zero date of supply is missing",
			entry: func() Entry {
				e := complete()
				e.DateOfSupply = Date{}
				return e
			}(),
			wantMissing: []string{"date of supply"},
		},
		{
			name: "several missing fields are all reported",
			entry: func() Entry {
				e := complete()
				e.PrescriberName = ""
				e.PrescriberRegNo = ""
				e.PharmacistName = ""
				return e
			}(),
			wantMissing: []string{
				"name of the prescriber",
				"registration number of the prescriber",
				"name of the dispensing pharmacist",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.entry)
			if len(tc.wantMissing) == 0 {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want missing %v", tc.wantMissing)
			}
			var inc *IncompleteEntryError
			if !errors.As(err, &inc) {
				t.Fatalf("Validate() returned %T, want *IncompleteEntryError", err)
			}
			if got := strings.Join(inc.Missing, "|"); got != strings.Join(tc.wantMissing, "|") {
				t.Errorf("missing = %v, want %v", inc.Missing, tc.wantMissing)
			}
			if len(inc.MissingKeys) != len(inc.Missing) {
				t.Errorf("MissingKeys has %d entries, Missing has %d", len(inc.MissingKeys), len(inc.Missing))
			}
			// The message must name each missing column, so the
			// pharmacist is told what to go and get.
			for _, label := range tc.wantMissing {
				if !strings.Contains(err.Error(), label) {
					t.Errorf("error %q does not name %q", err.Error(), label)
				}
			}
		})
	}
}

func TestValidateErrorWordingNamesEveryMissingField(t *testing.T) {
	e := complete()
	e.DrugName = ""
	e.BatchNo = ""
	err := Validate(e)
	want := "incomplete register entry: missing name of the drug and batch number"
	if err == nil || err.Error() != want {
		t.Fatalf("Validate() = %v, want %q", err, want)
	}
}

func TestFieldsAreTheStatutoryColumnOrder(t *testing.T) {
	want := []string{
		"serial", "dateOfSupply", "drugName", "manufacturer", "batchNo", "quantity",
		"prescriberName", "prescriberRegNo", "patientName", "patientAddress", "pharmacistName",
	}
	if len(Fields) != len(want) {
		t.Fatalf("Fields has %d columns, want %d", len(Fields), len(want))
	}
	for i, key := range want {
		if Fields[i].Key != key {
			t.Errorf("Fields[%d].Key = %q, want %q", i, Fields[i].Key, key)
		}
		if Fields[i].Label == "" || Fields[i].Column == "" {
			t.Errorf("Fields[%d] (%s) is missing a label or column heading", i, key)
		}
	}
}

func TestRowAndHeadersMatchFieldOrder(t *testing.T) {
	row := complete().Row()
	headers := Headers()
	if len(row) != len(Fields) || len(headers) != len(Fields) {
		t.Fatalf("row has %d cells and headers %d, want %d of each", len(row), len(headers), len(Fields))
	}
	wantFirstThree := []string{"1", "2026-04-10", "Levofloxacin 500 mg tablet"}
	for i, want := range wantFirstThree {
		if row[i] != want {
			t.Errorf("row[%d] = %q, want %q", i, row[i], want)
		}
	}
	if row[len(row)-1] != "S Rao" {
		t.Errorf("last cell = %q, want the pharmacist name", row[len(row)-1])
	}
}

func TestValueUnknownKey(t *testing.T) {
	if got := complete().Value("notAColumn"); got != "" {
		t.Errorf("Value(unknown) = %q, want empty", got)
	}
}
