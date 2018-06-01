package main

import "strings"

type forest [][]rune

const treesPerPlayer = 5

func growForest(players map[string]*player) forest {
	//var trees = []rune("🌲🌳🌴🌵")
	var trees = []rune("🌲🌳")
	//var trees = []rune{'🌲'}

easterEgg:
	for n := range players { // santa trumps any request to play indoors
		name := strings.ToLower(n)
		if isSanta(name) {
			trees = []rune{'🎄'}
			break easterEgg
		}
		if strings.Contains(name, "indoor") || strings.Contains(name, "inside") {
			trees = []rune{'🚪'}
		}
	}

	totalTrees := len(players) * treesPerPlayer
	perRow := treesPerRow(totalTrees)
	rows := totalTrees/perRow
	if rows*perRow < totalTrees { rows++ } // we might end up with too many trees
	treesToRemove := rows*perRow - totalTrees

	f := make(forest, rows)

	for r := 0; r < rows; r++ { // make forest
		f[r] = randomLineOfTrees(perRow, trees)
	}

	for t := 0; t < treesToRemove; t++ { // remove the extra trees
		f[random.Intn(rows)][random.Intn(perRow)] = ' '
	}

	return f
}



func randomLineOfTrees(resultLength int, runesToPickFrom []rune) []rune {
	result := make([]rune, resultLength)
	n := len(runesToPickFrom)

	if n < 2 {
		for i := 0; i < resultLength; i++ {
			result[i] = runesToPickFrom[0]
		}
	} else {
		for i := 0; i < resultLength; i++ {
			result[i] = runesToPickFrom[random.Intn(n)]
		}
	}

	return result
}

// notes on treesPerRow:
//
// Our forest is a grid. Judging from my phone and my wife's phone,
// phones are typically a 1:2 rectangle (height is double the width).
// I want a grid as close to that as possible.
// NOTE! The return value will not necessarily evenly divide your
// number of trees. That was not a goal. For example, a forest with
// 30 trees will have 8 rows with 4 trees in the first 7 rows, and
// 2 straggler trees in the last row--not 6 rows with 5 each.
// I think 8 rows of 4 with an incomplete row satisfies our goal of a 1:2 rectangle. 

func treesPerRow(trees int) int { // this took a lot of tweeking and
	w := 0                         // testing to get it just right :P
	if trees < 50 { // a little fatter at the beginning
		for {
			if w*w*2 >= trees { return w }
			w++
		}
	} else {
		for {
			if w*w*2 == trees { return w }
			if w*w*2 > trees { return w-1 }
			w++
		}
	}
}

/* testing
func main() {
	for trees := 0; trees <= 200; trees++ {
		fmt.Printf("%3d   %2d\n", trees, treesPerRow(trees))
	}
}
*/




func populateForest(g *game) {
	for n := range g.players {
		g.players[n].col = -1
		g.players[n].row = -1
	}

	height := len(g.wood)
	width := len(g.wood[0])

	for n := range g.players {
		var col, row int
randomCoord:
		for {
			col = random.Intn(width)
			row = random.Intn(height)

			if g.wood[row][col] == ' ' { continue }
			for m := range g.players { // technically you're comparing against yourself, but it doesn't matter
				if col == g.players[m].col && row == g.players[m].row {
					continue randomCoord
				}
			}
			break
		}
		g.players[n].col = col
		g.players[n].row = row
	}
}

