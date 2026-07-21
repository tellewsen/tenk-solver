package main

import (
	"fmt"
	"math/rand"
)

// Reuses the exact house-rule scoring/decision logic from
// tenk_solver_houserule.go, but instead of exact backward induction over a
// (score, score, ...) grid (which blows up as 200^N for N players), this
// simulates full games using the already-computed OPTIMAL single-turn
// policy (policyA/B/C) and counts who wins. This scales to any number of
// players with no state-space explosion, at the cost of being a Monte
// Carlo estimate rather than an exact probability. Validated against the
// exact 2-player result (53.35%) below.

const CAP = 40000

func kindScore(v, g int) int {
	base := v * 100
	if v == 1 {
		base = 1000
	}
	switch g {
	case 3:
		return base
	case 4:
		return base * 2
	case 5:
		return base * 4
	case 6:
		return base * 8
	}
	return 0
}

func decompose(counts [6]int) (points int, used int, banked [6]int) {
	sum := 0
	for _, c := range counts {
		sum += c
	}
	if sum == 6 {
		allOne := true
		for _, c := range counts {
			if c != 1 {
				allOne = false
				break
			}
		}
		if allOne {
			return 2000, 6, counts
		}
		pairCount, validPairs := 0, true
		for _, c := range counts {
			if c == 2 {
				pairCount++
			} else if c != 0 {
				validPairs = false
			}
		}
		if validPairs && pairCount == 3 {
			return 1500, 6, counts
		}
		tripleCount, validTriples := 0, true
		var vals []int
		for i, c := range counts {
			if c == 3 {
				tripleCount++
				vals = append(vals, i+1)
			} else if c != 0 {
				validTriples = false
			}
		}
		if validTriples && tripleCount == 2 {
			pts := (kindScore(vals[0], 3) + kindScore(vals[1], 3)) * 2
			return pts, 6, counts
		}
	}
	for idx := 0; idx < 6; idx++ {
		v := idx + 1
		c := counts[idx]
		if v == 1 || v == 5 {
			unit := 100
			if v == 5 {
				unit = 50
			}
			if c >= 3 {
				points += kindScore(v, c)
			} else {
				points += unit * c
			}
			used += c
			banked[idx] = c
		} else if c >= 3 {
			points += kindScore(v, c)
			used += c
			banked[idx] = c
		}
	}
	return points, used, banked
}

type RollOutcome struct {
	prob   float64
	counts [6]int
}

func factorial(x int) float64 {
	r := 1.0
	for i := 2; i <= x; i++ {
		r *= float64(i)
	}
	return r
}
func pow6(n int) float64 {
	r := 1.0
	for i := 0; i < n; i++ {
		r *= 6
	}
	return r
}

func generateOutcomes(n int) []RollOutcome {
	var outcomes []RollOutcome
	var counts [6]int
	var rec func(face, remaining int)
	rec = func(face, remaining int) {
		if face == 6 {
			if remaining == 0 {
				prob := factorial(n)
				for _, c := range counts {
					prob /= factorial(c)
				}
				prob /= pow6(n)
				outcomes = append(outcomes, RollOutcome{prob, counts})
			}
			return
		}
		for c := 0; c <= remaining; c++ {
			counts[face] = c
			rec(face+1, remaining-c)
		}
		counts[face] = 0
	}
	rec(0, n)
	return outcomes
}

var outcomesByN [7][]RollOutcome

func initOutcomes() {
	for n := 1; n <= 6; n++ {
		outcomesByN[n] = generateOutcomes(n)
	}
}

var threshold int
var memoA map[int]float64
var policyA map[int]bool

