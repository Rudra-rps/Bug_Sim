/* ═══════════════════════════════════════════════════
   BUGOFF — APP ENGINE (CLEAN)
   Connects to Go API, falls back to mock data.
   ═══════════════════════════════════════════════════ */

// ── API BASE ──
const API = ''; // Same origin — Go server serves both

// ── SEVERITY LABELS ──
const SEV_LABELS = ['', 'LOW', 'LOW', 'MEDIUM', 'MEDIUM', 'HIGH', 'HIGH', 'HIGH', 'CRITICAL', 'CRITICAL', 'CRITICAL'];
function sevLabel(sev) {
  if (sev >= 9) return 'critical';
  if (sev >= 6) return 'high';
  if (sev >= 4) return 'medium';
  return 'low';
}

// ── MOCK DATA (used when Go server isn't running) ──
const MOCK_BUGS = [
  { ID: 1,  Title: 'SQL Injection in Auth Endpoint',       Severity: 10, Age: 45, BountyValue: 5000, Reproductions: 12, EstimatedFixHours: 8,  Source: 'OWASP BLT', URL: '#', Priority: 0 },
  { ID: 2,  Title: 'XSS via User Profile Field',           Severity: 9,  Age: 30, BountyValue: 4200, Reproductions: 8,  EstimatedFixHours: 6,  Source: 'OWASP BLT', URL: '#', Priority: 0 },
  { ID: 3,  Title: 'IDOR in Admin Panel API',              Severity: 8,  Age: 22, BountyValue: 3500, Reproductions: 6,  EstimatedFixHours: 5,  Source: 'GitHub', URL: '#', Priority: 0 },
  { ID: 4,  Title: 'CSRF on Password Reset Flow',          Severity: 7,  Age: 18, BountyValue: 2800, Reproductions: 5,  EstimatedFixHours: 4,  Source: 'OWASP BLT', URL: '#', Priority: 0 },
  { ID: 5,  Title: 'Open Redirect on Login Callback',      Severity: 7,  Age: 60, BountyValue: 2200, Reproductions: 9,  EstimatedFixHours: 3,  Source: 'GitHub', URL: '#', Priority: 0 },
  { ID: 6,  Title: 'Rate Limiting Bypass on OTP',          Severity: 6,  Age: 14, BountyValue: 1800, Reproductions: 4,  EstimatedFixHours: 4,  Source: 'OWASP BLT', URL: '#', Priority: 0 },
  { ID: 7,  Title: 'Sensitive Data in Error Logs',         Severity: 5,  Age: 90, BountyValue: 1500, Reproductions: 7,  EstimatedFixHours: 3,  Source: 'GitHub', URL: '#', Priority: 0 },
  { ID: 8,  Title: 'CORS Misconfiguration',                Severity: 5,  Age: 10, BountyValue: 1200, Reproductions: 3,  EstimatedFixHours: 2,  Source: 'OWASP BLT', URL: '#', Priority: 0 },
  { ID: 9,  Title: 'Missing CSP Header',                   Severity: 3,  Age: 120, BountyValue: 800, Reproductions: 11, EstimatedFixHours: 2,  Source: 'GitHub', URL: '#', Priority: 0 },
  { ID: 10, Title: 'Clickjacking on Settings Page',        Severity: 3,  Age: 5,  BountyValue: 500,  Reproductions: 2,  EstimatedFixHours: 1,  Source: 'OWASP BLT', URL: '#', Priority: 0 },
  { ID: 11, Title: 'Subdomain Takeover Risk',              Severity: 9,  Age: 35, BountyValue: 4800, Reproductions: 3,  EstimatedFixHours: 7,  Source: 'GitHub', URL: '#', Priority: 0 },
  { ID: 12, Title: 'SSRF via Image Proxy',                 Severity: 8,  Age: 28, BountyValue: 3200, Reproductions: 5,  EstimatedFixHours: 6,  Source: 'OWASP BLT', URL: '#', Priority: 0 },
  { ID: 13, Title: 'JWT None Algorithm Accepted',          Severity: 10, Age: 7,  BountyValue: 4500, Reproductions: 6,  EstimatedFixHours: 3,  Source: 'OWASP BLT', URL: '#', Priority: 0 },
  { ID: 14, Title: 'Path Traversal in File Upload',        Severity: 8,  Age: 42, BountyValue: 3000, Reproductions: 4,  EstimatedFixHours: 5,  Source: 'GitHub', URL: '#', Priority: 0 },
  { ID: 15, Title: 'Verbose Server Headers Exposed',       Severity: 2,  Age: 200, BountyValue: 400, Reproductions: 15, EstimatedFixHours: 1,  Source: 'GitHub', URL: '#', Priority: 0 },
];

