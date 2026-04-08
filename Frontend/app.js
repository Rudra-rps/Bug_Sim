const API = "";

let allBugs = [];
let selectedBugID = null;
let initialBugCount = 0;
let fixedCount = 0;

function toNumber(value, fallback = 0) {
  const n = Number(value);
  return Number.isFinite(n) ? n : fallback;
}

function normalizeBug(raw) {
  const id = toNumber(raw.id ?? raw.ID, 0);
  const title = String(raw.title ?? raw.Title ?? "Untitled bug");
  const severity = Math.max(1, Math.min(5, Math.round(toNumber(raw.severity ?? raw.Severity, 3))));
  const age = Math.max(0, Math.round(toNumber(raw.age ?? raw.Age, 0)));
  const bountyValue = Math.max(0, Math.round(toNumber(raw.bountyValue ?? raw.BountyValue, 0)));
  const reproductions = Math.max(1, Math.round(toNumber(raw.reproductions ?? raw.Reproductions, 1)));
  const estimatedFixHours = Math.max(1, Math.round(toNumber(raw.estimatedFixHours ?? raw.EstimatedFixHours, 1)));
  const source = String(raw.source ?? raw.Source ?? "github");
  const url = String(raw.url ?? raw.URL ?? "#");
  const priority = toNumber(raw.priority ?? raw.Priority, 0);

  return {
    id,
    title,
    severity,
    age,
    bountyValue,
    reproductions,
    estimatedFixHours,
    source,
    url,
    priority,
  };
}

function normalizeBugs(data) {
  if (!Array.isArray(data)) {
    return [];
  }
  return data
    .map(normalizeBug)
    .filter((bug) => bug.id > 0)
    .sort((a, b) => b.priority - a.priority);
}

function severityLabel(severity) {
  if (severity >= 5) return "critical";
  if (severity >= 4) return "high";
  if (severity >= 3) return "medium";
  return "low";
}

async function fetchBugs(refresh = false) {
  const suffix = refresh ? "?refresh=1" : "";
  const response = await fetch(`${API}/api/bugs${suffix}`);
  if (!response.ok) {
    throw new Error(`Failed to fetch bugs (${response.status})`);
  }
  const data = await response.json();
  allBugs = normalizeBugs(data);
  if (initialBugCount === 0) {
    initialBugCount = allBugs.length;
  }
}

