"use strict";

// Glue layer: htmx owns data/views (server-rendered fragments); this file
// owns what htmx can't — selection at scale, lightbox, folder navigation,
// toasts, and anime.js transitions.

const $ = (sel) => document.querySelector(sel);
const grid = $("#grid");
const filters = $("#filters");

const sel = {
  items: new Map(), // key -> {env,type,path}
  filterMode: false, // when true, actions target everything matching the filter
  lastIndex: -1, // anchor for shift-range selection
};

function keyOf(el) { return `${el.dataset.env}|${el.dataset.type}|${el.dataset.path}`; }
function refOf(el) { return { env: el.dataset.env, type: el.dataset.type, path: el.dataset.path }; }
function cards() { return [...grid.querySelectorAll(".card:not(.dupecard)")]; }

function currentFilter() {
  const fd = new FormData(filters);
  return {
    env: fd.get("env") || "", type: fd.get("type") || "", q: fd.get("q") || "",
    folder: fd.get("folder") || "", from: fd.get("from") || "", to: fd.get("to") || "",
  };
}

function toast(msg, isError) {
  const el = document.createElement("div");
  el.className = "toast" + (isError ? " error" : "");
  el.textContent = msg;
  document.body.appendChild(el);
  if (window.anime) anime({ targets: el, translateY: [20, 0], opacity: [0, 1], duration: 250, easing: "easeOutQuad" });
  setTimeout(() => el.remove(), isError ? 6000 : 2800);
}

// ---------- modal system (replaces native alert/confirm/prompt) ----------

const modalEl = $("#modal");
let modalResolve = null;

// showModal renders a dialog and resolves with the clicked button's value
// (or the input text for the confirming button when input is enabled).
// Backdrop click / Escape resolve null.
function showModal({ title, message = "", input = null, buttons }) {
  return new Promise((resolve) => {
    modalResolve = resolve;
    $("#modalTitle").textContent = title;
    $("#modalMessage").textContent = message;
    $("#modalMessage").classList.toggle("hidden", !message);
    const inp = $("#modalInput");
    inp.classList.toggle("hidden", input === null);
    if (input !== null) {
      inp.value = input.value ?? "";
      inp.placeholder = input.placeholder ?? "";
    }
    const btns = $("#modalButtons");
    btns.innerHTML = "";
    for (const b of buttons) {
      const el = document.createElement("button");
      el.textContent = b.label;
      if (b.danger) el.classList.add("danger");
      if (b.primary) el.classList.add("primary");
      el.onclick = () => closeModal(b.input ? inp.value.trim() : b.value);
      btns.appendChild(el);
    }
    modalEl.classList.remove("hidden");
    if (window.anime) anime({ targets: "#modalBody", scale: [0.94, 1], opacity: [0, 1], duration: 160, easing: "easeOutQuad" });
    if (input !== null) { inp.focus(); inp.select(); }
  });
}

function closeModal(result) {
  modalEl.classList.add("hidden");
  const r = modalResolve;
  modalResolve = null;
  if (r) r(result ?? null);
}

modalEl.addEventListener("click", (e) => { if (e.target.id === "modal") closeModal(null); });
$("#modalInput").addEventListener("keydown", (e) => {
  if (e.key === "Enter") closeModal($("#modalInput").value.trim());
});

function confirmModal(title, message, confirmLabel = "Confirm", danger = true) {
  return showModal({
    title, message,
    buttons: [
      { label: "Cancel", value: null },
      { label: confirmLabel, value: true, danger, primary: !danger },
    ],
  });
}

async function promptModal(title, message, value = "", confirmLabel = "OK") {
  const r = await showModal({
    title, message,
    input: { value },
    buttons: [
      { label: "Cancel", value: null },
      { label: confirmLabel, input: true, primary: true },
    ],
  });
  return typeof r === "string" ? r : null;
}

async function api(path, body) {
  const r = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  let json = {};
  try { json = await r.json(); } catch { /* non-JSON */ }
  if (!r.ok || json.success === false) {
    throw new Error(json.error || (json.errors || []).join("; ") || `HTTP ${r.status}`);
  }
  return json;
}

function refreshGrid() {
  clearSelection();
  htmx.trigger(document.body, "refresh-grid");
}

// ---------- selection ----------

function totalCount() {
  const status = $("#gridStatus");
  if (!status) return 0;
  const m = status.textContent.match(/^(\d+)/);
  return m ? parseInt(m[1], 10) : 0;
}