func VA(aUnits int) float64 {
	a := aUnits * 50
	if a >= CAP {
		return float64(a)
	}
	if v, ok := memoA[aUnits]; ok {
		return v
	}
	R := 0.0
	for _, o := range outcomesByN[6] {
		points, used, banked := decompose(o.counts)
		var val float64
		switch {
		case used == 0:
			val = 0
		case used == 6:
			val = VA(aUnits + points/50)
		default:
			val = VB(6-used, banked, aUnits)
		}
		R += o.prob * val
	}
	bankVal := 0.0
	if a >= threshold {
		bankVal = float64(a)
	}
	result := bankVal
	isBank := true
	if R > result {
		result = R
		isBank = false
	}
	memoA[aUnits] = result
	policyA[aUnits] = isBank
	return result
}

type keyB struct {
	n1   int
	kept [6]int
	a    int
}

var memoB map[keyB]float64
var policyB map[keyB]bool

func VB(n1 int, kept [6]int, aUnits int) float64 {
	a := aUnits * 50
	key := keyB{n1, kept, aUnits}
	if v, ok := memoB[key]; ok {
		return v
	}
	points1, used1, _ := decompose(kept)
	bankVal := 0.0
	if a+points1 >= threshold {
		bankVal = float64(a + points1)
	}
	R := 0.0
	for _, o := range outcomesByN[n1] {
		var combined [6]int
		for i := 0; i < 6; i++ {
			combined[i] = kept[i] + o.counts[i]
		}
		pointsC, usedC, _ := decompose(combined)
		var val float64
		switch {
		case usedC == used1:
			val = 0
		case usedC == 6:
			val = VA(aUnits + pointsC/50)
		default:
			val = VC(6-usedC, aUnits+pointsC/50)
		}
		R += o.prob * val
	}
	result := bankVal
	isBank := true
	if R > result {
		result = R
		isBank = false
	}
	memoB[key] = result
	policyB[key] = isBank
	return result
}

type keyC struct{ n, a int }

var memoC map[keyC]float64
var policyC map[keyC]bool

func VC(n, aUnits int) float64 {
	a := aUnits * 50
	if a >= CAP {
		return float64(a)
	}
	key := keyC{n, aUnits}
	if v, ok := memoC[key]; ok {
		return v
	}
	R := 0.0
	for _, o := range outcomesByN[n] {
		points, used, _ := decompose(o.counts)
		var val float64
		switch {
		case used == 0:
			val = 0
		case used == n:
			val = VA(aUnits + points/50)
		default:
			val = VC(n-used, aUnits+points/50)
		}
		R += o.prob * val
	}
	bankVal := 0.0
	if a >= threshold {
		bankVal = float64(a)
	}
	result := bankVal
	isBank := true
	if R > result {
		result = R
		isBank = false
	}
	memoC[key] = result
	policyC[key] = isBank
	return result
}

func resetMemos() {
	memoA = map[int]float64{}
	policyA = map[int]bool{}
	memoB = map[keyB]float64{}
	policyB = map[keyB]bool{}
	memoC = map[keyC]float64{}
	policyC = map[keyC]bool{}
}

// ==== two saved policy sets: on-board (threshold 0) and entry (threshold 1000) ====

var policyA0, policyA1000 map[int]bool
var policyB0, policyB1000 map[keyB]bool
var policyC0, policyC1000 map[keyC]bool

func buildPolicies() {
	threshold = 1000
	resetMemos()
	VA(0)
	policyA1000, policyB1000, policyC1000 = policyA, policyB, policyC

	threshold = 0
	resetMemos()
	VA(0)
	policyA0, policyB0, policyC0 = policyA, policyB, policyC
}

func getPolicyA(th, aUnits int) bool {
	if th == 0 {
		return policyA0[aUnits]
	}
	return policyA1000[aUnits]
}
func getPolicyB(th, n1 int, kept [6]int, aUnits int) bool {
	key := keyB{n1, kept, aUnits}
	if th == 0 {
		return policyB0[key]
	}
	return policyB1000[key]
}
func getPolicyC(th, n, aUnits int) bool {
	key := keyC{n, aUnits}
	if th == 0 {
		return policyC0[key]
	}
	return policyC1000[key]
}

// ==== simulation ====

func rollDice(n int) [6]int {
	var counts [6]int
	for i := 0; i < n; i++ {
		counts[rand.Intn(6)]++
	}
	return counts
}

