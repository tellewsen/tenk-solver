package main

import (
	"fmt"
)

// ==== Scoring rules ====
// Single 1  = 100
// Single 5  = 50
// Three of a kind of value v: v==1 -> 1000, else v*100
// Four of a kind: double the three-of-a-kind value
// Five of a kind: double again
// Six of a kind: double again
// Three pairs (uses all 6 dice) = 1500
// Straight 1-2-3-4-5-6 (uses all 6 dice) = 2000
//
// Player may choose which scoring dice to bank each roll and which to
// reroll (partial keep allowed). Busting (no scoring die on a reroll)
// forfeits the ENTIRE turn's accumulated score. Using all 6 dice in
// scoring (hot dice) resets the reroll pool back to 6.

const CAP = 40000 // accumulated turn score beyond which we treat "always bank" as optimal (verified below to be well past the real decision threshold)

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

type Outcome struct {
	prob       float64
	isBust     bool
	candidates map[int]int // k = dice banked -> max points achievable using exactly k dice
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

func generateOutcomes(n int) []Outcome {
	var outcomes []Outcome
	var counts [6]int
	var rec func(face, remaining int)
	rec = func(face, remaining int) {
		if face == 6 {
			if remaining == 0 {
				outcomes = append(outcomes, buildOutcome(n, counts))
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

func buildOutcome(n int, counts [6]int) Outcome {
	prob := factorial(n)
	for _, c := range counts {
		prob /= factorial(c)
	}
	prob /= pow6(n)

	type opt struct{ b, pts int }
	var faceOpts [6][]opt
	for idx := 0; idx < 6; idx++ {
		v := idx + 1
		c := counts[idx]
		opts := []opt{{0, 0}}
		if v == 1 || v == 5 {
			unit := 100
			if v == 5 {
				unit = 50
			}
			for b := 1; b <= c; b++ {
				pts := unit * b
				if b >= 3 {
					pts = kindScore(v, b)
				}
				opts = append(opts, opt{b, pts})
			}
		} else {
			for _, g := range []int{3, 4, 5, 6} {
				if g <= c {
					opts = append(opts, opt{g, kindScore(v, g)})
				}
			}
		}
		faceOpts[idx] = opts
	}

	best := map[int]int{}
	var rec func(face, k, pts int)
	rec = func(face, k, pts int) {
		if face == 6 {
			if k > 0 {
				if cur, ok := best[k]; !ok || pts > cur {
					best[k] = pts
				}
			}
			return
		}
		for _, o := range faceOpts[face] {
			rec(face+1, k+o.b, pts+o.pts)
		}
	}
	rec(0, 0, 0)

	if n == 6 {
		allDistinct := true
		for _, c := range counts {
			if c != 1 {
				allDistinct = false
				break
			}
		}
		if allDistinct {
			if cur, ok := best[6]; !ok || 2000 > cur {
				best[6] = 2000
			}
		}
		pairCount := 0
		validPairs := true
		for _, c := range counts {
			if c == 2 {
				pairCount++
			} else if c != 0 {
				validPairs = false
			}
		}
		if validPairs && pairCount == 3 {
			if cur, ok := best[6]; !ok || 1500 > cur {
				best[6] = 1500
			}
		}
	}

	return Outcome{prob: prob, isBust: len(best) == 0, candidates: best}
}

var outcomesByN [7][]Outcome

func initOutcomes() {
	for n := 1; n <= 6; n++ {
		outcomesByN[n] = generateOutcomes(n)
	}
}

type stateKey struct{ n, a int }

var memo map[stateKey]float64
var threshold int // minimum turn total required to be allowed to bank (0 = no minimum)

func V(n, aUnits int) float64 {
	a := aUnits * 50
	if a >= CAP {
		return float64(a)
	}
	key := stateKey{n, aUnits}
	if v, ok := memo[key]; ok {
		return v
	}
	bankVal := 0.0
	if a >= threshold {
		bankVal = float64(a)
	}
	R := 0.0
	for _, o := range outcomesByN[n] {
		if o.isBust {
			continue
		}
		best := -1.0
		for k, pts := range o.candidates {
			nprime := n - k
			if nprime == 0 {
				nprime = 6
			}
			if val := V(nprime, aUnits+pts/50); val > best {
				best = val
			}
		}
		R += o.prob * best
	}
	result := bankVal
	if R > result {
		result = R
	}
	memo[key] = result
	return result
}

func findThreshold(n int) int {
	for aUnits := 0; aUnits*50 < CAP; aUnits++ {
		a := aUnits * 50
		bankVal := float64(a)
		if a < threshold {
			bankVal = 0
		}
		if V(n, aUnits) <= bankVal+0.001 {
			return a
		}
	}
	return CAP
}

var probMemo map[stateKey]float64

func probReachThreshold(n, aUnits int) float64 {
	a := aUnits * 50
	if a >= threshold {
		return 1.0
	}
	if a >= CAP {
		return 0.0
	}
	key := stateKey{n, aUnits}
	if v, ok := probMemo[key]; ok {
		return v
	}
	p := 0.0
	for _, o := range outcomesByN[n] {
		if o.isBust {
			continue
		}
		bestK, bestVal := -1, -1.0
		for k, pts := range o.candidates {
			nprime := n - k
			if nprime == 0 {
				nprime = 6
			}
			if val := V(nprime, aUnits+pts/50); val > bestVal {
				bestVal = val
				bestK = k
			}
		}
		pts := o.candidates[bestK]
		nprime := n - bestK
		if nprime == 0 {
			nprime = 6
		}
		p += o.prob * probReachThreshold(nprime, aUnits+pts/50)
	}
	probMemo[key] = p
	return p
}

func main() {
	initOutcomes()

	fmt.Println("=== Bust probability by dice rolled ===")
	for n := 1; n <= 6; n++ {
		bp := 0.0
		for _, o := range outcomesByN[n] {
			if o.isBust {
				bp += o.prob
			}
		}
		fmt.Printf("  n=%d: %.4f\n", n, bp)
	}

	fmt.Println("\n=== Normal turn (already on the board) ===")
	threshold = 0
	memo = map[stateKey]float64{}
	fmt.Printf("EV of a fresh turn (6 dice, 0 banked): %.1f\n", V(6, 0))
	for n := 1; n <= 6; n++ {
		fmt.Printf("  n=%d dice remaining: bank once turn total >= %d\n", n, findThreshold(n))
	}

	fmt.Println("\n=== Getting on the board (need 1000+ in one turn) ===")
	threshold = 1000
	memo = map[stateKey]float64{}
	probMemo = map[stateKey]float64{}
	fmt.Printf("EV of a fresh turn under entry rules: %.1f\n", V(6, 0))
	fmt.Printf("P(successfully reach 1000+ in one turn): %.4f\n", probReachThreshold(6, 0))
}