function updateSelectionBar() {
  const bar = $("#selectionBar");
  const n = sel.filterMode ? totalCount() : sel.items.size;
  const show = n > 0 || sel.filterMode;
  if (show && bar.classList.contains("hidden")) {
    bar.classList.remove("hidden");
    if (window.anime) anime({ targets: bar, translateY: [-12, 0], opacity: [0, 1], duration: 200, easing: "easeOutQuad" });
  } else if (!show) {
    bar.classList.add("hidden");
  }
  $("#selCount").textContent = sel.filterMode ? `ALL ${n} matching` : `${n} selected`;
  const total = totalCount();
  const btn = $("#selectAllBtn");
  btn.textContent = sel.filterMode ? "Back to manual selection" : `Select all ${total} matching`;
  btn.classList.toggle("hidden", total === 0);
}

function setCardSelected(card, on) {
  card.classList.toggle("selected", on);
  const check = card.querySelector(".check");
  if (check) check.checked = on;
}

function toggleCard(card, forceOn) {
  const k = keyOf(card);
  const on = forceOn !== undefined ? forceOn : !sel.items.has(k);
  if (on) sel.items.set(k, refOf(card));
  else sel.items.delete(k);
  setCardSelected(card, on);
}

function selectRange(toIndex) {
  const list = cards();
  if (sel.lastIndex < 0 || sel.lastIndex >= list.length) return;
  const [a, b] = [Math.min(sel.lastIndex, toIndex), Math.max(sel.lastIndex, toIndex)];
  for (let i = a; i <= b; i++) toggleCard(list[i], true);
}

function clearSelection() {
  sel.items.clear();
  sel.filterMode = false;
  sel.lastIndex = -1;
  cards().forEach((c) => setCardSelected(c, false));
  updateSelectionBar();
}

function selectionPayload() {
  if (sel.filterMode) return { filter: currentFilter() };
  return { items: [...sel.items.values()] };
}

function selectionCount() { return sel.filterMode ? totalCount() : sel.items.size; }

// ---------- grid interaction ----------

grid.addEventListener("click", (e) => {
  const del = e.target.closest("[data-delfolder]");
  if (del) {
    e.preventDefault();
    e.stopPropagation();
    deleteFolderDialog(del.dataset.delfolder);
    return;
  }
  const folderLink = e.target.closest("[data-folder]");
  if (folderLink) {
    e.preventDefault();
    $("#folderInput").value = folderLink.dataset.folder;
    htmx.trigger(filters, "change");
    return;
  }
  if (e.target.id === "newFolderBtn") {
    newFolder();
    return;
  }
  if (e.target.id === "cleanFoldersBtn") {
    cleanEmptyFolders();
    return;
  }
  if (e.target.classList.contains("backToGrid")) {
    refreshGrid();
    return;
  }
  if (e.target.id === "trashAllDupes") {
    trashAllDupes();
    return;
  }

  const card = e.target.closest(".card");
  if (!card) return;
  if (card.classList.contains("dupecard")) return; // dupes use their checkboxes only

  const list = cards();
  const idx = list.indexOf(card);
  if (e.shiftKey && sel.lastIndex >= 0) {
    selectRange(idx);
    sel.lastIndex = idx;
    updateSelectionBar();
    document.getSelection()?.removeAllRanges();
    return;
  }
  if (e.target.classList.contains("check") || e.ctrlKey || e.metaKey) {
    toggleCard(card);
    sel.lastIndex = idx;
    updateSelectionBar();
    return;
  }
  openLightbox(card);
});

// Thumbs that fail to generate (e.g. corrupted file) degrade to an icon tile.
grid.addEventListener("error", (e) => {
  const img = e.target;
  if (img.tagName === "IMG" && img.classList.contains("thumb")) {
    const div = document.createElement("div");
    div.className = "icon";
    div.textContent = img.dataset.fallback || "FILE";
    img.replaceWith(div);
  }
}, true);

// Animate cards as fragments land; re-apply selection to re-rendered cards.
document.body.addEventListener("htmx:afterSwap", (e) => {
  if (!grid.contains(e.detail.target) && e.detail.target !== grid) return;
  const fresh = e.detail.target === grid ? cards() : [...e.detail.target.querySelectorAll?.(".card") ?? []];
  cards().forEach((c) => { if (sel.items.has(keyOf(c))) setCardSelected(c, true); });
  if (window.anime && fresh.length) {
    anime({ targets: fresh.slice(0, 60), opacity: [0, 1], translateY: [10, 0], delay: anime.stagger(8), duration: 220, easing: "easeOutQuad" });
  }
  updateSelectionBar();
});

