# Project context for Claude Code

This repo contains optimal-strategy solvers for the Norwegian dice game
"10,000" (Terning 10 000 / Cows), built via backward induction / dynamic
programming (same general approach as the author's other Yatzy solvers).

See README.md for what each subdirectory does and how to run it.

## Rules reference (confirmed with the user, not guessed)

**Basic ruleset** (`basic/`):
- Single 1 = 100, single 5 = 50
- Three of a kind: v==1 -> 1000, else v*100
- Four/five/six of a kind: double the three-of-a-kind value per extra die
  (4-kind = 2x, 5-kind = 4x, 6-kind = 8x the base triple value)
- Three pairs = 1500, straight (1-6) = 2000
- Player chooses which scoring dice to bank each throw (partial keeps
  allowed) - e.g. can leave a "free" single 1 unbanked to reroll for a
  bigger combo
- Need 1000+ points in a single turn to get "on the board" the first
  time; no minimum after that
- Bust (reroll yields zero scoring dice) forfeits the entire turn's
  accumulated score, not just the current throw
- Hot dice (all 6 dice banked) resets the reroll pool to 6

**House-rule variant** (`houserule/`), used by this particular group:
- Every scoring die MUST be banked each throw - no partial keeps
- Combining: throw 1's kept dice can combine with throw 2's dice and get
  re-scored together as if it were one 6-dice roll (e.g. keep one 1 on
  throw 1, roll two more 1s on throw 2 -> scores as a triple, 1000).
  Throw 3 onward: NO combining, each throw scored entirely on its own
  (verified against two worked examples from the user: 100->1000->1100
  and 100->200->300)
- Two different three-of-a-kinds in one roll (all 6 dice, e.g. three 2s
  + three 5s) scores as `(triple1 + triple2) * 2`
- Straight/three-pairs only possible on a genuinely fresh 6-dice throw
  (cycle start, or the throw1+2 combined result - both always total
  exactly 6 dice)
- Same bust/hot-dice/entry-threshold rules as the basic ruleset

## Modeling decisions worth knowing before touching this code

- Turn scores are tracked in units of 50 (the smallest scoring
  increment) to keep state keys small integers.
- `CAP` in each solver truncates the accumulated-turn-score state space
  at a point where the optimal policy has been verified (not assumed)
  to have converged to "always bank" - if you change scoring rules,
  re-verify this convergence rather than trusting the existing CAP value.
- The 2-player race solvers (`basic/`, `houserule/`) use exact backward
  induction over a `(myScore, oppScore)` grid, exploiting the fact that
  busting returns the game to a state with turns swapped but scores
  unchanged - this requires solving a small 2x2 linear system per score
  pair rather than pure top-down memoization, because of that circular
  dependency.
- `nplayer/` exists because the exact grid approach is O(200^N) and
  becomes infeasible past ~3 players. It instead simulates full games
  using the already-computed exact single-turn policy and counts
  outcomes (Monte Carlo). Validated against the exact 2-player number
  (53.35%) - got 53.34% over 500k simulated games.
- Known simplification, not yet addressed: the bank-vs-reroll policy
  used everywhere is a fixed, score-blind, EV-maximizing policy. It does
  not adjust turn strategy based on the opponent's score (e.g. playing
  more conservatively when close to 10,000 with a big lead, or more
  aggressively when behind). This is a reasonable approximation except
  very close to the finish line. A fully race-aware policy would need
  bank/reroll decisions to depend on opponent score too, which changes
  the single-turn state space and hasn't been built.

## Open threads / likely next asks

- Race-aware turn policy (see above) for basic and/or houserule rules
- Applying the same house-rule mechanics to the n-player simulator
  (currently n-player only wraps houserule, not basic)
- Win probability conditioned on mid-game scores, not just a cold start
- Average game length / turns-to-win stats