async function apiFix(id) {
  const response = await fetch(`${API}/api/fix/${id}`, { method: "POST" });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Failed to fix bug ${id}`);
  }
  return response.json();
}

async function apiSchedule(hours) {
  const response = await fetch(`${API}/api/schedule?hours=${hours}`);
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || "Failed to fetch schedule");
  }
  const data = await response.json();
  return normalizeBugs(data);
}

async function apiCompare(hours) {
  const response = await fetch(`${API}/api/compare?hours=${hours}`);
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || "Failed to compare schedules");
  }
  return response.json();
}

function localSchedule(bugs, budgetHours) {
  const sorted = [...bugs].sort((a, b) => {
    const ratioA = a.priority / a.estimatedFixHours;
    const ratioB = b.priority / b.estimatedFixHours;
    if (ratioA === ratioB) return b.priority - a.priority;
    return ratioB - ratioA;
  });

  const selected = [];
  let usedHours = 0;
  let totalPriority = 0;
  for (const bug of sorted) {
    if (usedHours + bug.estimatedFixHours > budgetHours) continue;
    selected.push(bug);
    usedHours += bug.estimatedFixHours;
    totalPriority += bug.priority;
  }

  return { bugs: selected, usedHours, totalPriority };
}

function localBruteForce(bugs, budgetHours, cap = 15) {
  const candidates = [...bugs].sort((a, b) => b.priority - a.priority).slice(0, cap);
  const n = candidates.length;
  let bestMask = 0;
  let bestPriority = -1;
  let bestHours = 0;

  const limit = 1 << n;
  for (let mask = 0; mask < limit; mask++) {
    let usedHours = 0;
    let totalPriority = 0;
    let valid = true;

    for (let i = 0; i < n; i++) {
      if ((mask & (1 << i)) === 0) continue;
      usedHours += candidates[i].estimatedFixHours;
      if (usedHours > budgetHours) {
        valid = false;
        break;
      }
      totalPriority += candidates[i].priority;
    }

    if (!valid) continue;
    if (totalPriority > bestPriority || (Math.abs(totalPriority - bestPriority) < 0.0001 && usedHours < bestHours)) {
      bestPriority = totalPriority;
      bestHours = usedHours;
      bestMask = mask;
    }
  }

  const selected = [];
  for (let i = 0; i < n; i++) {
    if ((bestMask & (1 << i)) !== 0) selected.push(candidates[i]);
  }
  return {
    bugs: selected,
    usedHours: bestHours,
    totalPriority: Math.max(bestPriority, 0),
  };
}

function renderStats() {
  const totalEl = document.getElementById("stat-total");
  const criticalEl = document.getElementById("stat-critical");
  const integrityEl = document.getElementById("stat-integrity");

  totalEl.textContent = String(allBugs.length);
  criticalEl.textContent = String(allBugs.filter((bug) => bug.severity >= 5).length);

  if (initialBugCount <= 0) {
    integrityEl.textContent = "-";
    return;
  }
  const pct = Math.round((fixedCount / initialBugCount) * 100);
  integrityEl.textContent = `${pct}%`;
}

function renderBugList() {
  const list = document.getElementById("bug-list");
  const topKRaw = Number(document.getElementById("top-k-select").value || 5);
  const maxCount = topKRaw >= 15 ? allBugs.length : topKRaw;
  const bugs = allBugs.slice(0, maxCount);
  const maxPriority = bugs.length > 0 ? bugs[0].priority : 1;

  list.innerHTML = "";
  for (let i = 0; i < bugs.length; i++) {
    const bug = bugs[i];
    const severityClass = severityLabel(bug.severity);
    const card = document.createElement("div");
    card.className = `bug-card sev-${severityClass}`;
    if (bug.id === selectedBugID) card.classList.add("selected");

    const bountyText = bug.bountyValue >= 1000
      ? `$${(bug.bountyValue / 1000).toFixed(1)}K`
      : `$${bug.bountyValue}`;
    const scoreBarWidth = Math.round((bug.priority / maxPriority) * 100);
    const color = severityClass === "critical"
      ? "var(--red)"
      : severityClass === "high"
        ? "var(--coral)"
        : severityClass === "medium"
          ? "var(--amber)"
          : "var(--cyan)";

    card.innerHTML = `
      <div class="bug-rank">#${i + 1}</div>
      <div class="bug-info">
        <div class="bug-title">${escapeHTML(bug.title)}</div>
        <div class="bug-meta">
          <span class="bug-tag tag-severity sev-${severityClass}">${severityClass.toUpperCase()}</span>
          <span class="bug-tag tag-bounty">${bountyText}</span>
          <span class="bug-tag tag-hours">${bug.estimatedFixHours}h</span>
          <span class="bug-tag tag-repro">${bug.reproductions} repro</span>
        </div>
        <div class="bug-source">${escapeHTML(bug.source)}</div>
      </div>
      <div class="bug-stats">
        <div class="bug-score">${bug.priority.toFixed(1)}</div>
        <div class="bug-score-label">priority</div>
        <div class="score-bar">
          <div class="score-bar-fill" style="width:${scoreBarWidth}%;background:${color}"></div>
        </div>
      </div>
    `;

    card.addEventListener("click", () => selectBug(bug.id));
    list.appendChild(card);
  }
}

function renderDeployTarget() {
  const target = document.getElementById("deploy-target");
  const button = document.getElementById("btn-deploy");
  const bug = allBugs.find((item) => item.id === selectedBugID);

  if (!bug) {
    target.innerHTML = "<p class='deploy-empty'>Select a bug to deploy</p>";
    button.disabled = true;
    return;
  }

  const severityClass = severityLabel(bug.severity);
  const bountyText = bug.bountyValue >= 1000
    ? `$${(bug.bountyValue / 1000).toFixed(1)}K`
    : `$${bug.bountyValue}`;
  const safeURL = bug.url && bug.url !== "#" ? bug.url : "";
  const urlHTML = safeURL
    ? `<a href="${safeURL}" target="_blank" rel="noopener noreferrer">View Issue</a>`
    : "";

  target.innerHTML = `
    <div class="deploy-bug-info">
      <div class="deploy-bug-title">${escapeHTML(bug.title)}</div>
      <div class="deploy-bug-meta">
        <span class="bug-tag tag-severity sev-${severityClass}">${severityClass.toUpperCase()}</span>
        <span class="bug-tag tag-bounty">${bountyText}</span>
        <span class="bug-tag tag-hours">${bug.estimatedFixHours}h to fix</span>
      </div>
      <div class="deploy-bug-link">${urlHTML}</div>
    </div>
  `;
  button.disabled = false;
}

function selectBug(id) {
  selectedBugID = id;
  renderBugList();
  renderDeployTarget();
}

async function deployFix() {
  if (!selectedBugID) return;
  const bug = allBugs.find((item) => item.id === selectedBugID);
  if (!bug) return;

  try {
    await apiFix(selectedBugID);
  } catch (error) {
    showFlash(`Fix failed: ${String(error.message || error)}`, "error");
    return;
  }

  allBugs = allBugs.filter((item) => item.id !== selectedBugID);
  selectedBugID = null;
  fixedCount += 1;

  showFlash(`Bug fixed: ${bug.title}`, "success");
  renderStats();
  renderBugList();
  renderDeployTarget();
}

async function runSchedule() {
  const hours = Math.max(1, Number(document.getElementById("budget-input").value || 8));
  const list = document.getElementById("schedule-list");
  const summary = document.getElementById("schedule-summary");

  let scheduledBugs = [];
  try {
    scheduledBugs = await apiSchedule(hours);
  } catch (error) {
    showFlash(`Schedule API failed, using local fallback: ${String(error.message || error)}`, "error");
    scheduledBugs = localSchedule(allBugs, hours).bugs;
  }

  list.innerHTML = "";
  if (scheduledBugs.length === 0) {
    list.innerHTML = "<p class='schedule-empty'>No bugs fit in the selected budget</p>";
    summary.classList.add("hidden");
    return;
  }

  let usedHours = 0;
  let totalPriority = 0;
  scheduledBugs.forEach((bug, index) => {
    usedHours += bug.estimatedFixHours;
    totalPriority += bug.priority;
    const item = document.createElement("div");
    item.className = "schedule-item";
    item.innerHTML = `
      <span class="sched-num">${index + 1}.</span>
      <span class="sched-title">${escapeHTML(bug.title)}</span>
      <span class="sched-cost">${bug.estimatedFixHours}h</span>
    `;
    list.appendChild(item);
  });

  summary.classList.remove("hidden");
  summary.innerHTML = `
    <div class="summary-row"><span>Bugs to fix:</span><span>${scheduledBugs.length}</span></div>
    <div class="summary-row"><span>Hours used:</span><span>${usedHours}h / ${hours}h</span></div>
    <div class="summary-row"><span>Total priority:</span><span>${totalPriority.toFixed(1)}</span></div>
  `;
}

async function runComparison() {
  const resultBox = document.getElementById("compare-result");
  const hours = Math.max(1, Number(document.getElementById("budget-input").value || 8));

  let greedyPriority = 0;
  let optimalPriority = 0;
  try {
    const apiResult = await apiCompare(hours);
    greedyPriority = toNumber(apiResult?.greedy?.totalPriority, 0);
    optimalPriority = toNumber(apiResult?.optimal?.totalPriority, 0);
  } catch (error) {
    showFlash(`Compare API failed, using local fallback: ${String(error.message || error)}`, "error");
    const greedy = localSchedule(allBugs, hours);
    const optimal = localBruteForce(allBugs, hours);
    greedyPriority = greedy.totalPriority;
    optimalPriority = optimal.totalPriority;
  }

  const maxPriority = Math.max(1, greedyPriority, optimalPriority);
  const greedyPct = (greedyPriority / maxPriority) * 100;
  const optimalPct = (optimalPriority / maxPriority) * 100;
  const efficiency = optimalPriority > 0 ? ((greedyPriority / optimalPriority) * 100).toFixed(1) : "100.0";

  resultBox.classList.remove("hidden");
  resultBox.innerHTML = `
    <div class="compare-row">
      <span class="compare-label">Greedy</span>
      <span class="compare-value compare-greedy">${greedyPriority.toFixed(1)}</span>
    </div>
    <div class="compare-row">
      <span class="compare-label">Optimal (brute-force)</span>
      <span class="compare-value compare-optimal">${optimalPriority.toFixed(1)}</span>
    </div>
    <div class="compare-bar">
      <div class="compare-bar-optimal" style="width:${optimalPct}%"></div>
      <div class="compare-bar-greedy" style="width:${greedyPct}%"></div>
    </div>
    <div class="compare-verdict">Greedy achieves ${efficiency}% of optimal</div>
  `;
}

function showFlash(message, type = "success") {
  const flash = document.getElementById("flash");
  flash.textContent = message;
  flash.className = `flash ${type}`;
  setTimeout(() => flash.classList.add("hidden"), 2600);
}

function escapeHTML(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

async function init() {
  try {
    await fetchBugs(true);
  } catch (error) {
    showFlash(`Could not load live bugs: ${String(error.message || error)}`, "error");
    allBugs = [];
  }

  renderStats();
  renderBugList();
  renderDeployTarget();

  document.getElementById("btn-deploy").addEventListener("click", deployFix);
  document.getElementById("btn-schedule").addEventListener("click", runSchedule);
  document.getElementById("btn-compare").addEventListener("click", runComparison);
  document.getElementById("top-k-select").addEventListener("change", renderBugList);
}

document.addEventListener("DOMContentLoaded", init);