type phase int

const (
	phaseA phase = iota
	phaseB
	phaseC
)

// simulateTurn plays one full turn (following the exact optimal policy for
// the given threshold: 0 = already on the board, 1000 = entry rule) and
// returns the points gained (0 if busted).
func simulateTurn(th int) int {
	aUnits := 0
	ph := phaseA
	var n1 int
	var kept [6]int
	var nC int

	for {
		switch ph {
		case phaseA:
			if getPolicyA(th, aUnits) {
				return aUnits * 50
			}
			counts := rollDice(6)
			points, used, banked := decompose(counts)
			if used == 0 {
				return 0
			}
			if used == 6 {
				aUnits += points / 50
				continue
			}
			n1 = 6 - used
			kept = banked
			ph = phaseB
		case phaseB:
			if getPolicyB(th, n1, kept, aUnits) {
				points1, _, _ := decompose(kept)
				return (aUnits + points1/50) * 50
			}
			o := rollDice(n1)
			var combined [6]int
			for i := 0; i < 6; i++ {
				combined[i] = kept[i] + o[i]
			}
			pointsC, usedC, _ := decompose(combined)
			_, used1, _ := decompose(kept)
			if usedC == used1 {
				return 0
			}
			if usedC == 6 {
				aUnits += pointsC / 50
				ph = phaseA
				continue
			}
			nC = 6 - usedC
			aUnits += pointsC / 50
			ph = phaseC
		case phaseC:
			if getPolicyC(th, nC, aUnits) {
				return aUnits * 50
			}
			o := rollDice(nC)
			points, used, _ := decompose(o)
			if used == 0 {
				return 0
			}
			if used == nC {
				aUnits += points / 50
				ph = phaseA
				continue
			}
			nC = nC - used
			aUnits += points / 50
		}
	}
}

// simulateGame plays one full N-player game (turn order 0..N-1, repeating)
// and returns the index of the winner. Ties in the final-round comparison
// go to whoever triggered the ending.
func simulateGame(numPlayers int) int {
	scores := make([]int, numPlayers)
	onBoard := make([]bool, numPlayers)

	for {
		for p := 0; p < numPlayers; p++ {
			th := 1000
			if onBoard[p] {
				th = 0
			}
			gain := simulateTurn(th)
			scores[p] += gain
			if gain > 0 {
				onBoard[p] = true
			}
			if scores[p] >= 10000 {
				// everyone else gets exactly one more turn
				for q := 1; q < numPlayers; q++ {
					pp := (p + q) % numPlayers
					thq := 1000
					if onBoard[pp] {
						thq = 0
					}
					gq := simulateTurn(thq)
					scores[pp] += gq
				}
				best, bestScore := p, scores[p]
				for q := 0; q < numPlayers; q++ {
					if q != p && scores[q] > bestScore {
						best, bestScore = q, scores[q]
					}
				}
				return best
			}
		}
	}
}

func monteCarloWinProbs(numPlayers, numGames int) []float64 {
	wins := make([]int, numPlayers)
	for i := 0; i < numGames; i++ {
		wins[simulateGame(numPlayers)]++
	}
	probs := make([]float64, numPlayers)
	for i := range probs {
		probs[i] = float64(wins[i]) / float64(numGames)
	}
	return probs
}

func main() {
	initOutcomes()
	buildPolicies()

	const numGames = 500000

	fmt.Println("=== Validation: 2 players (compare to exact DP result 0.5335) ===")
	p2 := monteCarloWinProbs(2, numGames)
	for i, p := range p2 {
		fmt.Printf("  player %d (turn order %d): %.4f\n", i+1, i+1, p)
	}

	for _, n := range []int{3, 4, 5} {
		fmt.Printf("\n=== %d players ===\n", n)
		probs := monteCarloWinProbs(n, numGames)
		for i, p := range probs {
			fmt.Printf("  player %d (turn order %d): %.4f\n", i+1, i+1, p)
		}
	}
}