// ---------- header wiring ----------

document.querySelectorAll("#tabs button").forEach((b) => {
  b.onclick = () => {
    document.querySelectorAll("#tabs button").forEach((x) => x.classList.remove("active"));
    b.classList.add("active");
    $("#typeInput").value = b.dataset.type;
    $("#folderInput").value = "";
    clearSelection();
    htmx.trigger(filters, "change");
  };
});

const sizeRange = $("#sizeRange");
function applyCardSize(px) {
  grid.style.setProperty("--card-size", px + "px");
  sizeRange.value = px;
  localStorage.setItem("cc.cardSize", px);
}
applyCardSize(parseInt(localStorage.getItem("cc.cardSize"), 10) || 440);
sizeRange.oninput = (e) => applyCardSize(parseInt(e.target.value, 10));

$("#emptyTrash").onclick = async () => {
  const f = currentFilter();
  const scope = `${f.type || "all types"} / ${f.env || "all environments"}`;
  if (!(await confirmModal("Empty trash", `Permanently delete everything in the .trash of ${scope}? This cannot be undone.`, "Empty trash"))) return;
  try {
    const res = await api("/api/empty-trash", { env: f.env, type: f.type });
    toast(res.files ? `Freed ${fmtSize(res.bytes)} (${res.files} file(s))` : "Trash was already empty");
  } catch (e) { toast(e.message, true); }
};

function fmtSize(n) {
  if (n > 1 << 30) return (n / (1 << 30)).toFixed(1) + " GB";
  if (n > 1 << 20) return (n / (1 << 20)).toFixed(1) + " MB";
  if (n > 1024) return (n / 1024).toFixed(0) + " KB";
  return n + " B";
}

// ---------- selection bar actions ----------

$("#selectAllBtn").onclick = () => {
  sel.filterMode = !sel.filterMode;
  if (sel.filterMode) {
    sel.items.clear();
    cards().forEach((c) => setCardSelected(c, true));
  } else {
    cards().forEach((c) => setCardSelected(c, false));
  }
  updateSelectionBar();
};
$("#clearSel").onclick = clearSelection;

async function bulkAction(label, fn) {
  const n = selectionCount();
  if (!n) return;
  try {
    await fn();
    toast(`${label}: ${n} item(s)`);
    refreshGrid();
  } catch (e) { toast(e.message, true); }
}

$("#copyBtn").onclick = () => bulkAction("Copied", () =>
  api("/api/transfer", { ...selectionPayload(), dst_env: $("#dstEnv").value, move: false }));
$("#moveBtn").onclick = () => bulkAction("Moved", () =>
  api("/api/transfer", { ...selectionPayload(), dst_env: $("#dstEnv").value, move: true }));
$("#deleteBtn").onclick = async () => {
  const n = selectionCount();
  if (!(await confirmModal("Delete", `Move ${n} item(s) to .trash?`, "Delete"))) return;
  bulkAction("Trashed", () => api("/api/delete", selectionPayload()));
};
$("#folderBtn").onclick = async () => {
  const folder = await promptModal("Move to folder", "Relative to the asset root; empty = root.", currentFilter().folder, "Move");
  if (folder === null) return;
  bulkAction("Moved to " + (folder || "root"), () =>
    api("/api/move-folder", { ...selectionPayload(), folder }));
};
$("#renameBtn").onclick = async () => {
  if (sel.filterMode || sel.items.size !== 1) { toast("Rename works on exactly one selected item", true); return; }
  const item = [...sel.items.values()][0];
  const name = await promptModal("Rename", item.path, item.path.split("/").pop(), "Rename");
  if (!name) return;
  bulkAction("Renamed", () => api("/api/rename", { items: [item], name }));
};

async function newFolder() {
  const f = currentFilter();
  const name = await promptModal("New folder", f.folder ? `Inside ${f.folder}` : "At the asset root", "", "Create");
  if (!name) return;
  const folder = f.folder ? `${f.folder}/${name}` : name;
  try {
    await api("/api/mkdir", { env: f.env, type: f.type, folder });
    toast(`Created ${folder}`);
    refreshGrid();
  } catch (e) { toast(e.message, true); }
}

