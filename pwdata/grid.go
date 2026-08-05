package pwdata

// gridScript is the browser half of the table page.
//
// It is development-only code under decision:dev-browser-runtime-scope, which
// is why it may exist at all: nothing here reaches an application, and the
// pane is the one page in the console where a grid earns its keep.
//
// Everything it does is local to the page. Sorting and filtering never ask the
// server, because the page holds one bounded read already and a round trip to
// reorder fifty rows would be slower and would lose the edits in progress.
const gridScript = `
(() => {
  const table = document.getElementById("rows");
  if (!table) return;

  // Tabs. The schema and the data are the same table asked two questions, and
  // showing both at once made the page about neither.
  for (const tab of document.querySelectorAll('[role="tab"]')) {
    tab.addEventListener("click", () => {
      for (const other of document.querySelectorAll('[role="tab"]')) {
        const selected = other === tab;
        other.setAttribute("aria-selected", String(selected));
        document.getElementById("panel-" + other.dataset.panel).hidden = !selected;
      }
    });
  }

  const body = table.tBodies[0];
  const rows = () => Array.from(body.rows);
  const cellText = (row, index) => {
    const cell = row.cells[index + 1];
    const input = cell.querySelector("input");
    return (input ? input.value : cell.textContent).trim();
  };

  // Sorting. Numbers compare as numbers where both sides are numeric, because
  // a column of ids sorted as text is the one sort nobody wants.
  let sortIndex = -1, ascending = true;
  for (const header of table.querySelectorAll("th.sortable")) {
    header.addEventListener("click", () => {
      const index = Number(header.dataset.index);
      ascending = index === sortIndex ? !ascending : true;
      sortIndex = index;
      for (const other of table.querySelectorAll("th.sortable .dir")) other.textContent = "";
      header.querySelector(".dir").textContent = ascending ? "↑" : "↓";
      const sorted = rows().filter(row => !isBlank(row)).sort((a, b) => {
        const left = cellText(a, index), right = cellText(b, index);
        const numeric = left !== "" && right !== "" && !isNaN(Number(left)) && !isNaN(Number(right));
        const order = numeric ? Number(left) - Number(right) : left.localeCompare(right);
        return ascending ? order : -order;
      });
      const blanks = rows().filter(isBlank);
      if (blanks.length) body.appendChild(blanks[0]);
      for (const row of sorted) body.appendChild(row);
      for (const row of blanks.slice(1)) body.appendChild(row);
      // The first blank moves back to the top after the sort reordered the rest.
      if (blanks.length) body.insertBefore(blanks[0], body.firstChild);
    });
  }

  // A blank row is never filtered away or sorted into the middle: it is the
  // place to type, and it belongs at the ends where it was put.
  const isBlank = row => row.dataset.new === "1";

  // Filtering. It matches anywhere in the row, because a developer filtering a
  // development table is looking for a value, not composing a predicate — the
  // statement console is where a predicate belongs.
  const filter = document.getElementById("filter");
  const shown = document.getElementById("shown");
  const applyFilter = () => {
    const needle = filter.value.trim().toLowerCase();
    let visible = 0;
    for (const row of rows()) {
      if (isBlank(row)) continue;
      const hit = !needle || Array.from(row.cells).slice(1)
        .some((_, index) => cellText(row, index).toLowerCase().includes(needle));
      row.hidden = !hit;
      if (hit) visible++;
    }
    const total = rows().filter(row => !isBlank(row)).length;
    shown.textContent = needle ? visible + " of " + total + " rows" : "";
  };
  filter.addEventListener("input", applyFilter);

  // Editing. A change marks its cell and turns that row's delete into a revert,
  // so the row says what will happen to it and the way back is where the way
  // forward was. Nothing reaches the database until save.
  const save = document.getElementById("save");
  if (!save) return;
  const count = document.getElementById("dirtycount");

  const rowState = row => {
    const edited = Array.from(row.querySelectorAll("input")).some(i => i.value !== i.dataset.original);
    // A blank row is not an edit of anything; it is an insert waiting to be
    // filled, and an untouched one is not a change at all.
    return { edited, deleted: row.dataset.deleted === "1", isNew: row.dataset.new === "1" };
  };
  const refresh = () => {
    let dirty = 0;
    for (const row of rows()) {
      const state = rowState(row);
      const marked = state.edited || state.deleted;
      row.classList.toggle("dirty", marked);
      row.style.opacity = state.deleted ? "0.45" : "";
      for (const input of row.querySelectorAll("input")) {
        input.classList.toggle("dirty", input.value !== input.dataset.original);
      }
      const button = row.querySelector("button.act");
      if (button) {
        if (state.isNew) {
          button.textContent = "clr";
          button.dataset.act = "clear";
          button.classList.remove("danger");
        } else {
          button.textContent = marked ? "revert" : "del";
          button.dataset.act = marked ? "revert" : "delete";
          button.classList.toggle("danger", !marked);
        }
      }
      if (marked) dirty++;
    }
    save.disabled = dirty === 0;
    count.textContent = dirty === 0 ? "no changes" : dirty + " row(s) changed";
  };

  body.addEventListener("input", event => {
    if (event.target.tagName === "INPUT") refresh();
  });
  body.addEventListener("click", event => {
    const button = event.target.closest("button.act");
    if (!button) return;
    const row = button.closest("tr");
    if (button.dataset.act === "revert" || button.dataset.act === "clear") {
      row.dataset.deleted = "";
      for (const input of row.querySelectorAll("input")) input.value = input.dataset.original;
    } else {
      row.dataset.deleted = "1";
    }
    refresh();
  });

  save.addEventListener("click", async () => {
    const edits = [], deletes = [], inserts = [];
    for (const row of rows()) {
      const state = rowState(row);
      if (!state.edited && !state.deleted) continue;
      const values = {};
      for (const cell of row.querySelectorAll("td[data-column]")) {
        const input = cell.querySelector("input");
        if (!input) continue;
        // An insert sends only what was filled in, so a column left blank
        // takes the default the schema decided rather than an empty string.
        if (state.isNew ? input.value !== "" : input.value !== input.dataset.original) {
          values[cell.dataset.column] = input.value;
        }
      }
      if (state.isNew) { inserts.push({ values }); continue; }
      const key = JSON.parse(row.dataset.key || "{}");
      if (state.deleted) { deletes.push({ key }); continue; }
      edits.push({ key, values });
    }
    save.disabled = true;
    const response = await fetch(table.dataset.endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ edits, deletes, inserts }),
    });
    if (response.ok) { location.reload(); return; }
    // A refused write leaves the page as it was, edits intact, so nothing is
    // lost to a message the developer then has to act on.
    count.textContent = await response.text();
    count.className = "bad";
    save.disabled = false;
  });

  refresh();
})();
`
