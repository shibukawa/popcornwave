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
      const sorted = rows().sort((a, b) => {
        const left = cellText(a, index), right = cellText(b, index);
        const numeric = left !== "" && right !== "" && !isNaN(Number(left)) && !isNaN(Number(right));
        const order = numeric ? Number(left) - Number(right) : left.localeCompare(right);
        return ascending ? order : -order;
      });
      for (const row of sorted) body.appendChild(row);
    });
  }

  // Filtering. It matches anywhere in the row, because a developer filtering a
  // development table is looking for a value, not composing a predicate — the
  // statement console is where a predicate belongs.
  const filter = document.getElementById("filter");
  const shown = document.getElementById("shown");
  const applyFilter = () => {
    const needle = filter.value.trim().toLowerCase();
    let visible = 0;
    for (const row of rows()) {
      const hit = !needle || Array.from(row.cells).slice(1)
        .some((_, index) => cellText(row, index).toLowerCase().includes(needle));
      row.hidden = !hit;
      if (hit) visible++;
    }
    shown.textContent = needle ? visible + " of " + rows().length + " rows" : "";
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
    return { edited, deleted: row.dataset.deleted === "1" };
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
        button.textContent = marked ? "revert" : "del";
        button.dataset.act = marked ? "revert" : "delete";
        button.classList.toggle("danger", !marked);
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
    if (button.dataset.act === "revert") {
      row.dataset.deleted = "";
      for (const input of row.querySelectorAll("input")) input.value = input.dataset.original;
    } else {
      row.dataset.deleted = "1";
    }
    refresh();
  });

  save.addEventListener("click", async () => {
    const edits = [], deletes = [];
    for (const row of rows()) {
      const key = JSON.parse(row.dataset.key || "{}");
      const state = rowState(row);
      if (state.deleted) { deletes.push({ key }); continue; }
      if (!state.edited) continue;
      const values = {};
      for (const cell of row.querySelectorAll("td[data-column]")) {
        const input = cell.querySelector("input");
        if (input && input.value !== input.dataset.original) values[cell.dataset.column] = input.value;
      }
      edits.push({ key, values });
    }
    save.disabled = true;
    const response = await fetch(table.dataset.endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ edits, deletes }),
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
