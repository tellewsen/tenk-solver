# tenk-solver

Optimal-strategy solvers for the Norwegian dice game "10,000" (Terning
10 000 / Cows), commonly bundled with Yatzy sets.

## Contents

- **`basic/`** — solver for the standard ruleset: players choose which
  scoring dice to bank each throw (partial keeps allowed), single
  three-of-a-kind/four/five/six-of-a-kind scoring, three pairs (1500),
  straight (2000). Includes a single-turn optimal policy (bank vs.
  reroll thresholds, bust probabilities, expected value) and an exact
  2-player race-to-10,000 solver via backward induction.

- **`houserule/`** — solver for a house-rule variant: every scoring die
  must be banked each throw (no partial keeps), throw 1 and throw 2 of a
  cycle can combine into a bigger set (e.g. one kept "1" plus two more
  "1"s on the next throw scores as a triple), throw 3 onward is scored
  independently with no combining, and two different three-of-a-kinds in
  one roll score as `(triple1 + triple2) * 2`. Includes the same
  single-turn policy and exact 2-player race solver as `basic/`.

- **`nplayer/`** — Monte Carlo win-probability simulator for 2–5+
  players under the house rules, built on the exact optimal single-turn
  policy from `houserule/`. Used because exact backward induction over
  the race becomes infeasible past 3 players (state space grows as
  `200^N`); this instead simulates full games using the known-optimal
  turn policy and counts outcomes, validated against the exact 2-player
  result (53.35%).

## Usage

Each subdirectory is a standalone Go program:

```bash
cd basic && go run main.go
cd houserule && go run main.go
cd nplayer && go run main.go
```

No external dependencies.

## Modeling notes

All solvers use backward induction / dynamic programming over
`(dice remaining, turn score so far)` states, with turn scores tracked
in units of 50 (the smallest scoring increment). A cap (`CAP` constant)
truncates the accumulated-score state space at a point well past where
the optimal policy converges to "always bank" — this is verified in
each solver rather than assumed.

The `houserule` scoring model additionally tracks which state within a
"combining cycle" the turn is in (fresh 6-dice throw / throw-2 combining
window / independent-throw phase past that window), since the mandatory
combining rule changes what state is needed to make the bank-vs-reroll
decision correctly.

The 2-player race solver treats the bank-vs-reroll decision as a fixed,
score-blind, EV-maximizing policy (i.e. it does not adjust turn strategy
based on the opponent's score). This is a reasonable approximation
except very close to the 10,000 finish line, where a genuinely
race-aware player might play more conservatively/aggressively than this
policy suggests.
