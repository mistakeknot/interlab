# interlab

Autonomous experiment loop for Demarch — an Interverse plugin providing 3 stateless MCP tools for running, recording, and managing optimization experiments.

## Tools

- **init_experiment** — Configure a new experiment campaign: metric, direction, benchmark command, files in scope
- **run_experiment** — Execute the benchmark, capture output and timing, check circuit breaker
- **log_experiment** — Record result: keep (commit), discard (revert), or crash (revert + increment crash counter)

## Installation

Install as a Claude Code plugin:
```bash
claude plugin install interlab
```

## License

MIT
