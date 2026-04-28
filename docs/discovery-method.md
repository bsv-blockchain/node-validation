# Discovery methodology

This document records how `docs/discovery.md` was produced so a future
engineer can reproduce or extend the discovery without re-deriving the
methodology from scratch.

## Upstream pinning

- Repository: `/Users/oskarsson/gitcheckout/teranode/`
- Commit:     `<filled in by Task 4>`
- Branch:     `<filled in by Task 4>`
- Captured:   `<filled in by Task 4>`

## Agent fan-out

Nine parallel `Explore` agents, each over `/Users/oskarsson/gitcheckout/teranode/`.

| # | Surface(s) | Search anchors |
|---|---|---|
| 1 | JSON-RPC + auth model | (filled in by Task 3) |
| 2 | REST / Asset HTTP API | (filled in by Task 3) |
| 3 | Notifications (WS / SSE / gRPC / Kafka) | (filled in by Task 3) |
| 4 | P2P listener | (filled in by Task 3) |
| 5 | Metrics + Health | (filled in by Task 3) |
| 6 | Extended transaction format | (filled in by Task 3) |
| 7 | Mempool + testmempoolaccept + fee estimation | (filled in by Task 3) |
| 8 | Double-spend detection / notification | (filled in by Task 3) |
| 9 | Settings / configuration | (filled in by Task 3) |

## Synthesis rules

1. Each agent's output is appended verbatim to `docs/discovery.md` in the order above.
2. Cross-agent conflicts (e.g. port disagreements between code and `settings.conf`) are inserted by the orchestrator as `**Discrepancy noted**` callouts citing both source references.
3. `docs/discovery.yaml` is derived programmatically from the same findings; the markdown is the human-readable face, the YAML is the structural source of truth.