// ── SCORING (mirrors Go formula) ──
function scoreBug(bug) {
  // Normalize to 0-100
  const maxSev = 10, maxBounty = 5000, maxRepro = 15, maxAge = 200;
  const normSev   = Math.min(bug.Severity / maxSev, 1) * 100;
  const normBount = Math.min(bug.BountyValue / maxBounty, 1) * 100;
  const normRepro = Math.min(bug.Reproductions / maxRepro, 1) * 100;
  const normAge   = Math.min(bug.Age / maxAge, 1) * 100;

  return (normSev * 0.4) + (normBount * 0.3) + (normRepro * 0.2) + (normAge * 0.1);
}

function scoreBugs(bugs) {
  return bugs.map(b => ({ ...b, Priority: scoreBug(b) }))
             .sort((a, b) => b.Priority - a.Priority);
}

// ── STATE ──
let allBugs = [];
let selectedBugId = null;
let usingMock = false;

// ── API CALLS ──
async function fetchBugs() {
  try {
    const res = await fetch(`${API}/api/bugs`);
    if (!res.ok) throw new Error('API error');
    const data = await res.json();
    if (data && data.length > 0) {
      allBugs = data.sort((a, b) => (b.Priority || 0) - (a.Priority || 0));
      usingMock = false;
      return;
    }
  } catch (e) {
    console.log('Go server not available, using mock data');
  }

  // Fallback to mock
  allBugs = scoreBugs(MOCK_BUGS);
  usingMock = true;
}

async function apiFix(id) {
  try {
    const res = await fetch(`${API}/api/fix/${id}`, { method: 'POST' });
    if (res.ok) return true;
  } catch (e) {}
  return false;
}

async function apiSchedule(hours) {
  try {
    const res = await fetch(`${API}/api/schedule?hours=${hours}`);
    if (res.ok) return await res.json();
  } catch (e) {}
  return null;
}

async function apiCompare() {
  try {
    const res = await fetch(`${API}/api/compare`);
    if (res.ok) return await res.json();
  } catch (e) {}
  return null;
}

// ── LOCAL SCHEDULING (fallback) ──
function localSchedule(bugs, budget) {
  const sorted = [...bugs].sort((a, b) => {
    const ratioA = a.Priority / a.EstimatedFixHours;
    const ratioB = b.Priority / b.EstimatedFixHours;
    return ratioB - ratioA;
  });

  const picked = [];
  let remaining = budget;
  let totalPriority = 0;

  for (const bug of sorted) {
    if (bug.EstimatedFixHours <= remaining) {
      picked.push(bug);
      remaining -= bug.EstimatedFixHours;
      totalPriority += bug.Priority;
    }
  }

  return {
    bugs: picked,
    hoursUsed: budget - remaining,
    hoursRemaining: remaining,
    totalPriority: totalPriority,
  };
}

// ── LOCAL BRUTE FORCE (for comparison — small datasets only) ──
function bruteForceSchedule(bugs, budget) {
  const n = bugs.length;
  let bestPriority = 0;
  let bestSet = [];

  // Limit to first 15 bugs for performance
  const subset = bugs.slice(0, Math.min(n, 15));
  const m = subset.length;
  const combos = 1 << m;

  for (let mask = 0; mask < combos; mask++) {
    let hours = 0;
    let prio = 0;
    const set = [];

    for (let i = 0; i < m; i++) {
      if (mask & (1 << i)) {
        hours += subset[i].EstimatedFixHours;
        if (hours > budget) break;
        prio += subset[i].Priority;
        set.push(subset[i]);
      }
    }

    if (hours <= budget && prio > bestPriority) {
      bestPriority = prio;
      bestSet = [...set];
    }
  }

  return {
    bugs: bestSet,
    totalPriority: bestPriority,
    hoursUsed: bestSet.reduce((s, b) => s + b.EstimatedFixHours, 0),
  };
}