async function deleteFolderDialog(folder) {
  const f = currentFilter();
  const choice = await showModal({
    title: `Delete folder “${folder}”`,
    message: "Keep contents moves everything up one level. Delete everything moves the folder and its contents to .trash.",
    buttons: [
      { label: "Cancel", value: null },
      { label: "Keep contents", value: "keep", primary: true },
      { label: "Delete everything", value: "trash", danger: true },
    ],
  });
  if (!choice) return;
  try {
    await api("/api/delete-folder", { env: f.env, type: f.type, folder, keep_contents: choice === "keep" });
    toast(choice === "keep" ? `Removed ${folder}, contents moved up` : `Moved ${folder} to .trash`);
    refreshGrid();
  } catch (e) { toast(e.message, true); }
}

async function cleanEmptyFolders() {
  const f = currentFilter();
  try {
    const dry = await api("/api/clean-folders", { env: f.env, type: f.type, dry: true });
    if (!dry.folders.length) {
      toast("No empty folders found");
      return;
    }
    const names = dry.folders.slice(0, 8).map((x) => x.folder).join(", ");
    const more = dry.folders.length > 8 ? `, … (+${dry.folders.length - 8} more)` : "";
    const ok = await confirmModal(
      "Remove empty folders",
      `Remove ${dry.folders.length} empty folder(s)? ${names}${more}`,
      "Remove", true);
    if (!ok) return;
    const res = await api("/api/clean-folders", { env: f.env, type: f.type, dry: false });
    toast(`Removed ${res.folders.length} empty folder(s)`);
    refreshGrid();
  } catch (e) { toast(e.message, true); }
}

async function trashAllDupes() {
  const copies = [...grid.querySelectorAll(".dupecard:not(.keeper)")].map(refOf);
  if (!copies.length) return;
  if (!(await confirmModal("Trash duplicates", `Move ${copies.length} duplicate copies to .trash (keeping the newest of each group)?`, "Trash copies"))) return;
  try {
    await api("/api/delete", { items: copies });
    toast(`Trashed ${copies.length} duplicate(s)`);
    refreshGrid();
  } catch (e) { toast(e.message, true); }
}

// ---------- lightbox ----------

function openLightbox(card) {
  const lb = $("#lightbox");
  lb.classList.remove("hidden");
  if (window.anime) anime({ targets: "#lightboxBody", scale: [0.96, 1], opacity: [0, 1], duration: 180, easing: "easeOutQuad" });
  const q = new URLSearchParams(refOf(card));
  htmx.ajax("GET", `/detail?${q}`, { target: "#lbContent", swap: "innerHTML" });
}

function closeLightbox() {
  $("#lightbox").classList.add("hidden");
  const v = $("#lbContent video, #lbContent audio");
  if (v) v.pause();
}

$("#lbClose").onclick = closeLightbox;
$("#lightbox").addEventListener("click", (e) => { if (e.target.id === "lightbox") closeLightbox(); });
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    if (!modalEl.classList.contains("hidden")) { closeModal(null); return; }
    closeLightbox();
    clearSelection();
  }
  if (e.key === "a" && (e.ctrlKey || e.metaKey) && !e.target.matches("input,textarea,select")) {
    e.preventDefault();
    cards().forEach((c) => toggleCard(c, true));
    updateSelectionBar();
  }
});

// Detail-panel actions (elements arrive via htmx swap → event delegation).
$("#lbContent").addEventListener("click", async (e) => {
  const b = e.target;
  const ref = b.dataset?.path ? { env: b.dataset.env, type: b.dataset.type, path: b.dataset.path } : null;
  try {
    if (b.id === "lbRename" && ref) {
      const name = $("#renameInput").value.trim();
      if (!name) return;
      await api("/api/rename", { items: [ref], name });
      toast("Renamed");
      closeLightbox();
      refreshGrid();
    } else if (b.id === "lbCopy" && ref) {
      await api("/api/transfer", { items: [ref], dst_env: $("#lbDstEnv").value });
      toast(`Copied to ${$("#lbDstEnv").value}`);
    } else if (b.id === "saveWf" && ref) {
      const res = await api("/api/extract-workflow", { items: [ref], dst_env: $("#wfDstEnv").value });
      toast(`Workflow saved as ${res.saved}`);
    }
  } catch (err) { toast(err.message, true); }
});
