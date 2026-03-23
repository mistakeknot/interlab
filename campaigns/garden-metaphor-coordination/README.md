# Campaign: garden-metaphor-coordination

**Task Type:** `prompt-framing`
**Hypothesis:** Agents coordinate better and exercise more autonomy when work is framed using garden/cultivation metaphors vs factory/industrial metaphors
**Inspiration:** Alex Komoroske (gardening vs building), Maggie Appleton (digital gardens)

## Background

Meadowsyn's UI is shifting from factory language to garden language. The deeper question: does the metaphor in agent *system prompts* affect coordination quality, autonomy, and decision-making?

Factory framing: "execute task", "dispatch agent", "queue depth", "blocked", "pipeline"
Garden framing: "tend to", "seed work", "cultivate", "wilting", "pruning", "growth"

## Research Questions

1. **Do agents make better autonomous decisions** (fewer false escalations, better judgment calls) when prompted with garden metaphors vs factory metaphors?
2. **Does coordination quality change** (fewer conflicts, better handoffs, less over-communication) under garden framing?
3. **Does the metaphor affect risk tolerance?** Gardens imply patience and organic growth; factories imply speed and throughput. Does garden framing reduce premature optimization?
4. **Is there a novelty/priming effect?** Do agents just perform better with any non-standard framing, or is garden specifically better than factory?

## Experiment Design

### Phase 1: Literature Search (web research, no code changes)

Search for:
- Academic papers on metaphor effects in LLM prompting
- Komoroske's "Gardening Strategies" deck and related writing
- Appleton's digital garden philosophy
- Any research on prompt framing effects on agent autonomy/coordination
- Related: Lakoff & Johnson's "Metaphors We Live By" applied to AI systems

### Phase 2: A/B Prompt Comparison (controlled experiment)

**Control:** Current factory-framed prompts (dispatch, execute, queue, blocked)
**Treatment:** Garden-reframed prompts (tend, seed, cultivate, wilting, pruning)

**Metric candidates:**
- `autonomy_score`: ratio of (decisions made without escalation) / (total decision points)
- `coordination_quality`: conflicts per session, redundant work incidents
- `judgment_accuracy`: did the agent's autonomous decision match what a human would have chosen?
- `rework_rate`: garden framing might reduce premature shipping (pruning = quality gates)

**Benchmark approach:**
- Run same set of 10 representative tasks under both framings
- Score outputs blind (evaluator doesn't know which framing produced which output)
- Compare on autonomy, coordination, and judgment metrics

### Phase 3: UI Text Variants

If Phase 2 shows signal, extend garden vocabulary to Meadowsyn UI:
- Status labels: EXEC→TENDING, DISP→SEEDING, IDLE→RESTING, FAIL→WILTING, GATE→PRUNING
- Make it a user-toggleable setting (factory mode / garden mode)

## Prior Art to Check

- `docs/research/assess-*.md` — any existing metaphor/framing research
- PHILOSOPHY.md — does the project philosophy already encode a preferred metaphor?
- Komoroske's deck: https://komoroske.com/slime-mold/ (complex systems, not exactly gardens but related)