// ── RENDER FUNCTIONS ──
function renderStats() {
  document.getElementById('stat-total').textContent = allBugs.length;
  const crits = allBugs.filter(b => b.Severity >= 9).length;
  document.getElementById('stat-critical').textContent = crits;

  // Integrity estimate: based on how many bugs are fixed vs total
  const maxBugs = MOCK_BUGS.length;
  const pct = Math.max(0, Math.round((1 - allBugs.length / maxBugs) * 100));
  document.getElementById('stat-integrity').textContent = pct + '%';
}

function renderBugList() {
  const container = document.getElementById('bug-list');
  container.innerHTML = '';

  const k = parseInt(document.getElementById('top-k-select').value);
  const bugs = allBugs.slice(0, k);
  const maxPriority = bugs.length > 0 ? bugs[0].Priority : 1;

  bugs.forEach((bug, i) => {
    const sev = sevLabel(bug.Severity);
    const card = document.createElement('div');
    card.className = `bug-card sev-${sev}`;
    if (bug.ID === selectedBugId) card.classList.add('selected');
    card.dataset.bugId = bug.ID;

    const bountyStr = bug.BountyValue >= 1000 ? `$${(bug.BountyValue/1000).toFixed(1)}K` : `$${bug.BountyValue}`;
    const barWidth = Math.round((bug.Priority / maxPriority) * 100);

    let barColor;
    if (sev === 'critical') barColor = 'var(--red)';
    else if (sev === 'high') barColor = 'var(--coral)';
    else if (sev === 'medium') barColor = 'var(--amber)';
    else barColor = 'var(--cyan)';

    card.innerHTML = `
      <div class="bug-rank">#${i + 1}</div>
      <div class="bug-info">
        <div class="bug-title">${bug.Title}</div>
        <div class="bug-meta">
          <span class="bug-tag tag-severity sev-${sev}">${sev.toUpperCase()}</span>
          <span class="bug-tag tag-bounty">${bountyStr}</span>
          <span class="bug-tag tag-hours">${bug.EstimatedFixHours}h</span>
          <span class="bug-tag tag-repro">${bug.Reproductions} repro</span>
        </div>
        <div class="bug-source">${bug.Source}${bug.URL && bug.URL !== '#' ? ' · ' + bug.URL : ''}</div>
      </div>
      <div class="bug-stats">
        <div class="bug-score">${bug.Priority.toFixed(1)}</div>
        <div class="bug-score-label">priority</div>
        <div class="score-bar">
          <div class="score-bar-fill" style="width:${barWidth}%;background:${barColor}"></div>
        </div>
      </div>
    `;

    card.addEventListener('click', () => selectBug(bug.ID));
    container.appendChild(card);
  });
}

function selectBug(id) {
  selectedBugId = id;
  renderBugList();
  renderDeployTarget();
}

function renderDeployTarget() {
  const container = document.getElementById('deploy-target');
  const btn = document.getElementById('btn-deploy');
  const bug = allBugs.find(b => b.ID === selectedBugId);

  if (!bug) {
    container.innerHTML = '<p class="deploy-empty">Click a bug from the queue to select it</p>';
    btn.disabled = true;
    return;
  }

  const sev = sevLabel(bug.Severity);
  const bountyStr = bug.BountyValue >= 1000 ? `$${(bug.BountyValue/1000).toFixed(1)}K` : `$${bug.BountyValue}`;

  container.innerHTML = `
    <div class="deploy-bug-info">
      <div class="deploy-bug-title">${bug.Title}</div>
      <div class="deploy-bug-meta">
        <span class="bug-tag tag-severity sev-${sev}">${sev.toUpperCase()}</span>
        <span class="bug-tag tag-bounty">${bountyStr}</span>
        <span class="bug-tag tag-hours">${bug.EstimatedFixHours}h to fix</span>
      </div>
    </div>
  `;
  btn.disabled = false;
}

// ── DEPLOY FIX ──
async function deployFix() {
  if (!selectedBugId) return;

  const bug = allBugs.find(b => b.ID === selectedBugId);
  if (!bug) return;

  // Try API first
  if (!usingMock) {
    await apiFix(selectedBugId);
  }

  // Remove locally
  allBugs = allBugs.filter(b => b.ID !== selectedBugId);
  selectedBugId = null;

  showFlash(`Bug fixed: ${bug.Title}`, 'success');

  renderStats();
  renderBugList();
  renderDeployTarget();
}

