# Bug Bounty Priority Engine Plan

## Goal
Build a hackathon-ready Go application that fetches real bug bounty issues, scores them using a weighted priority formula, stores them in a max-heap for fast retrieval, schedules fixes within limited developer hours, and presents the results in an interactive Clash Royale-inspired dashboard.

The app should be locally runnable, demo-safe, and strong on both DSA depth and presentation quality.

## Core Product Summary
The engine solves a real maintainer problem: too many issues, not enough time. It prioritizes bugs using a normalized weighted formula, keeps the highest-priority bugs accessible in `O(log n)` via a max-heap, and uses scheduling logic to recommend the best set of bugs to fix under a limited time budget.

The primary data source is OWASP BLT. A GitHub issues fallback is included to protect the demo from API instability. AI is used as an explanation layer, not as the ranking engine, so the technical story remains grounded in DSA and deterministic logic.

## Architecture
Project structure:

```text
bug-bounty-engine/
  main.go
  heap/
    bugheap.go
  api/
    fetch.go
  scorer/
    score.go
  scheduler/
    greedy.go
  server/
    server.go
  frontend/
    index.html
    style.css
```

Key components:

- `api/`: fetches and normalizes bugs from BLT or GitHub
- `scorer/`: computes normalized weighted priority score
- `heap/`: stores scored bugs in a max-heap
- `scheduler/`: picks best bugs under a developer-hour constraint
- `server/`: exposes REST endpoints and serves frontend assets
- `frontend/`: interactive dashboard with Royale-style presentation

## Data Model
Primary bug model:

```go
type Bug struct {
    ID                 int
    Title              string
    Severity           int
    Age                int
    BountyValue        int
    Reproductions      int
    EstimatedFixHours  int
    Source             string
    URL                string
    Priority           float64
}
```

Notes:

- `EstimatedFixHours` is required for scheduling.
- If the source does not provide fix time, derive it heuristically.
- `Source` and `URL` are included for traceability and UI linking.

## Priority Formula
All ranking inputs are normalized to a 0-100 scale before weighting.

```text
Priority =
  (Severity * 0.4) +
  (BountyValue * 0.3) +
  (Reproductions * 0.2) +
  (Age * 0.1)
```

Design intent:

- Severity matters most.
- Bounty value strongly influences urgency.
- Reproductions improve confidence and impact.
- Age ensures older unresolved issues still rise.

## Heap Strategy
Use Go's `container/heap` with a custom `Less` function to behave as a max-heap:

```go
func (h BugHeap) Less(i, j int) bool {
    return h[i].Priority > h[j].Priority
}
```

Heap responsibilities:

- insert scored bugs
- retrieve top-priority bugs efficiently
- rebalance after simulated fixes
- support top-K extraction for dashboard display

## Scheduler Strategy
Use two scheduling approaches:

### Greedy
Pick bugs by highest `priority / estimated_fix_hours` ratio until the hour budget is exhausted.

Use case:
- fast and demo-friendly
- intuitive explanation for judges
- works well for larger inputs

### Brute Force
Run exhaustive search only on small subsets to compare against greedy and show algorithmic rigor.

Use case:
- validates greedy quality
- creates a compelling "compare" feature
- must be capped to avoid combinatorial blow-up

## API Surface
Required endpoints:

- `GET /api/bugs`
  - fetch, normalize, score, and return all bugs
- `GET /api/top?k=5`
  - return top-K bugs from heap order
- `POST /api/fix/:id`
  - remove a bug and rebalance the heap
- `GET /api/schedule?hours=8`
  - run greedy scheduler for a developer-hour budget
- `GET /api/compare`
  - compare greedy output with brute-force optimal output

Optional AI endpoint:

- `GET /api/explain?id=123`
  - returns a human-readable explanation of why a bug ranks highly

## Frontend Requirements
Must-have features:

- top-K bug list
- fix simulation
- score breakdown bars
- scheduler output
- compare table for greedy vs brute force

Nice-to-have features:

- heap tree visualization
- card-hand metaphor for top bugs
- arena-style animated header

Theme direction:

- each bug appears as a Clash Royale-style troop card
- fix time is shown like elixir cost
- fix action feels like deploying a counter-card
- schedule output is framed as a battle plan

## AI Layer
AI should not decide ranking. It should explain the deterministic output.

Use AI for:

- short bug summaries
- "why this bug matters" explanations
- battle-plan narration for the selected schedule
- judge-facing one-line insights for the current top bug

Fallback behavior:

- if AI is unavailable, generate template-based explanations from score data
- demo must remain fully functional without AI

Positioning:
"The engine decides. AI explains."

## 5-Hour Delivery Timeline

### Hour 1
- initialize module and folders
- define `Bug` struct
- implement scorer
- implement heap
- verify with seeded sample data

Deliverable:
Core DSA layer works with local sample bugs.

### Hour 2
- implement BLT fetcher
- add GitHub fallback fetcher
- normalize external issue data into `Bug`
- add in-memory caching

Deliverable:
Real bug data flows into the engine.

### Hour 3
- implement greedy scheduler
- implement capped brute-force comparator
- add REST endpoints for bugs, top-K, fix, schedule, compare

Deliverable:
Backend becomes demo-ready.

### Hour 4
- build dashboard UI
- render bug cards, score bars, and scheduler recommendations
- wire fix simulation and live refresh

Deliverable:
Interactive frontend runs locally.

### Hour 5
- add Royale styling and lightweight animations
- add AI explanation layer
- prepare README, demo script, and fallback flow

Deliverable:
Polished local demo ready for judging.

## Test Plan
Unit tests:

- score normalization
- weighted score ordering
- heap push/pop behavior
- fix removal and rebalance
- greedy scheduler budget handling
- brute-force correctness on small fixtures

Integration tests:

- fetch to score pipeline
- API endpoint correctness with seeded data
- fallback provider behavior

Manual checks:

- BLT fetch succeeds
- GitHub fallback succeeds
- top-K updates after fix
- scheduler output stays within hour budget
- AI explanation degrades gracefully

## Assumptions
- This project is greenfield.
- Local laptop demo is the primary runtime target.
- BLT is the headline source, but fallback support is mandatory.
- `EstimatedFixHours` may be heuristic.
- Heap visualization is optional and should be dropped first if time is tight.

## Success Criteria
The project is complete when:

- real bug data can be fetched and scored
- top-priority bugs are exposed through heap-backed APIs
- a user can simulate fixing bugs from the UI
- the scheduler recommends bugs for a fixed time budget
- greedy and brute-force outputs can be compared
- the dashboard is visually distinctive and demo-friendly
- AI adds explanation value without becoming a dependency for correctness
