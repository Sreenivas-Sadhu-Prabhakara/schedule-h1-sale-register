package h1register

import (
	"strconv"
	"strings"
)

// Entry is one line of the Schedule H1 sale register: a single supply of a
// Schedule H1 drug against a prescription.
//
// Every field below must be filled in. The Schedule H1 condition itself names
// the prescriber (name and address), the patient, the drug and the quantity;
// the remaining columns — batch number, manufacturer, the prescriber's
// registration number and the dispensing pharmacist — are the ones inspection
// formats in practice ask for, and a register without them is hard to defend.
// This tool treats all of them as mandatory.
type Entry struct {
	Serial          int    `json:"serial"`
	DateOfSupply    Date   `json:"dateOfSupply"`
	DrugName        string `json:"drugName"`
	Manufacturer    string `json:"manufacturer"`
	BatchNo         string `json:"batchNo"`
	Quantity        string `json:"quantity"`
	PrescriberName  string `json:"prescriberName"`
	PrescriberRegNo string `json:"prescriberRegNo"`
	PatientName     string `json:"patientName"`
	PatientAddress  string `json:"patientAddress"`
	PharmacistName  string `json:"pharmacistName"`
	Notes           string `json:"notes,omitempty"`
}

// Field describes one register column: its JSON key, the label used in the
// form and in validation errors, and the short heading used on the printed
// register page.
type Field struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Column string `json:"column"`
}

// Fields is the statutory column order of the register, declared once here and
// used everywhere: the entry form, validation messages, the printable
// bound-register view and the CSV export all read this list, so the columns can
// never drift apart from each other.
var Fields = []Field{
	{Key: "serial", Label: "serial number", Column: "Sl. No."},
	{Key: "dateOfSupply", Label: "date of supply", Column: "Date of supply"},
	{Key: "drugName", Label: "name of the drug", Column: "Name of drug"},
	{Key: "manufacturer", Label: "manufacturer", Column: "Manufacturer"},
	{Key: "batchNo", Label: "batch number", Column: "Batch No."},
	{Key: "quantity", Label: "quantity supplied", Column: "Quantity"},
	{Key: "prescriberName", Label: "name of the prescriber", Column: "Prescriber"},
	{Key: "prescriberRegNo", Label: "registration number of the prescriber", Column: "Reg. No."},
	{Key: "patientName", Label: "name of the patient", Column: "Patient"},
	{Key: "patientAddress", Label: "address of the patient", Column: "Patient address"},
	{Key: "pharmacistName", Label: "name of the dispensing pharmacist", Column: "Pharmacist"},
}

// Value returns the entry's value for a register column as it should be
// printed. An unknown key returns "".
func (e Entry) Value(key string) string {
	switch key {
	case "serial":
		if e.Serial <= 0 {
			return ""
		}
		return strconv.Itoa(e.Serial)
	case "dateOfSupply":
		return e.DateOfSupply.String()
	case "drugName":
		return strings.TrimSpace(e.DrugName)
	case "manufacturer":
		return strings.TrimSpace(e.Manufacturer)
	case "batchNo":
		return strings.TrimSpace(e.BatchNo)
	case "quantity":
		return strings.TrimSpace(e.Quantity)
	case "prescriberName":
		return strings.TrimSpace(e.PrescriberName)
	case "prescriberRegNo":
		return strings.TrimSpace(e.PrescriberRegNo)
	case "patientName":
		return strings.TrimSpace(e.PatientName)
	case "patientAddress":
		return strings.TrimSpace(e.PatientAddress)
	case "pharmacistName":
		return strings.TrimSpace(e.PharmacistName)
	default:
		return ""
	}
}

// Row renders the entry in statutory column order, ready for the printed
// register or a CSV line.
func (e Entry) Row() []string {
	row := make([]string, 0, len(Fields))
	for _, f := range Fields {
		row = append(row, e.Value(f.Key))
	}
	return row
}

// Headers returns the register column headings in statutory order.
func Headers() []string {
	h := make([]string, 0, len(Fields))
	for _, f := range Fields {
		h = append(h, f.Column)
	}
	return h
}

// Trim normalises every text field, so a value of only spaces counts as blank
// everywhere.
func (e Entry) Trim() Entry {
	e.DrugName = strings.TrimSpace(e.DrugName)
	e.Manufacturer = strings.TrimSpace(e.Manufacturer)
	e.BatchNo = strings.TrimSpace(e.BatchNo)
	e.Quantity = strings.TrimSpace(e.Quantity)
	e.PrescriberName = strings.TrimSpace(e.PrescriberName)
	e.PrescriberRegNo = strings.TrimSpace(e.PrescriberRegNo)
	e.PatientName = strings.TrimSpace(e.PatientName)
	e.PatientAddress = strings.TrimSpace(e.PatientAddress)
	e.PharmacistName = strings.TrimSpace(e.PharmacistName)
	e.Notes = strings.TrimSpace(e.Notes)
	return e
}

// IncompleteEntryError reports every blank mandatory column of an entry at
// once, named in statutory order. A half-filled line in the register is the
// defect an inspector writes up, so the entry is refused until all of it is
// there.
type IncompleteEntryError struct {
	// MissingKeys holds the JSON keys of the blank columns, so a form can
	// highlight exactly those inputs.
	MissingKeys []string
	// Missing holds the human labels of the blank columns.
	Missing []string
}

// Error names every missing column.
func (e *IncompleteEntryError) Error() string {
	return "incomplete register entry: missing " + joinList(e.Missing)
}

// Validate reports every blank mandatory column of the entry. It returns nil
// only when the whole statutory field set is present.
func Validate(e Entry) error {
	e = e.Trim()
	miss := &IncompleteEntryError{}
	for _, f := range Fields {
		if e.Value(f.Key) == "" {
			miss.MissingKeys = append(miss.MissingKeys, f.Key)
			miss.Missing = append(miss.Missing, f.Label)
		}
	}
	if len(miss.Missing) == 0 {
		return nil
	}
	return miss
}

// joinList renders "a", "a and b", "a, b and c".
func joinList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}
