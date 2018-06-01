package main

import (
	"strings"
	"log"
)

const santa = '🎅'
var emojis = [][]rune{[]rune("😛👽💩🤖👾👻😸🙈👶🐶🦁🐴🦄🐮🐷⛄🎃🌛🐐🐪🐘🐭🐰🐿🐨🐼🐔🐣🐧🕊🐸🐊🐢🐍🐳🐟🐡🐙🦀🐌🐜🐝🐞🕷"), []rune("🐚⛷🚣🏎👌👃💋🕶🎒👟👑🎓💎🍇🍉🍋🍍🍎🍓🍅🍄🍞🧀🍔🍟🍕🌭🍿🍦🍩🍪🎂🍫🍭☕🍽🗽🎠💈🚂🚌🚲🛢⚓⏰☂🎈📖🕯💡📷📺💾☎🎷🔔🏐🔮🎮🎲📡💼📬☯⚛🏁"), []rune("🂡🂢🂣🂤🂥🂦🂧🂨🂩🂪🂫🂭🂮🂱🂲🂳🂴🂵🂶🂷🂸🂹🂺🂻🂽🂾🃁🃂🃃🃄🃅🃆🃇🃈🃉🃊🃋🃍🃎🃑🃒🃓🃔🃕🃖🃗🃘🃙🃚🃛🃝🃞🂿"), []rune("🁣🁤🁥🁦🁧🁨🁩🁪🁫🁬🁭🁮🁯🁰🁱🁲🁳🁴🁵🁶🁷🁸🁹🁺🁻🁼🁽🁾🁿🂀🂁🂂🂃🂄🂅🂆🂇🂈🂉🂊🂋🂌🂍🂎🂏🂐🂑🂒🂓"), []rune("①②③④⑤⑥⑦⑧⑨⑩⑪⑫⑬⑭⑮⑯⑰⑱⑲⑳")}

var maxPlayersPerGame int

func init() {
	for _, set := range emojis {
		maxPlayersPerGame += len(set)
	}
	log.Printf("  maximum number of players per game: %v\n", maxPlayersPerGame)
}

func randomEmoji(g *game, name string) string {

	name = strings.ToLower(name)
	if isSanta(name) && !g.santaInUse {
		g.santaInUse = true
		return string(santa)
	}

	grabFromSet := func(i int) rune {
		len := len(emojis[i])
		if len == 1 { // the loop below doesn't work if len is 1
			if g.usedEmojis[i][0] {
				return rune(0)
			}
			g.usedEmojis[i][0] = true
			return emojis[i][0]
		}

		r := random.Intn(len)
		startingPoint := r
		for g.usedEmojis[i][r] { // starting at r, cycle through runes
			r++
			if r == startingPoint {
				return rune(0);
			}
			if r == len {
				r = 0
			}
		}
		g.usedEmojis[i][r] = true
		return emojis[i][r]
	}

	var r rune
	for i := 0; i < len(emojis) && r == 0; i++ {
		r = grabFromSet(i)
	}

	return string(r) // this could fail if server.go isn't making sure we don't max out on players
}

func isSanta(name string) bool {
	// ! This assumes toLower has already been done on string
	switch name {
	case "santa", "santa claus", "father christmas", "father xmas", "saint nicholas", "st. nicholas", "saint nick", "st. nick", "kris kringle", "kringle":
		return true
	default:
		return false
	}
}
