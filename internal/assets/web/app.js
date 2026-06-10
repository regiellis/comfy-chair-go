"use strict";

const state = {
  type: "output",
  env: "",
  q: "",
  sort: "date",
  offset: 0,
  limit: 120,
  total: 0,
  items: [],
  selected: new Map(), // key -> item
  envs: [],
};

const $ = (sel) => document.querySelector(sel);
const grid = $("#grid");

function keyOf(it) { return `${it.env}|${it.type}|${it.path}`; }

function fmtSize(n) {
  if (n > 1024 * 1024 * 1024) return (n / 1024 / 1024 / 1024).toFixed(1) + " GB";
  if (n > 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + " MB";
  if (n > 1024) return (n / 1024).toFixed(0) + " KB";
  return n + " B";
}
function fmtDate(s) { return new Date(s).toLocaleString(); }

function fileURL(it) {
  return `/api/file?env=${encodeURIComponent(it.env)}&type=${encodeURIComponent(it.type)}&path=${encodeURIComponent(it.path)}`;
}
function thumbURL(it) {
  return `/api/thumb?env=${encodeURIComponent(it.env)}&type=${encodeURIComponent(it.type)}&path=${encodeURIComponent(it.path)}`;
}

async function api(path, opts) {
  const r = await fetch(path, opts);
  let json = {};
  try { json = await r.json(); } catch { /* non-JSON */ }
  if (!r.ok || json.success === false) {
    throw new Error(json.error || (json.errors || []).join("; ") || `HTTP ${r.status}`);
  }
  return json;
}

function toast(msg, isError) {
  const el = document.createElement("div");
  el.className = "toast" + (isError ? " error" : "");
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), isError ? 5000 : 2500);
}

async function loadEnvs() {
  const { envs } = await api("/api/envs");
  state.envs = envs;
  const envSelect = $("#envSelect");
  const dstEnv = $("#dstEnv");
  for (const e of envs) {
    envSelect.insertAdjacentHTML("beforeend", `<option value="${e.name}">${e.name}</option>`);
    dstEnv.insertAdjacentHTML("beforeend", `<option value="${e.name}">→ ${e.name}</option>`);
  }
}

async function loadAssets(append) {
  if (!append) { state.offset = 0; state.items = []; }
  $("#status").textContent = "Loading…";
  const params = new URLSearchParams({
    type: state.type, env: state.env, q: state.q, sort: state.sort,
    offset: state.offset, limit: state.limit,
  });
  try {
    const { items, total } = await api(`/api/assets?${params}`);
    state.total = total;
    state.items = append ? state.items.concat(items) : items;
    render();
  } catch (e) {
    $("#status").textContent = "Error: " + e.message;
  }
}

function cardHTML(it) {
  const k = keyOf(it);
  const sel = state.selected.has(k) ? " selected" : "";
  let media;
  if (it.kind === "image") {
    media = `<img class="thumb" loading="lazy" src="${thumbURL(it)}" alt="" data-fallback="🖼">`;
  } else if (it.kind === "video") {
    media = `<img class="thumb" loading="lazy" src="${thumbURL(it)}" alt="" data-fallback="🎬"><span class="playbadge">▶</span>`;
  } else {
    media = `<div class="icon">${it.kind === "audio" ? "🎵" : "🧩"}</div>`;
  }
  return `<div class="card${sel}" data-key="${k}">
    <input type="checkbox" class="check" ${sel ? "checked" : ""}>
    ${media}
    <div class="meta">
      <div class="name">${it.path}</div>
      <div class="sub"><span class="badge">${it.env}</span><span>${fmtSize(it.size)}</span><span>${fmtDate(it.mtime).split(",")[0]}</span></div>
    </div>
  </div>`;
}

function render() {
  grid.innerHTML = state.items.map(cardHTML).join("");
  $("#status").textContent = state.items.length
    ? `${state.items.length} of ${state.total} item(s)`
    : "No assets found.";
  $("#loadMore").classList.toggle("hidden", state.items.length >= state.total);
  updateSelectionBar();
}

