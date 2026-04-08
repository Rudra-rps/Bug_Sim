const API_BASE = "";

// --------------------------
// State
// --------------------------
const state = {
  bugs: [],
  selectedId: null,
  sortMode: "priority_desc",
  limit: 20,
  loadingBugs: false,
  fetchError: "",
  online: false,
  lastSync: null,
  explain: {
    loading: false,
    error: "",
    summary: "",
    detail: "",
    bugId: null,
    seq: 0,
  },
  schedule: {
    loading: false,
    error: "",
    hours: 8,
    bugs: [],
  },
  compare: {
    loading: false,
    error: "",
    data: null,
  },
};

const elements = {
  statusDot: document.getElementById("statusDot"),
  statusText: document.getElementById("statusText"),
  lastSync: document.getElementById("lastSync"),
  btnRefresh: document.getElementById("btnRefresh"),
  globalAlert: document.getElementById("globalAlert"),

  kpiTotal: document.getElementById("kpiTotal"),
  kpiCritical: document.getElementById("kpiCritical"),
  kpiAvg: document.getElementById("kpiAvg"),

  sortSelect: document.getElementById("sortSelect"),
  limitSelect: document.getElementById("limitSelect"),
  queueState: document.getElementById("queueState"),
  queueList: document.getElementById("queueList"),

  detailFix: document.getElementById("detailFix"),
  detailState: document.getElementById("detailState"),
  detailTitle: document.getElementById("detailTitle"),
  detailMeta: document.getElementById("detailMeta"),
  sourceLink: document.getElementById("sourceLink"),

  breakSeverity: document.getElementById("breakSeverity"),
  breakBounty: document.getElementById("breakBounty"),
  breakRepro: document.getElementById("breakRepro"),
  breakAge: document.getElementById("breakAge"),
  labelSeverity: document.getElementById("labelSeverity"),
  labelBounty: document.getElementById("labelBounty"),
  labelRepro: document.getElementById("labelRepro"),
  labelAge: document.getElementById("labelAge"),

  explainState: document.getElementById("explainState"),
  explainSummary: document.getElementById("explainSummary"),
  explainDetail: document.getElementById("explainDetail"),

  scheduleForm: document.getElementById("scheduleForm"),
  hoursInput: document.getElementById("hoursInput"),
  scheduleState: document.getElementById("scheduleState"),
  scheduleList: document.getElementById("scheduleList"),
  scheduleTotals: document.getElementById("scheduleTotals"),

  compareButton: document.getElementById("compareButton"),
  compareState: document.getElementById("compareState"),
  compareGreedy: document.getElementById("compareGreedy"),
  compareOptimal: document.getElementById("compareOptimal"),
  compareBarGreedy: document.getElementById("compareBarGreedy"),
  compareBarOptimal: document.getElementById("compareBarOptimal"),
  compareEfficiency: document.getElementById("compareEfficiency"),

  toast: document.getElementById("toast"),
};

// --------------------------
// API Client
// --------------------------
const api = {
  async getBugs(forceRefresh) {
    const query = forceRefresh ? "?refresh=1" : "";
    return requestJSON(`/api/bugs${query}`);
  },

  async getTop(k) {
    return requestJSON(`/api/top?k=${encodeURIComponent(k)}`);
  },

  async fix(id) {
    return requestJSON(`/api/fix/${encodeURIComponent(id)}`, { method: "POST" });
  },

  async schedule(hours) {
    return requestJSON(`/api/schedule?hours=${encodeURIComponent(hours)}`);
  },

  async compare(hours) {
    return requestJSON(`/api/compare?hours=${encodeURIComponent(hours)}`);
  },

  async explain(id) {
    return requestJSON(`/api/explain/${encodeURIComponent(id)}`);
  },
};

async function requestJSON(path, options = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      Accept: "application/json",
    },
    ...options,
  });

  const raw = await response.text();
  let parsed = null;
  if (raw) {
    try {
      parsed = JSON.parse(raw);
    } catch {
      parsed = null;
    }
  }

  if (!response.ok) {
    const message = parsed?.error || raw || `Request failed (${response.status})`;
    throw new Error(message);
  }

  return parsed;
}