// ── SCHEDULE ──
async function runSchedule() {
  const hours = parseInt(document.getElementById('budget-input').value) || 8;
  const container = document.getElementById('schedule-list');
  const summary = document.getElementById('schedule-summary');

  let result;

  if (!usingMock) {
    const apiResult = await apiSchedule(hours);
    if (apiResult) {
      // Assume API returns an array of bugs
      result = {
        bugs: apiResult,
        hoursUsed: apiResult.reduce((s, b) => s + b.EstimatedFixHours, 0),
        hoursRemaining: hours - apiResult.reduce((s, b) => s + b.EstimatedFixHours, 0),
        totalPriority: apiResult.reduce((s, b) => s + (b.Priority || 0), 0),
      };
    }
  }

  if (!result) {
    result = localSchedule(allBugs, hours);
  }

  container.innerHTML = '';

  if (result.bugs.length === 0) {
    container.innerHTML = '<p class="schedule-empty">No bugs fit in the budget</p>';
    summary.classList.add('hidden');
    return;
  }

  result.bugs.forEach((bug, i) => {
    const item = document.createElement('div');
    item.className = 'schedule-item';
    item.innerHTML = `
      <span class="sched-num">${i + 1}.</span>
      <span class="sched-title">${bug.Title}</span>
      <span class="sched-cost">${bug.EstimatedFixHours}h</span>
    `;
    container.appendChild(item);
  });

  summary.classList.remove('hidden');
  summary.innerHTML = `
    <div class="summary-row"><span>Bugs to fix:</span><span>${result.bugs.length}</span></div>
    <div class="summary-row"><span>Hours used:</span><span>${result.hoursUsed}h / ${hours}h</span></div>
    <div class="summary-row"><span>Total priority:</span><span>${result.totalPriority.toFixed(1)}</span></div>
  `;
}

// ── COMPARISON ──
async function runComparison() {
  const container = document.getElementById('compare-result');
  const hours = parseInt(document.getElementById('budget-input').value) || 8;

  let greedyResult, optimalResult;

  if (!usingMock) {
    const apiResult = await apiCompare();
    if (apiResult && apiResult.greedy && apiResult.optimal) {
      greedyResult = apiResult.greedy;
      optimalResult = apiResult.optimal;
    }
  }

  if (!greedyResult) {
    const g = localSchedule(allBugs, hours);
    greedyResult = { totalPriority: g.totalPriority, count: g.bugs.length, hoursUsed: g.hoursUsed };

    const o = bruteForceSchedule(allBugs, hours);
    optimalResult = { totalPriority: o.totalPriority, count: o.bugs.length, hoursUsed: o.hoursUsed };
  }

  const gp = greedyResult.totalPriority || 0;
  const op = optimalResult.totalPriority || 0;
  const max = Math.max(gp, op, 1);
  const efficiency = op > 0 ? ((gp / op) * 100).toFixed(1) : '100.0';

  container.classList.remove('hidden');
  container.innerHTML = `
    <div class="compare-row">
      <span class="compare-label">Greedy</span>
      <span class="compare-value compare-greedy">${gp.toFixed(1)}</span>
    </div>
    <div class="compare-row">
      <span class="compare-label">Optimal (brute-force)</span>
      <span class="compare-value compare-optimal">${op.toFixed(1)}</span>
    </div>
    <div class="compare-bar">
      <div class="compare-bar-optimal" style="width:${(op/max)*100}%"></div>
      <div class="compare-bar-greedy" style="width:${(gp/max)*100}%"></div>
    </div>
    <div class="compare-verdict">Greedy achieves ${efficiency}% of optimal</div>
  `;
}

// ── FLASH ──
function showFlash(msg, type = 'success') {
  const el = document.getElementById('flash');
  el.textContent = msg;
  el.className = `flash ${type}`;
  setTimeout(() => {
    el.classList.add('hidden');
  }, 2500);
}

// ── INIT ──
async function init() {
  await fetchBugs();
  renderStats();
  renderBugList();
  renderDeployTarget();

  document.getElementById('btn-deploy').addEventListener('click', deployFix);
  document.getElementById('btn-schedule').addEventListener('click', runSchedule);
  document.getElementById('btn-compare').addEventListener('click', runComparison);
  document.getElementById('top-k-select').addEventListener('change', renderBugList);
}

document.addEventListener('DOMContentLoaded', init);