function updateSelectionBar() {
  const n = state.selected.size;
  $("#selectionBar").classList.toggle("hidden", n === 0);
  $("#selCount").textContent = `${n} selected`;
}

function toggleSelect(key) {
  const it = state.items.find((i) => keyOf(i) === key);
  if (!it) return;
  if (state.selected.has(key)) state.selected.delete(key);
  else state.selected.set(key, it);
  const card = grid.querySelector(`[data-key="${CSS.escape(key)}"]`);
  if (card) {
    card.classList.toggle("selected", state.selected.has(key));
    card.querySelector(".check").checked = state.selected.has(key);
  }
  updateSelectionBar();
}

// --- lightbox ---

async function openLightbox(it) {
  const lb = $("#lightbox");
  const media = $("#lbMedia");
  const info = $("#lbInfo");
  lb.classList.remove("hidden");

  if (it.kind === "image") {
    media.innerHTML = `<img src="${fileURL(it)}" alt="">`;
  } else if (it.kind === "video") {
    media.innerHTML = `<video src="${fileURL(it)}" controls autoplay loop></video>`;
  } else if (it.kind === "audio") {
    media.innerHTML = `<audio src="${fileURL(it)}" controls autoplay></audio>`;
  } else {
    media.innerHTML = `<pre>Loading…</pre>`;
    try {
      const r = await fetch(fileURL(it));
      const data = await r.json();
      media.querySelector("pre").textContent = JSON.stringify(data, null, 2);
    } catch {
      media.querySelector("pre").textContent = "(could not load JSON)";
    }
  }

  const envOptions = state.envs.map((e) => `<option value="${e.name}">${e.name}</option>`).join("");
  let html = `<h2>${it.path}</h2>
    <div class="kv">${it.env} · ${it.type} · ${fmtSize(it.size)}<br>${fmtDate(it.mtime)}</div>
    <div class="actions">
      <a href="${fileURL(it)}" download="${it.name}"><button style="width:100%">Download</button></a>`;
  if (it.kind === "workflow") {
    html += `<div style="display:flex;gap:6px"><select id="lbDstEnv" style="flex:1">${envOptions}</select>
      <button id="lbCopyWf">Copy to env</button></div>`;
  }
  html += `</div><div id="lbMeta"></div>`;
  info.innerHTML = html;

  const copyBtn = $("#lbCopyWf");
  if (copyBtn) {
    copyBtn.onclick = async () => {
      try {
        await api("/api/transfer", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ items: [refOf(it)], dst_env: $("#lbDstEnv").value }),
        });
        toast(`Copied to ${$("#lbDstEnv").value}`);
      } catch (e) { toast(e.message, true); }
    };
  }

  // Embedded ComfyUI metadata for PNGs
  if (it.kind === "image" && it.name.toLowerCase().endsWith(".png")) {
    try {
      const { meta } = await api(`/api/meta?env=${encodeURIComponent(it.env)}&type=${encodeURIComponent(it.type)}&path=${encodeURIComponent(it.path)}`);
      const metaEl = $("#lbMeta");
      if (meta.workflow) {
        metaEl.insertAdjacentHTML("beforeend", `
          <h2 style="margin-top:14px">Embedded workflow</h2>
          <div style="display:flex;gap:6px;margin:6px 0">
            <select id="wfDstEnv" style="flex:1">${envOptions}</select>
            <button id="saveWf">Save to env</button>
          </div>`);
        $("#saveWf").onclick = async () => {
          try {
            const res = await api("/api/extract-workflow", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ items: [refOf(it)], dst_env: $("#wfDstEnv").value }),
            });
            toast(`Workflow saved as ${res.saved}`);
          } catch (e) { toast(e.message, true); }
        };
      }
      if (meta.prompt) {
        let pretty = meta.prompt;
        try { pretty = JSON.stringify(JSON.parse(meta.prompt), null, 2); } catch { /* keep raw */ }
        metaEl.insertAdjacentHTML("beforeend", `<h2 style="margin-top:14px">Prompt</h2><pre>${pretty.replace(/</g, "&lt;")}</pre>`);
      }
      if (!meta.workflow && !meta.prompt) {
        metaEl.insertAdjacentHTML("beforeend", `<div class="kv">No embedded ComfyUI metadata.</div>`);
      }
    } catch { /* meta is best-effort */ }
  }
}