// --------------------------
// Data helpers
// --------------------------
function normalizeBug(raw) {
  const bug = {
    id: toInt(raw?.id ?? raw?.ID, 0),
    title: String(raw?.title ?? raw?.Title ?? "Untitled bug"),
    severity: clamp(toInt(raw?.severity ?? raw?.Severity, 3), 1, 5),
    age: Math.max(0, toInt(raw?.age ?? raw?.Age, 0)),
    bountyValue: Math.max(0, toInt(raw?.bountyValue ?? raw?.BountyValue, 0)),
    reproductions: Math.max(0, toInt(raw?.reproductions ?? raw?.Reproductions, 0)),
    estimatedFixHours: Math.max(1, toInt(raw?.estimatedFixHours ?? raw?.EstimatedFixHours, 1)),
    source: String(raw?.source ?? raw?.Source ?? "unknown"),
    url: String(raw?.url ?? raw?.URL ?? ""),
    priority: toFloat(raw?.priority ?? raw?.Priority, 0),
    priorityBreakdown: {
      severity: clamp(toFloat(raw?.priorityBreakdown?.severity ?? raw?.PriorityBreakdown?.severity, 0), 0, 100),
      bountyValue: clamp(toFloat(raw?.priorityBreakdown?.bountyValue ?? raw?.PriorityBreakdown?.bountyValue, 0), 0, 100),
      reproductions: clamp(toFloat(raw?.priorityBreakdown?.reproductions ?? raw?.PriorityBreakdown?.reproductions, 0), 0, 100),
      age: clamp(toFloat(raw?.priorityBreakdown?.age ?? raw?.PriorityBreakdown?.age, 0), 0, 100),
    },
  };
  return bug;
}

function normalizeBugList(list) {
  if (!Array.isArray(list)) {
    return [];
  }
  return list.map(normalizeBug).filter((bug) => bug.id > 0);
}

function selectedBug() {
  return state.bugs.find((bug) => bug.id === state.selectedId) || null;
}

function visibleBugs() {
  const copy = [...state.bugs];
  copy.sort((a, b) => {
    if (state.sortMode === "priority_asc") {
      return a.priority - b.priority;
    }
    return b.priority - a.priority;
  });
  return copy.slice(0, Math.max(1, state.limit));
}

function severityName(severity) {
  switch (severity) {
    case 5: return "Critical";
    case 4: return "High";
    case 3: return "Medium";
    case 2: return "Low";
    default: return "Info";
  }
}

function toInt(value, fallback) {
  const n = Number.parseInt(value, 10);
  return Number.isFinite(n) ? n : fallback;
}

function toFloat(value, fallback) {
  const n = Number.parseFloat(value);
  return Number.isFinite(n) ? n : fallback;
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function formatMoney(amount) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(amount);
}

