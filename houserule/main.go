package main

import "fmt"

// House rules implemented here:
// - Every scoring die must be banked each throw (no choosing to leave
//   points unbanked to chase a bigger combo).
// - Combining: dice banked on throw 1 of a cycle can combine with throw 2's
//   dice (re-scored together, as if it were one 6-dice roll). From throw 3
//   onward, each throw is scored entirely on its own - no combining with
//   anything kept earlier.
// - Two different three-of-a-kinds in one roll (all 6 dice, e.g. three 2s
//   + three 5s): score = (triple1 + triple2) * 2.
// - Straight and three-pairs still require a genuinely fresh 6-dice roll
//   (throw 1 of a cycle, or the throw1+2 combined result).
// - Hot dice (all 6 banked) resets to a fresh cycle.
// - Busting a throw (zero new scoring dice) forfeits the WHOLE turn.

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

// decompose mandatorily scores a set of dice (given as face counts). If the
// counts sum to 6, whole-roll specials (straight, three pairs, two triples)
// are checked first, since this is reused both for a genuinely fresh
// 6-dice throw AND for the throw1+2 combined re-scoring (which always
// totals exactly 6 dice too).
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

// ==== DP: three mutually-recursive states ====
// VA: about to roll a fresh 6 dice (cycle start)
// VB: threw once already (0<used<6), about to roll the remainder with
//     combining still available against `kept`
// VC: past the combining window; from here each throw is independent

var threshold int // minimum turn total required to bank (0 = none, 1000 = entry rule)

var memoA map[int]float64
var policyA map[int]bool // true = bank

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
			val = 0 // bust: no new scoring dice this throw
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

// ---- sanity check against the worked example from the conversation ----
func sanityCheck() {
	c1 := [6]int{1, 0, 2, 1, 0, 2} // sum=6, one "1", rest junk
	pts1, used1, banked1 := decompose(c1)
	fmt.Printf("[check] throw1 one-1 case: points=%d used=%d (want 100,1)\n", pts1, used1)

	o2 := [6]int{2, 0, 1, 1, 0, 1} // sum=5, two more "1"s
	var combined [6]int
	for i := 0; i < 6; i++ {
		combined[i] = banked1[i] + o2[i]
	}
	ptsC, usedC, _ := decompose(combined)
	fmt.Printf("[check] after throw2 (combined): points=%d used=%d (want 1000,3)\n", ptsC, usedC)

	o3 := [6]int{1, 0, 1, 0, 0, 1} // sum=3, one more "1", no combining
	pts3, used3, _ := decompose(o3)
	fmt.Printf("[check] throw3 independent: points=%d used=%d, running total=%d (want 100,1,1100)\n",
		pts3, used3, ptsC+pts3)

	o2b := [6]int{1, 0, 1, 1, 0, 1} // sum=4, only one more "1" (not two)
	var combinedB [6]int
	for i := 0; i < 6; i++ {
		combinedB[i] = banked1[i] + o2b[i]
	}
	ptsB, usedB, _ := decompose(combinedB)
	fmt.Printf("[check] two-ones-total combined (not a triple): points=%d used=%d (want 200,2)\n", ptsB, usedB)

	dt := [6]int{0, 3, 0, 0, 3, 0} // three 2s + three 5s
	ptsDT, usedDT, _ := decompose(dt)
	fmt.Printf("[check] double triple (three 2s + three 5s): points=%d used=%d (want 1400,6)\n", ptsDT, usedDT)
}

// ==== Forward pass: exact turn-gain distribution under the optimal policy ====

var distMemoA map[int]map[int]float64
var distMemoB map[keyB]map[int]float64
var distMemoC map[keyC]map[int]float64

func DA(aUnits int) map[int]float64 {
	if aUnits*50 >= CAP {
		return map[int]float64{aUnits: 1.0}
	}
	if d, ok := distMemoA[aUnits]; ok {
		return d
	}
	result := map[int]float64{}
	if policyA[aUnits] {
		result[aUnits] = 1.0
	} else {
		for _, o := range outcomesByN[6] {
			points, used, banked := decompose(o.counts)
			var sub map[int]float64
			switch {
			case used == 0:
				result[0] += o.prob
				continue
			case used == 6:
				sub = DA(aUnits + points/50)
			default:
				sub = DB(6-used, banked, aUnits)
			}
			for u, p := range sub {
				result[u] += o.prob * p
			}
		}
	}
	distMemoA[aUnits] = result
	return result
}

func DB(n1 int, kept [6]int, aUnits int) map[int]float64 {
	key := keyB{n1, kept, aUnits}
	if d, ok := distMemoB[key]; ok {
		return d
	}
	_, used1, _ := decompose(kept)
	result := map[int]float64{}
	if policyB[key] {
		points1, _, _ := decompose(kept)
		result[aUnits+points1/50] = 1.0
	} else {
		for _, o := range outcomesByN[n1] {
			var combined [6]int
			for i := 0; i < 6; i++ {
				combined[i] = kept[i] + o.counts[i]
			}
			pointsC, usedC, _ := decompose(combined)
			var sub map[int]float64
			switch {
			case usedC == used1:
				result[0] += o.prob
				continue
			case usedC == 6:
				sub = DA(aUnits + pointsC/50)
			default:
				sub = DC(6-usedC, aUnits+pointsC/50)
			}
			for u, p := range sub {
				result[u] += o.prob * p
			}
		}
	}
	distMemoB[key] = result
	return result
}

