# Bug Bounty Priority Engine

A hackathon project that turns bug triage into a strategy game.

The Bug Bounty Priority Engine fetches real bug reports from OWASP BLT, scores them using a weighted formula, stores them in a max heap for fast priority access, and recommends which bugs to fix first within a limited developer hour budget.

Think of it like Clash Royale for open source maintenance: bugs are invading troops, and this engine tells you which card to play next.

## Why This Exists
Maintainers often face hundreds of open issues and bug bounty reports with limited time to investigate and fix them. Raw issue lists are noisy and hard to prioritize.

This project helps by answering four practical questions:

- Which bugs matter most right now?
- What are the top 5 bugs I should look at first?
- If I fix one bug, how does the priority landscape change?
- Given limited developer time, which bugs maximize impact?

## Core Features
- Real issue ingestion from OWASP BLT
- GitHub issues fallback for demo reliability
- Weighted bug scoring with normalized inputs
- Max heap for fast top priority retrieval
- Fix simulation with heap rebalance
- Greedy scheduler for hour constrained planning
- Brute force comparison for algorithmic validation
- Royale inspired dashboard UI
- AI explanation layer for score reasoning and battle-plan narration

## Tech Stack
- Go
- Standard library HTTP server
- `container/heap`
- HTML / CSS / JavaScript
- Optional AI API for explanation generation

## Project Structure
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

## Data Model
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

## Scoring Formula
Each metric is normalized to a 0-100 scale before applying weights:

```text
Priority =
  (Severity * 0.4) +
  (BountyValue * 0.3) +
  (Reproductions * 0.2) +
  (Age * 0.1)
```

This favors severe, high-value, reproducible, and long-open bugs.

## Scheduling Strategy
The scheduler uses a greedy strategy based on:

```text
priority / estimated_fix_hours
```

This approximates the best "impact per hour" selection under a fixed developer-time budget.

A brute-force comparator is also included for small datasets to show how close greedy gets to the optimal answer.

## API Endpoints
### `GET /api/bugs`
Fetch and return all scored bugs.

### `GET /api/top?k=5`
Return the top-K bugs by priority.

### `POST /api/fix/:id`
Simulate fixing a bug by removing it from the heap.

### `GET /api/schedule?hours=8`
Return the recommended bugs to fix within the given hour budget.

### `GET /api/compare`
Compare greedy scheduling against brute-force optimal selection.

### `GET /api/explain/{id}`
Return a human-readable explanation of why a bug ranks highly, including:
- backward-compatible `summary` and `detail`
- `agents.security` and `agents.optimization` persona outputs
- `ai` metadata (`provider`, `model`, `fallback`, `reason`)

## Frontend
The dashboard is designed around a Clash Royale-inspired presentation:

- bugs shown as troop-style cards
- estimated fix hours shown like elixir cost
- fix action presented as a deploy button
- top heap represented as your "hand"
- scheduler output shown as a battle plan

Core UI features:

- live top-K bug list
- score breakdown bars
- fix simulation
- schedule recommendation
- greedy vs brute-force comparison

## AI Layer
AI is used as an explanation layer, not as the ranking engine.

It can be used to:
- summarize bug reports
- explain why a bug is high priority
- narrate the chosen fix plan
- generate a short judge-facing insight

If AI is unavailable, the app should fall back to deterministic template-based explanations so the demo remains stable.

### Multi-Agent Setup (Groq)
Set these environment variables to enable live dual-agent explainability:

```bash
GROQ_API_KEY=your_key_here
GROQ_MODEL=llama-3.1-8b-instant
# Optional
GROQ_API_URL=https://api.groq.com/openai/v1/chat/completions
GROQ_TIMEOUT_SEC=12
GROQ_MAX_TOKENS=220
GROQ_TEMPERATURE=0.2
```

When `GROQ_API_KEY` is missing or Groq fails, the API automatically returns deterministic security and optimization explanations.

## Local Run Plan
Expected developer flow:

```bash
go run main.go
```

Then open the local server in a browser and interact with the dashboard.

A full implementation should also support:
- fetching live issues on startup or on demand
- serving frontend assets from the Go server
- local demo without cloud deployment

## Demo Story
This project combines:
- real-world relevance
- strong DSA foundations
- live interactivity
- visually memorable presentation

Judge pitch:

> Most open source repos have hundreds of issues. Maintainers waste time deciding what to fix. We built a priority engine that fetches real OWASP BLT issues, scores them using a weighted formula, stores them in a max-heap for O(log n) access, and uses greedy scheduling to maximize bugs fixed within developer time constraints.

## What Makes It Strong
- Real data, not just mock objects
- Multiple algorithmic layers: scoring, heap, greedy, brute force
- Interactive simulation, not a static dashboard
- AI adds explanation value without undermining the DSA story
- Clear and memorable product framing

## Planned Testing
- unit tests for score calculation and normalization
- heap behavior tests
- scheduler tests
- API handler tests
- manual demo validation against live and fallback data sources

## Assumptions
- BLT is the primary source of truth
- GitHub fallback exists for reliability
- estimated fix hours may be derived heuristically
- local demo reliability matters more than cloud deployment

## Future Extensions
- persistent cache or database
- user-adjustable scoring weights
- custom issue-source connectors
- collaborative team scheduling
- historical triage analytics
- richer heap visualization

## License
Hackathon prototype. Add a license before publishing publicly.
