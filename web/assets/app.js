/* Schedule H1 sale register — local interface.
   Talks only to this program's own /api endpoints on the same origin. */
(function () {
  "use strict";

  var ENTRY_FIELDS = [
    "dateOfSupply", "drugName", "manufacturer", "batchNo", "quantity",
    "prescriberName", "prescriberRegNo", "patientName", "patientAddress",
    "pharmacistName", "notes"
  ];

  var state = null;
  var query = { drug: "", batch: "", patient: "", prescriber: "", from: "", to: "" };
  var rowsPerPage = 15;

  function $(id) { return document.getElementById(id); }

  function el(tag, className, text) {
    var n = document.createElement(tag);
    if (className) { n.className = className; }
    if (text !== undefined && text !== null) { n.textContent = String(text); }
    return n;
  }

  function clear(node) { while (node.firstChild) { node.removeChild(node.firstChild); } }

  /* ------------------------------------------------------------ theme */

  function initTheme() {
    var root = document.documentElement;
    var btn = document.querySelector("[data-themer]");
    if (!btn) { return; }
    function stored() { try { return localStorage.getItem("omsme-theme"); } catch (e) { return null; } }
    function current() {
      var t = root.getAttribute("data-theme");
      if (t) { return t; }
      return (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) ? "dark" : "light";
    }
    function paint() {
      var dark = current() === "dark";
      btn.textContent = dark ? "☀ Light" : "☾ Dark";
      btn.setAttribute("aria-label", dark ? "Switch to light theme" : "Switch to dark theme");
    }
    paint();
    btn.addEventListener("click", function () {
      var next = current() === "dark" ? "light" : "dark";
      root.setAttribute("data-theme", next);
      try { localStorage.setItem("omsme-theme", next); } catch (e) { /* private mode */ }
      paint();
    });
    if (window.matchMedia) {
      window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function () {
        if (!stored()) { paint(); }
      });
    }
  }

  /* -------------------------------------------------------------- api */

  function api(path, options) {
    return fetch(path, options || {}).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (body) {
        if (!res.ok) {
          var err = new Error(body.error || ("the request failed with status " + res.status));
          err.missingKeys = body.missingKeys || [];
          throw err;
        }
        return body;
      });
    });
  }

  function post(path, body) {
    return api(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
  }

  function queryString() {
    var parts = [];
    Object.keys(query).forEach(function (k) {
      if (query[k]) { parts.push(encodeURIComponent(k) + "=" + encodeURIComponent(query[k])); }
    });
    return parts.join("&");
  }

  function load() {
    var qs = queryString();
    var url = "/api/state?rows=" + rowsPerPage + (qs ? "&" + qs : "");
    return api(url).then(function (s) {
      state = s;
      render();
    }).catch(function (err) {
      setBanner("bad", "Could not read the register: " + err.message);
    });
  }

  /* ----------------------------------------------------------- render */

  function setBanner(kind, text) {
    var b = $("banner");
    b.className = "banner" + (kind ? " " + kind : "");
    b.textContent = text;
  }

  function render() {
    renderBanner();
    renderNextSerial();
    renderRegister();
    renderPrintable();
    renderSetup();
  }

  function renderBanner() {
    var c = state.continuity;
    var text = c.summary;
    if (c.ok && !c.startsAtFirstSerial && c.count > 0) {
      text += " This volume starts at serial " + c.first + ", so it continues an earlier register.";
    }
    setBanner(c.ok ? "ok" : "bad", text);
  }

  function renderNextSerial() {
    $("next-serial").textContent = state.nextSerial;
  }

  function retentionCell(entry) {
    var r = entry.retention;
    var span = el("span", r.mayDestroy ? "tag-drop" : "tag-keep", r.status);
    span.title = r.detail;
    return span;
  }

  function renderRegister() {
    var head = $("register-head");
    var body = $("register-body");
    clear(head);
    clear(body);

    var tr = el("tr");
    state.fields.forEach(function (f) {
      tr.appendChild(el("th", f.key === "serial" ? "num" : null, f.column));
    });
    tr.appendChild(el("th", null, "Retain until"));
    tr.appendChild(el("th", null, "Retention"));
    head.appendChild(tr);

    state.entries.forEach(function (entry) {
      var row = el("tr");
      state.fields.forEach(function (f) {
        var cell = el("td", f.key === "serial" ? "num" : null, cellValue(entry, f.key));
        if (f.key === "drugName" && entry.h1Match && entry.h1Match.length) {
          cell.appendChild(document.createTextNode(" "));
          var tag = el("span", "tag-h1", entry.h1Match.join(", "));
          tag.title = "Matched against your Schedule H1 molecule list";
          cell.appendChild(tag);
        }
        row.appendChild(cell);
      });
      row.appendChild(el("td", "num", entry.retention.until));
      var rc = el("td");
      rc.appendChild(retentionCell(entry));
      row.appendChild(rc);
      body.appendChild(row);
    });

    var count = $("register-count");
    if (state.total === 0) {
      count.textContent = "The register is empty.";
    } else if (state.filtered) {
      count.textContent = state.shown + " of " + state.total + " entries match. " +
        state.mustRetain + " must be retained, " + state.mayDestroy + " past the " + state.retentionRule + ".";
    } else {
      count.textContent = state.total + " entries. " + state.mustRetain + " must be retained, " +
        state.mayDestroy + " past the " + state.retentionRule + ".";
    }
    $("register-empty").hidden = state.shown !== 0 || state.total === 0;

    var qs = queryString();
    $("export-csv").href = "/api/export.csv" + (qs ? "?" + qs : "");
  }

  function cellValue(entry, key) {
    if (key === "serial") { return entry.serial; }
    if (key === "dateOfSupply") { return entry.dateOfSupply; }
    return entry[key] || "";
  }

  function rangeLabel() {
    if (query.from && query.to) { return query.from + " to " + query.to; }
    if (query.from) { return "from " + query.from; }
    if (query.to) { return "up to " + query.to; }
    return "all entries";
  }

  function renderPrintable() {
    var host = $("printable");
    clear(host);
    $("print-range").value = rangeLabel();

    var bySerial = {};
    state.entries.forEach(function (e) { bySerial[e.serial] = e; });

    state.pages.forEach(function (page) {
      var sheet = el("div", "regpage");

      var head = el("div", "regpage-head");
      var left = el("div");
      left.appendChild(el("h3", null, "Schedule H1 sale register"));
      left.appendChild(el("div", "lic", (state.pharmacy.name || "Pharmacy name not set") +
        (state.pharmacy.licenceNo ? " · Licence " + state.pharmacy.licenceNo : "")));
      if (state.pharmacy.address) { left.appendChild(el("div", "lic", state.pharmacy.address)); }
      head.appendChild(left);

      var right = el("div", "meta");
      right.appendChild(el("div", null, "Page " + page.number + " of " + page.of));
      right.appendChild(el("div", null, "Period: " + rangeLabel()));
      right.appendChild(el("div", null, "Printed " + state.asOf));
      head.appendChild(right);
      sheet.appendChild(head);

      var table = el("table", "reg");
      var thead = el("thead");
      var tr = el("tr");
      state.fields.forEach(function (f) {
        tr.appendChild(el("th", f.key === "serial" ? "num" : null, f.column));
      });
      thead.appendChild(tr);
      table.appendChild(thead);

      var tbody = el("tbody");
      page.serials.forEach(function (serial) {
        var entry = bySerial[serial];
        if (!entry) { return; }
        var row = el("tr");
        state.fields.forEach(function (f) {
          row.appendChild(el("td", f.key === "serial" ? "num" : null, cellValue(entry, f.key)));
        });
        tbody.appendChild(row);
      });
      table.appendChild(tbody);
      // Wrapped so the eleven columns scroll on a narrow screen; the wrapper
      // is set back to visible overflow when printing.
      var wrap = el("div", "tablewrap");
      wrap.appendChild(table);
      sheet.appendChild(wrap);

      var foot = el("div", "regpage-foot");
      foot.appendChild(el("span", null, "Retention: " + state.retentionRule +
        ". Open to inspection under Rule 65(11A), Drugs and Cosmetics Rules, 1945."));
      foot.appendChild(el("span", null, "Signature of the registered pharmacist: ______________________"));
      sheet.appendChild(foot);

      host.appendChild(sheet);
    });
  }

  function renderSetup() {
    $("p-name").value = state.pharmacy.name || "";
    $("p-address").value = state.pharmacy.address || "";
    $("p-licenceNo").value = state.pharmacy.licenceNo || "";
    $("p-pharmacist").value = state.pharmacy.pharmacist || "";

    $("molecule-count").textContent = state.molecules.length;
    $("molecule-source").textContent = state.moleculesEdited
      ? "You have edited this list."
      : "This is the list supplied with the tool, unedited.";

    var list = $("molecule-list");
    clear(list);
    state.molecules.forEach(function (name) {
      var li = el("li");
      li.appendChild(el("span", null, name));
      var btn = el("button", null, "×");
      btn.type = "button";
      btn.setAttribute("aria-label", "Remove " + name);
      btn.addEventListener("click", function () {
        post("/api/molecules", { remove: name }).then(load).then(function () {
          message("molecule-msg", "ok", "Removed " + name + ".");
        }).catch(function (err) { message("molecule-msg", "bad", err.message); });
      });
      li.appendChild(btn);
      list.appendChild(li);
    });
  }

  function message(id, kind, text) {
    var n = $(id);
    n.className = "formmsg" + (kind ? " " + kind : "");
    n.textContent = text;
  }

  /* -------------------------------------------------------- new entry */

  function entryInput(key) { return $("f-" + key); }

  function clearMissingMarks() {
    ENTRY_FIELDS.forEach(function (key) {
      var input = entryInput(key);
      if (input) { input.classList.remove("missing"); }
    });
  }

  function readEntry() {
    var out = {};
    ENTRY_FIELDS.forEach(function (key) {
      var input = entryInput(key);
      out[key] = input ? input.value.trim() : "";
    });
    return out;
  }

  function submitEntry(event) {
    event.preventDefault();
    clearMissingMarks();
    message("entry-msg", "", "");

    var entry = readEntry();
    entry.serial = state ? state.nextSerial : 0;

    post("/api/entries", entry).then(function (res) {
      var serial = res.entry.serial;
      ["drugName", "manufacturer", "batchNo", "quantity", "prescriberName", "prescriberRegNo",
        "patientName", "patientAddress", "notes"].forEach(function (key) {
          var input = entryInput(key);
          if (input) { input.value = ""; }
        });
      return load().then(function () {
        message("entry-msg", "ok", "Serial " + serial + " written. Retain until " +
          res.entry.retention.until + ".");
        entryInput("drugName").focus();
      });
    }).catch(function (err) {
      (err.missingKeys || []).forEach(function (key) {
        var input = entryInput(key);
        if (input) { input.classList.add("missing"); }
      });
      message("entry-msg", "bad", err.message);
      var first = (err.missingKeys || [])[0];
      if (first && entryInput(first)) { entryInput(first).focus(); }
    });
  }

  var h1Timer = null;

  function checkH1() {
    var flag = $("h1-flag");
    var drug = entryInput("drugName").value.trim();
    if (!drug) {
      flag.className = "h1flag";
      flag.textContent = "";
      return;
    }
    api("/api/h1check?drug=" + encodeURIComponent(drug)).then(function (res) {
      if (res.isH1) {
        flag.className = "h1flag hit";
        flag.textContent = "Looks like a Schedule H1 molecule (" + res.matches.join(", ") +
          "). Supply only against a prescription and record it here. Check the pack and the Schedule.";
      } else {
        flag.className = "h1flag";
        flag.textContent = "No match in your Schedule H1 list. That does not make it non-H1 — check the pack, and add the molecule to your list if it is.";
      }
    }).catch(function () { /* the warning is advisory; a failed check stays silent */ });
  }

  function scheduleH1Check() {
    if (h1Timer) { window.clearTimeout(h1Timer); }
    h1Timer = window.setTimeout(checkH1, 250);
  }

  function undoLast() {
    if (!window.confirm("Remove the last line from the register? Only the most recent line can be removed, so no gap is left in the serial numbers.")) {
      return;
    }
    api("/api/entries/last", { method: "DELETE" }).then(function (res) {
      return load().then(function () {
        message("entry-msg", "ok", "Serial " + res.removed + " removed. The next line will be serial " + res.nextSerial + ".");
      });
    }).catch(function (err) { message("entry-msg", "bad", err.message); });
  }

  /* ----------------------------------------------------------- filters */

  function applyFilter(event) {
    if (event) { event.preventDefault(); }
    query.drug = $("q-drug").value.trim();
    query.batch = $("q-batch").value.trim();
    query.patient = $("q-patient").value.trim();
    query.prescriber = $("q-prescriber").value.trim();
    query.from = $("q-from").value;
    query.to = $("q-to").value;
    load();
  }

  function clearFilter() {
    ["q-drug", "q-batch", "q-patient", "q-prescriber", "q-from", "q-to"].forEach(function (id) {
      $(id).value = "";
    });
    applyFilter(null);
  }

  /* -------------------------------------------------------------- views */

  function showView(name) {
    ["entry", "register", "print", "setup"].forEach(function (v) {
      $("view-" + v).hidden = v !== name;
    });
    document.querySelectorAll(".views button").forEach(function (b) {
      b.setAttribute("aria-pressed", String(b.getAttribute("data-view") === name));
    });
  }

  /* --------------------------------------------------------------- wire */

  function wire() {
    $("entry-form").addEventListener("submit", submitEntry);
    entryInput("drugName").addEventListener("input", scheduleH1Check);
    $("undo-last").addEventListener("click", undoLast);

    $("filter-form").addEventListener("submit", applyFilter);
    $("filter-clear").addEventListener("click", clearFilter);

    $("rows-per-page").addEventListener("change", function () {
      var n = parseInt($("rows-per-page").value, 10);
      if (!isFinite(n) || n < 1 || n > 200) { return; }
      rowsPerPage = n;
      load();
    });
    $("do-print").addEventListener("click", function () { window.print(); });

    $("pharmacy-form").addEventListener("submit", function (event) {
      event.preventDefault();
      post("/api/pharmacy", {
        name: $("p-name").value.trim(),
        address: $("p-address").value.trim(),
        licenceNo: $("p-licenceNo").value.trim(),
        pharmacist: $("p-pharmacist").value.trim()
      }).then(load).then(function () {
        message("pharmacy-msg", "ok", "Saved.");
        prefill();
      }).catch(function (err) { message("pharmacy-msg", "bad", err.message); });
    });

    $("molecule-form").addEventListener("submit", function (event) {
      event.preventDefault();
      var name = $("m-add").value.trim();
      if (!name) { return; }
      post("/api/molecules", { add: name }).then(function () {
        $("m-add").value = "";
        return load();
      }).then(function () {
        message("molecule-msg", "ok", "Added " + name.toLowerCase() + ".");
        checkH1();
      }).catch(function (err) { message("molecule-msg", "bad", err.message); });
    });

    $("molecule-reset").addEventListener("click", function () {
      post("/api/molecules", { reset: true }).then(load).then(function () {
        message("molecule-msg", "ok", "Restored the list supplied with the tool.");
      }).catch(function (err) { message("molecule-msg", "bad", err.message); });
    });

    document.querySelectorAll(".views button").forEach(function (b) {
      b.addEventListener("click", function () { showView(b.getAttribute("data-view")); });
    });
  }

  function prefill() {
    var date = entryInput("dateOfSupply");
    if (!date.value && state) { date.value = state.asOf; }
    var ph = entryInput("pharmacistName");
    if (!ph.value && state && state.pharmacy.pharmacist) { ph.value = state.pharmacy.pharmacist; }
  }

  initTheme();
  wire();
  load().then(prefill);
})();