func DC(n, aUnits int) map[int]float64 {
	if aUnits*50 >= CAP {
		return map[int]float64{aUnits: 1.0}
	}
	key := keyC{n, aUnits}
	if d, ok := distMemoC[key]; ok {
		return d
	}
	result := map[int]float64{}
	if policyC[key] {
		result[aUnits] = 1.0
	} else {
		for _, o := range outcomesByN[n] {
			points, used, _ := decompose(o.counts)
			var sub map[int]float64
			switch {
			case used == 0:
				result[0] += o.prob
				continue
			case used == n:
				sub = DA(aUnits + points/50)
			default:
				sub = DC(n-used, aUnits+points/50)
			}
			for u, p := range sub {
				result[u] += o.prob * p
			}
		}
	}
	distMemoC[key] = result
	return result
}

func resetDistMemos() {
	distMemoA = map[int]map[int]float64{}
	distMemoB = map[keyB]map[int]float64{}
	distMemoC = map[keyC]map[int]float64{}
}

// ==== Two-player race to 10,000 (same math as the base-rules solver) ====

const targetScore = 10000
const unitSize = 50
const gridN = targetScore / unitSize // 200

var distEntry, distBoard map[int]float64

func distFor(onBoard bool) map[int]float64 {
	if onBoard {
		return distBoard
	}
	return distEntry
}

func bustProbFor(onBoard bool) float64 {
	return distFor(onBoard)[0]
}

func finalRoundWinProb(meFinalUnits, oppUnits int) float64 {
	d := distFor(oppUnits > 0)
	p := 0.0
	for g, prob := range d {
		if oppUnits+g <= meFinalUnits {
			p += prob
		}
	}
	return p
}

var W [][]float64

func computeC(meUnits, oppUnits int) float64 {
	d := distFor(meUnits > 0)
	c := 0.0
	for g, prob := range d {
		if g == 0 {
			continue
		}
		meFinal := meUnits + g
		var val float64
		if meFinal >= gridN {
			val = finalRoundWinProb(meFinal, oppUnits)
		} else {
			val = 1 - W[oppUnits][meFinal]
		}
		c += prob * val
	}
	return c
}

func solveRace() {
	W = make([][]float64, gridN)
	for i := range W {
		W[i] = make([]float64, gridN)
	}
	maxT := 2 * (gridN - 1)
	for T := maxT; T >= 0; T-- {
		lo := 0
		if T-(gridN-1) > lo {
			lo = T - (gridN - 1)
		}
		hi := gridN - 1
		if T < hi {
			hi = T
		}
		for me := lo; me <= hi; me++ {
			opp := T - me
			if opp < me {
				continue
			}
			if me == opp {
				p0 := bustProbFor(me > 0)
				c := computeC(me, opp)
				W[me][opp] = (p0 + c) / (1 + p0)
			} else {
				p0x := bustProbFor(me > 0)
				cx := computeC(me, opp)
				p0y := bustProbFor(opp > 0)
				cy := computeC(opp, me)
				denom := 1 - p0x*p0y
				x := (p0x*(1-p0y) - p0x*cy + cx) / denom
				y := p0y - p0y*x + cy
				W[me][opp] = x
				W[opp][me] = y
			}
		}
	}
}

func findThresholdA(t int) int {
	threshold = t
	resetMemos()
	for a := 0; a*50 < CAP; a++ {
		if VA(a) <= float64(a*50)+0.001 {
			return a * 50
		}
	}
	return CAP
}

func main() {
	initOutcomes()
	sanityCheck()

	fmt.Println()
	fmt.Println("=== House-rule normal turn (already on the board) ===")
	threshold = 0
	resetMemos()
	fmt.Printf("EV of a fresh turn (6 dice, 0 banked): %.1f\n", VA(0))
	fmt.Printf("Fresh-cycle bank threshold: %d\n", findThresholdA(0))

	fmt.Println()
	fmt.Println("=== House-rule getting on the board (need 1000+) ===")
	threshold = 1000
	resetMemos()
	fmt.Printf("EV of a fresh turn under entry rules: %.1f\n", VA(0))

	// Build exact turn-gain distributions for both regimes.
	resetDistMemos()
	distEntry = DA(0)

	threshold = 0
	resetMemos()
	VA(0)
	resetDistMemos()
	distBoard = DA(0)

	fmt.Println()
	fmt.Println("=== 2-player race to 10,000 (house rules) ===")
	solveRace()
	fmt.Printf("P(first player to roll wins), both at 0: %.4f\n", W[0][0])

	samples := [][2]int{{0, 0}, {40, 20}, {100, 80}, {150, 100}, {180, 150}, {190, 199}, {199, 190}}
	fmt.Println("Win probability for player-to-move, (myScore, oppScore) in raw points:")
	for _, s := range samples {
		me, opp := s[0], s[1]
		fmt.Printf("  (%5d, %5d): %.4f\n", me*unitSize, opp*unitSize, W[me][opp])
	}
}