function refOf(it) { return { env: it.env, type: it.type, path: it.path }; }

// --- bulk actions ---

async function bulk(action, move) {
  const items = [...state.selected.values()].map(refOf);
  const dst = $("#dstEnv").value;
  try {
    if (action === "delete") {
      if (!confirm(`Move ${items.length} item(s) to .trash?`)) return;
      await api("/api/delete", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ items }),
      });
      toast(`${items.length} item(s) moved to .trash`);
    } else {
      await api("/api/transfer", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ items, dst_env: dst, move }),
      });
      toast(`${items.length} item(s) ${move ? "moved" : "copied"} to ${dst}`);
    }
    state.selected.clear();
    loadAssets(false);
  } catch (e) { toast(e.message, true); }
}

// --- wiring ---

document.querySelectorAll("#tabs button").forEach((b) => {
  b.onclick = () => {
    document.querySelectorAll("#tabs button").forEach((x) => x.classList.remove("active"));
    b.classList.add("active");
    state.type = b.dataset.type;
    state.selected.clear();
    loadAssets(false);
  };
});
$("#envSelect").onchange = (e) => { state.env = e.target.value; loadAssets(false); };
$("#sortSelect").onchange = (e) => { state.sort = e.target.value; loadAssets(false); };
let searchTimer;
$("#search").oninput = (e) => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => { state.q = e.target.value; loadAssets(false); }, 250);
};
$("#refresh").onclick = () => loadAssets(false);
$("#loadMore").onclick = () => { state.offset = state.items.length; loadAssets(true); };

// Thumbs that fail to generate (e.g. no ffmpeg) degrade to an icon tile.
grid.addEventListener("error", (e) => {
  const img = e.target;
  if (img.tagName === "IMG" && img.classList.contains("thumb")) {
    const div = document.createElement("div");
    div.className = "icon";
    div.textContent = img.dataset.fallback || "🧩";
    img.replaceWith(div);
  }
}, true);

$("#emptyTrash").onclick = async () => {
  const scopeEnv = state.env || "all environments";
  const tabName = document.querySelector("#tabs button.active").textContent.toLowerCase();
  if (!confirm(`Permanently delete everything in the .trash of ${tabName} for ${scopeEnv}? This cannot be undone.`)) return;
  try {
    const res = await api("/api/empty-trash", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ env: state.env, type: state.type }),
    });
    toast(res.files ? `Freed ${fmtSize(res.bytes)} (${res.files} file(s))` : "Trash was already empty");
  } catch (e) { toast(e.message, true); }
};

grid.addEventListener("click", (e) => {
  const card = e.target.closest(".card");
  if (!card) return;
  const key = card.dataset.key;
  if (e.target.classList.contains("check") || e.ctrlKey || e.metaKey) {
    toggleSelect(key);
    return;
  }
  const it = state.items.find((i) => keyOf(i) === key);
  if (it) openLightbox(it);
});

$("#copyBtn").onclick = () => bulk("transfer", false);
$("#moveBtn").onclick = () => bulk("transfer", true);
$("#deleteBtn").onclick = () => bulk("delete");
$("#clearSel").onclick = () => {
  state.selected.clear();
  grid.querySelectorAll(".card.selected").forEach((c) => {
    c.classList.remove("selected");
    c.querySelector(".check").checked = false;
  });
  updateSelectionBar();
};
$("#lbClose").onclick = () => $("#lightbox").classList.add("hidden");
$("#lightbox").addEventListener("click", (e) => {
  if (e.target.id === "lightbox") $("#lightbox").classList.add("hidden");
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") $("#lightbox").classList.add("hidden");
});

(async function init() {
  try {
    await loadEnvs();
    await loadAssets(false);
  } catch (e) {
    $("#status").textContent = "Error: " + e.message;
  }
})();