function formatTime(date) {
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

// --------------------------
// Render
// --------------------------
function renderAll() {
  renderStatus();
  renderGlobalAlert();
  renderKpis();
  renderQueue();
  renderDetail();
  renderSchedule();
  renderCompare();
}

function renderStatus() {
  elements.statusDot.classList.toggle("status-online", state.online);
  elements.statusDot.classList.toggle("status-offline", !state.online);
  elements.statusText.textContent = state.online ? "Live" : "Offline";
  elements.lastSync.textContent = state.lastSync
    ? `Last sync: ${formatTime(state.lastSync)}`
    : "Last sync: never";
}

function renderGlobalAlert() {
  if (!state.fetchError) {
    elements.globalAlert.classList.add("hidden");
    elements.globalAlert.textContent = "";
    return;
  }
  elements.globalAlert.classList.remove("hidden");
  elements.globalAlert.textContent = state.fetchError;
}

function renderKpis() {
  const total = state.bugs.length;
  const critical = state.bugs.filter((bug) => bug.severity >= 5).length;
  const avg = total === 0
    ? 0
    : state.bugs.reduce((sum, bug) => sum + bug.priority, 0) / total;

  elements.kpiTotal.textContent = String(total);
  elements.kpiCritical.textContent = String(critical);
  elements.kpiAvg.textContent = avg.toFixed(1);
}

function renderQueue() {
  const bugs = visibleBugs();
  elements.queueList.innerHTML = "";

  if (state.loadingBugs) {
    elements.queueState.textContent = "Loading live issues...";
    return;
  }

  if (bugs.length === 0) {
    elements.queueState.textContent = "No bugs available.";
    return;
  }

  elements.queueState.textContent = `Showing ${bugs.length} issues`;

  bugs.forEach((bug, index) => {
    const item = document.createElement("button");
    item.type = "button";
    item.className = `queue-item severity-${bug.severity}`;
    item.setAttribute("role", "option");
    item.setAttribute("aria-selected", bug.id === state.selectedId ? "true" : "false");
    item.dataset.id = String(bug.id);
    item.style.animationDelay = `${index * 28}ms`;
    if (bug.id === state.selectedId) {
      item.classList.add("selected");
    }

    const title = document.createElement("p");
    title.className = "queue-title";
    title.textContent = bug.title;

    const rank = document.createElement("span");
    rank.className = "queue-rank";
    rank.textContent = `#${index + 1}`;

    const meta = document.createElement("div");
    meta.className = "queue-meta";
    meta.append(
      chip(`S${bug.severity} ${severityName(bug.severity)}`, `chip-severity-${bug.severity}`),
      chip(formatMoney(bug.bountyValue)),
      chip(`${bug.estimatedFixHours}h`),
      chip(`${bug.reproductions} repro`)
    );

    const footer = document.createElement("div");
    footer.className = "queue-footer";

    const source = document.createElement("span");
    source.textContent = `${bug.source} | Age ${bug.age}d`;

    const priority = document.createElement("span");
    priority.className = "queue-priority";
    priority.textContent = bug.priority.toFixed(1);

    footer.append(source, priority);
    item.append(rank, title, meta, footer);
    elements.queueList.appendChild(item);
  });
}

function chip(text, className) {
  const span = document.createElement("span");
  span.className = `chip ${className || ""}`.trim();
  span.textContent = text;
  return span;
}

function renderDetail() {
  const bug = selectedBug();
  const hasBug = !!bug;

  elements.detailFix.disabled = !hasBug;
  elements.detailTitle.textContent = hasBug ? bug.title : "";
  elements.detailMeta.innerHTML = "";

  if (!hasBug) {
    elements.detailState.textContent = "Select a bug to inspect.";
    elements.sourceLink.classList.add("hidden");
    clearBreakdown();
    renderExplain();
    return;
  }

  elements.detailState.textContent = `ID ${bug.id} | Priority ${bug.priority.toFixed(1)}`;

  addMeta("Severity", `${bug.severity} (${severityName(bug.severity)})`);
  addMeta("Bounty", formatMoney(bug.bountyValue));
  addMeta("Age", `${bug.age} days`);
  addMeta("Reproductions", String(bug.reproductions));
  addMeta("Est. Fix Time", `${bug.estimatedFixHours} hours`);
  addMeta("Source", bug.source);

  if (bug.url) {
    elements.sourceLink.href = bug.url;
    elements.sourceLink.classList.remove("hidden");
  } else {
    elements.sourceLink.classList.add("hidden");
  }

  applyBreakdown(bug.priorityBreakdown);
  renderExplain();
}

function addMeta(label, value) {
  const dt = document.createElement("dt");
  dt.textContent = label;
  const dd = document.createElement("dd");
  dd.textContent = value;
  elements.detailMeta.append(dt, dd);
}

function clearBreakdown() {
  applyBreakdown({
    severity: 0,
    bountyValue: 0,
    reproductions: 0,
    age: 0,
  });
}

function applyBreakdown(breakdown) {
  setMeter(elements.breakSeverity, elements.labelSeverity, breakdown.severity);
  setMeter(elements.breakBounty, elements.labelBounty, breakdown.bountyValue);
  setMeter(elements.breakRepro, elements.labelRepro, breakdown.reproductions);
  setMeter(elements.breakAge, elements.labelAge, breakdown.age);
}

function setMeter(bar, label, value) {
  const safe = clamp(toFloat(value, 0), 0, 100);
  bar.style.width = `${safe}%`;
  label.textContent = safe.toFixed(0);
}

function renderExplain() {
  if (!state.selectedId) {
    elements.explainState.textContent = "No explanation loaded.";
    elements.explainSummary.textContent = "";
    elements.explainDetail.textContent = "";
    return;
  }

  if (state.explain.loading) {
    elements.explainState.textContent = "Loading explanation...";
    elements.explainSummary.textContent = "";
    elements.explainDetail.textContent = "";
    return;
  }

  if (state.explain.error) {
    elements.explainState.textContent = state.explain.error;
    elements.explainSummary.textContent = "";
    elements.explainDetail.textContent = "";
    return;
  }

  elements.explainState.textContent = "";
  elements.explainSummary.textContent = state.explain.summary || "";
  elements.explainDetail.textContent = state.explain.detail || "";
}

function renderSchedule() {
  elements.scheduleList.innerHTML = "";
  elements.scheduleTotals.classList.add("hidden");
  elements.scheduleTotals.textContent = "";

  if (state.schedule.loading) {
    elements.scheduleState.textContent = "Generating schedule...";
    return;
  }

  if (state.schedule.error) {
    elements.scheduleState.textContent = state.schedule.error;
    return;
  }

  if (state.schedule.bugs.length === 0) {
    elements.scheduleState.textContent = "No schedule generated yet.";
    return;
  }

  elements.scheduleState.textContent = `Selected ${state.schedule.bugs.length} bugs`;

  let usedHours = 0;
  let totalPriority = 0;
  state.schedule.bugs.forEach((bug) => {
    usedHours += bug.estimatedFixHours;
    totalPriority += bug.priority;

    const li = document.createElement("li");
    li.className = "schedule-item";

    const title = document.createElement("strong");
    title.textContent = bug.title;

    const hours = document.createElement("span");
    hours.textContent = `${bug.estimatedFixHours}h`;

    const score = document.createElement("span");
    score.textContent = bug.priority.toFixed(1);

    li.append(title, hours, score);
    elements.scheduleList.appendChild(li);
  });

  elements.scheduleTotals.classList.remove("hidden");
  elements.scheduleTotals.innerHTML = "";
  elements.scheduleTotals.append(
    totalsRow("Budget", `${state.schedule.hours}h`),
    totalsRow("Used", `${usedHours}h`),
    totalsRow("Total Priority", totalPriority.toFixed(1))
  );
}

function totalsRow(label, value) {
  const row = document.createElement("div");
  row.textContent = `${label}: ${value}`;
  return row;
}

function renderCompare() {
  if (state.compare.loading) {
    elements.compareState.textContent = "Running comparison...";
    return;
  }

  if (state.compare.error) {
    elements.compareState.textContent = state.compare.error;
    return;
  }

  if (!state.compare.data) {
    elements.compareState.textContent = "Comparison not run yet.";
    elements.compareGreedy.textContent = "0.0";
    elements.compareOptimal.textContent = "0.0";
    elements.compareBarGreedy.style.width = "0%";
    elements.compareBarOptimal.style.width = "0%";
    elements.compareEfficiency.textContent = "";
    return;
  }

  const greedy = toFloat(state.compare.data?.greedy?.totalPriority, 0);
  const optimal = toFloat(state.compare.data?.optimal?.totalPriority, 0);
  const max = Math.max(greedy, optimal, 1);
  const greedyPct = (greedy / max) * 100;
  const optimalPct = (optimal / max) * 100;
  const efficiency = optimal > 0 ? (greedy / optimal) * 100 : 100;

  elements.compareState.textContent = "Comparison complete.";
  elements.compareGreedy.textContent = greedy.toFixed(1);
  elements.compareOptimal.textContent = optimal.toFixed(1);
  elements.compareBarGreedy.style.width = `${greedyPct}%`;
  elements.compareBarOptimal.style.width = `${optimalPct}%`;
  elements.compareEfficiency.textContent = `Greedy reaches ${efficiency.toFixed(1)}% of optimal`;
}

// --------------------------
// Actions
// --------------------------
async function loadBugs(forceRefresh) {
  state.loadingBugs = true;
  state.fetchError = "";
  renderAll();

  try {
    const response = await api.getBugs(forceRefresh);
    state.bugs = normalizeBugList(response);
    state.online = true;
    state.lastSync = new Date();

    if (!state.selectedId || !state.bugs.some((bug) => bug.id === state.selectedId)) {
      state.selectedId = state.bugs.length ? state.bugs[0].id : null;
    }
  } catch (error) {
    state.online = false;
    state.fetchError = `Live fetch failed: ${error.message}`;
  } finally {
    state.loadingBugs = false;
    renderAll();
  }

  if (state.selectedId) {
    await loadExplain(state.selectedId);
  }
}

async function selectBugById(id) {
  if (!state.bugs.some((bug) => bug.id === id)) {
    return;
  }
  state.selectedId = id;
  renderAll();
  await loadExplain(id);
}

async function loadExplain(bugId) {
  state.explain.seq += 1;
  const seq = state.explain.seq;
  state.explain.loading = true;
  state.explain.error = "";
  state.explain.summary = "";
  state.explain.detail = "";
  state.explain.bugId = bugId;
  renderDetail();

  try {
    const response = await api.explain(bugId);
    if (seq !== state.explain.seq) return;
    state.explain.summary = String(response?.summary || "");
    state.explain.detail = String(response?.detail || "");
  } catch (error) {
    if (seq !== state.explain.seq) return;
    state.explain.error = `Explain failed: ${error.message}`;
  } finally {
    if (seq !== state.explain.seq) return;
    state.explain.loading = false;
    renderDetail();
  }
}

async function deployFixSelected() {
  const bug = selectedBug();
  if (!bug) {
    return;
  }

  const previousBugs = [...state.bugs];
  const previousSelected = state.selectedId;

  state.bugs = state.bugs.filter((item) => item.id !== bug.id);
  state.selectedId = state.bugs.length ? state.bugs[0].id : null;
  showToast(`Deploying fix for #${bug.id}...`);
  renderAll();

  try {
    await api.fix(bug.id);
    showToast(`Bug #${bug.id} fixed.`);
    await loadBugs(false);
  } catch (error) {
    state.bugs = previousBugs;
    state.selectedId = previousSelected;
    state.fetchError = `Fix failed: ${error.message}`;
    showToast(`Fix failed for #${bug.id}.`);
    renderAll();
  }
}

async function runSchedule(hours) {
  state.schedule.loading = true;
  state.schedule.error = "";
  state.schedule.hours = hours;
  state.schedule.bugs = [];
  renderSchedule();

  try {
    const response = await api.schedule(hours);
    state.schedule.bugs = normalizeBugList(response);
  } catch (error) {
    state.schedule.error = `Schedule failed: ${error.message}`;
  } finally {
    state.schedule.loading = false;
    renderSchedule();
  }
}

async function runCompare(hours) {
  state.compare.loading = true;
  state.compare.error = "";
  state.compare.data = null;
  renderCompare();

  try {
    const response = await api.compare(hours);
    state.compare.data = response;
  } catch (error) {
    state.compare.error = `Compare failed: ${error.message}`;
  } finally {
    state.compare.loading = false;
    renderCompare();
  }
}

// --------------------------
// UI helpers
// --------------------------
let toastTimer = null;
function showToast(message) {
  elements.toast.textContent = message;
  elements.toast.classList.remove("hidden");
  if (toastTimer) {
    clearTimeout(toastTimer);
  }
  toastTimer = setTimeout(() => {
    elements.toast.classList.add("hidden");
  }, 2200);
}

// --------------------------
// Events
// --------------------------
function bindEvents() {
  elements.btnRefresh.addEventListener("click", () => {
    loadBugs(true);
  });

  elements.sortSelect.addEventListener("change", () => {
    state.sortMode = elements.sortSelect.value;
    renderQueue();
  });

  elements.limitSelect.addEventListener("change", () => {
    state.limit = toInt(elements.limitSelect.value, 20);
    renderQueue();
  });

  elements.queueList.addEventListener("click", (event) => {
    const target = event.target.closest("button.queue-item");
    if (!target) return;
    const id = toInt(target.dataset.id, 0);
    if (id > 0) {
      selectBugById(id);
    }
  });

  elements.detailFix.addEventListener("click", () => {
    deployFixSelected();
  });

  elements.scheduleForm.addEventListener("submit", (event) => {
    event.preventDefault();
    const hours = clamp(toInt(elements.hoursInput.value, 8), 1, 80);
    elements.hoursInput.value = String(hours);
    runSchedule(hours);
  });

  elements.compareButton.addEventListener("click", () => {
    const hours = clamp(toInt(elements.hoursInput.value, 8), 1, 80);
    runCompare(hours);
  });
}

// --------------------------
// Bootstrap
// --------------------------
async function init() {
  bindEvents();
  renderAll();
  await loadBugs(true);
}

document.addEventListener("DOMContentLoaded", init);
