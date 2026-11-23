# Schedule H1 sale register

A free, offline tool that keeps the separate register of Schedule H1 drug supplies that a pharmacy must maintain: the full statutory column set, serial numbers that cannot silently break, three-year retention worked out for every line, and clean pages to print for an inspector.

One binary, no dependencies, no account. It serves a small web interface on your own machine and stores the register in one local file.

**Explainer site:** https://sreenivas-sadhu-prabhakara.github.io/schedule-h1-sale-register/

## What it does

- **Guided entry that cannot be half-filled.** Every statutory column is required; a refused entry names each column still blank and marks the inputs.
- **Serial continuity audit.** Gaps and reused serial numbers are reported outright, by number, as an inspection failure.
- **Schedule H1 warning while you type.** The drug name is matched against your molecule list, including inside brand and combination names.
- **Three-year retention per line.** Calendar-correct dates, leap years included, with must-retain / may-destroy status and days remaining.
- **Printable bound-register view.** Statutory column order, paginated, with your pharmacy name and licence number at the head of each page.
- **Search and export.** Filter by drug, batch, patient, prescriber or date range, then print just that period or export it as CSV.
- **A molecule list you keep current.** The supplied list is a starting point you can add to and prune; your edits are saved with the register.

## Who it's for

Retail chemists and hospital pharmacies in India that dispense Schedule H1 drugs — the anti-TB drugs, the fluoroquinolones and third-generation cephalosporins, the carbapenems and the habit-forming molecules. Built for the single-counter pharmacy that keeps its register by hand.

## The rule it implements

The condition attached to **Schedule H1** of the **Drugs and Cosmetics Rules, 1945**, read with **Rule 65(11A)**: a drug specified in Schedule H1 may be supplied only against the prescription of a registered medical practitioner, the supply must be **recorded in a separate register at the time of supply** giving the prescriber, the patient, the drug and the quantity supplied, and the record must be **retained for three years and be open to inspection**.

Columns required by this tool, in order: serial number, date of supply, name of the drug, manufacturer, batch number, quantity supplied, name of the prescriber, registration number of the prescriber, name of the patient, address of the patient, name of the dispensing pharmacist.

The Rule's own wording names the prescriber, the patient, the drug and the quantity. The batch number, manufacturer, prescriber's registration number and dispensing pharmacist are included because inspection formats in practice ask for them. All eleven are mandatory here.

The retention period is a single named constant, `RetentionYears = 3` in `internal/h1register/retention.go`, so one line changes it if it is ever amended. The Schedule H1 molecule list is likewise a single named constant, `ScheduleH1Seed` in `internal/h1register/molecules.go`, and is editable from the interface without rebuilding.

## Install and run

```sh
git clone https://github.com/sreenivas-sadhu-prabhakara/schedule-h1-sale-register
cd schedule-h1-sale-register

# run it straight away
go run .

# or build the binary once and keep it
go build -o schedule-h1-sale-register .
./schedule-h1-sale-register
```

Then open <http://127.0.0.1:8080/>.

| Flag    | Default                                  | What it does                     |
| ------- | ---------------------------------------- | -------------------------------- |
| `-addr` | `127.0.0.1:8080`                         | Address the interface is served on |
| `-data` | `./schedule-h1-sale-register-data.json`  | Path to the register data file    |

It binds to the loopback address only, so nothing on your network can reach it.

## Where data is stored

One JSON file on the same machine — `./schedule-h1-sale-register-data.json` unless `-data` points elsewhere. It holds your pharmacy details, your Schedule H1 molecule list and every register entry. Writes are atomic: the file is written to a temporary file in the same directory and renamed over the old one, so an interrupted write cannot leave half a register behind.

Nothing leaves the machine. There are no outbound network calls of any kind — no telemetry, no analytics, no update check, no external fonts or scripts, and no connection to any drug-control portal. The register holds patient names and addresses, so the data file is created readable only by your own user account. Back it up as you would any patient record, and keep the backups for at least the retention period.

## Scope and accuracy

**This tool records and prints the register. It does not transmit anything to any authority, and it is not a substitute for the bound physical register where one is required.** A bound, serially paginated register is what an inspector asks to see, and state drug-control authorities commonly expect it. Use this to keep the data straight, find entries quickly and print clean pages; keep the book as well.

**The supplied Schedule H1 molecule list is not authoritative and not complete.** Schedule H1 is amended by notification, so molecules are added to it over time and any fixed list goes stale. The list shipped here is a data-entry aid: a warning is a prompt to check the pack and the Schedule, and the absence of a warning does not make a drug non-H1. Keep your own list current against Schedule H1 as notified by the Ministry of Health and Family Welfare, and confirm the current Schedule, the register format and the retention period with your State Drugs Controller or on the CDSCO website before relying on them.

A supply on 29 February is retained until 28 February three years later, the anniversary being the last real day of that month. Hold it to 1 March if you would rather keep a day in hand.

This is not legal or regulatory advice. The responsibility for the register, for what is dispensed and for the pharmacist's presence at the counter remains with the licensee.

## Tests

```sh
gofmt -l .
go vet ./...
go test ./... -count=1
go build -o /tmp/schedule-h1-sale-register-bin .
```

The rules live in `internal/h1register` as pure functions with table-driven tests: field validation, serial continuity, calendar retention arithmetic, molecule matching, search and pagination.

MIT licensed.
