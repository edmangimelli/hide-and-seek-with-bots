package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var random *rand.Rand
func init() {
   source := rand.NewSource(time.Now().UnixNano())
   random = rand.New(source)
	log.Printf("             maximum number of games: %d\n", maxCodes)
}

type player struct {
	// round variables
	seeker bool
	found bool
	ready map[string]bool
	row, col int
	movesThisRound int

	// game variables
	connChan chan string
	emoji string
	waiting bool
	score int
	totalMoves int
	numberOfTimesHasBeenSeeker int
	numberOfTimesHasBeenHider int
	numberOfTimesHasEarnedSeeker int
}

type game struct {
	wood forest
	players map[string]*player
	inRound bool // false = seeker hasn't started the game
	round int
	usedEmojis [][]bool
	santaInUse bool
	multiHiderRound bool
	firstReadyForNextSetupRcvd bool
	firstReadyToGoRcvd bool
	bootCancels []*bool
}

var games = make(map[string]*game, 0)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var mutex = sync.Mutex{}

const (
	active = iota
	found = iota
	waiting = iota
	waitingAndFound = iota
)

func main() {
	http.HandleFunc("/socket", func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)

		connChan := make(chan string)
		code, emoji, name := "", "", conn.RemoteAddr().String()

		conn.SetCloseHandler(func(codeNumber int, text string) error { // PLAYER LEAVES
			mutex.Lock()
			closeHandler(code, name)
			mutex.Unlock()
			return nil
		})


		go func () { // *** Receive messages from client (external)
			for {
				_, rawMsg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				log.Printf("\n✉ message received from %s/%s:\n%s\n", code, name, string(rawMsg))
				msg := strings.Split(string(rawMsg), "\n")

				switch msg[0] { // 6 message types can be received:

				case "good bye":
					sendMsg(conn, code, name, "bye!")
					conn.Close()
					mutex.Lock()
					closeHandler(code, name)
					mutex.Unlock()

				case "join": // code // name
					mutex.Lock()

					if _, exists := games[msg[1]]; !exists {
						sendMsg(conn, code, name, fmt.Sprintf("no such game\n%s", msg[1]))
						mutex.Unlock()
						break
					}

					if _, exists := games[msg[1]].players[msg[2]]; exists {
						sendMsg(conn, code, name, fmt.Sprintf("name is taken\n%s", msg[2]))
						mutex.Unlock()
						break
					}

					code = msg[1]
					name = msg[2]
					emoji = randomEmoji(games[code], name)
					games[code].players[name] = &player{
						emoji: emoji,
						waiting: games[code].inRound,
						connChan: connChan,
						row: -1,
						col: -1,
						ready: make(map[string]bool),
					}
					games[code].players[name].ready["ready to go"] = false
					games[code].players[name].ready["ready for next setup"] = false
					log.Printf("\nplayer has joined: %s/%s\n", code, name)

					for n, p := range games[code].players { // tell other players
						if n != name { // don't need to send the message to yourself
							p.connChan <- fmt.Sprintf("joined\n%s\n%s", emoji, name)
						}
					}

					var reply string
					if games[code].inRound {
						reply = "wait for next round"
					} else {
						reply = "wait for start"
					}
					reply += fmt.Sprintf("\n%s\n%s\n%s", code, emoji, name)
					for n := range games[code].players {
						if n != name {
							reply += fmt.Sprintf("\n%s\n%s", games[code].players[n].emoji, n)
						}
					}

					mutex.Unlock()
					sendMsg(conn, code, name, reply)


				case "move to": // row // col
					row, _ := strconv.Atoi(msg[1])
					col, _ := strconv.Atoi(msg[2])

					mutex.Lock()
					//if games[code].wood[row][col] == ' ' { // can't move to non-tree
					//	log.Printf("\ncan't move there. no tree.\n")
					//	mutex.Unlock()
					//	break
					//}

					occ := occupant(row, col, games[code])

					if games[code].players[name].seeker {

						if occ != "" {
							games[code].players[occ].found = true
							winner := reportWinnerIfThereIsOne(games[code])
							if winner != "" {
								games[code].players[name].movesThisRound++
								games[code].players[name].totalMoves++
								if games[code].multiHiderRound {
									games[code].players[winner].score++
								}
								mutex.Unlock()
								break
							} else {
								for _, p := range games[code].players { // tell non-waiting players
									if p.waiting { continue }
									p.connChan <- fmt.Sprintf("found\n%s\n%s\n%d\n%d", games[code].players[occ].emoji, occ, row, col)
								}
							}
						}

						for _, p := range games[code].players { // tell non-waiting players
							if p.waiting { continue }
							p.connChan <- fmt.Sprintf("moved\n%s\nfrom\n%d\n%d\nto\n%d\n%d", emoji, games[code].players[name].row, games[code].players[name].col, row, col)
						}

						games[code].players[name].movesThisRound++
						games[code].players[name].totalMoves++
						games[code].players[name].row = row
						games[code].players[name].col = col

					} else { // hider
						if occ != "" {
							mutex.Unlock()
							break
						}

						for _, p := range games[code].players { // tell non-waiting players
							if p.waiting { continue }
							p.connChan <- fmt.Sprintf("moved\n%s\nfrom\n%d\n%d\nto\n%d\n%d", emoji, games[code].players[name].row, games[code].players[name].col, row, col)
						}

						games[code].players[name].movesThisRound++
						games[code].players[name].totalMoves++
						games[code].players[name].row = row
						games[code].players[name].col = col
					}

					mutex.Unlock()

				case "new game": // name
					name = msg[1]
					log.Printf("\n?/%s is trying to initialize new game.\n", name)

					mutex.Lock()
					var err error
					code, err = newGameCode() // make new game
					if err != nil {
						sendMsg(conn, "?", name, "too many games in session")
						mutex.Unlock()
						break
					}

					games[code] = &game{
						players: make(map[string]*player),
						usedEmojis: make([][]bool, len(emojis)),
					}
					for i := range games[code].usedEmojis {
						games[code].usedEmojis[i] = make([]bool, len(emojis[i]))
					}
					log.Printf("\nnew game created: %s\n", code)

					emoji = randomEmoji(games[code], name)
					games[code].players[name] = &player{
						emoji: emoji,
						seeker: true,
						connChan: connChan,
						row: -1,
						col: -1,
						ready: make(map[string]bool),
					}
					games[code].players[name].ready["ready to go"] = false
					games[code].players[name].ready["ready for next setup"] = false

					mutex.Unlock()
					log.Printf("\nplayer has joined: %s/%s\n", code, name)

					sendMsg(conn, code, name, fmt.Sprintf("game initialized\n%s\n%s\n%s", code, emoji, name))

				case "ready to go":
					readyMsgs(msg[0], code, name, &(games[code].firstReadyToGoRcvd), func() {
						for _, p := range games[code].players {
							p.ready[msg[0]] = false
							p.connChan <- "go!"
						}
					})

				case "ready for next setup":
					readyMsgs(msg[0], code, name, &(games[code].firstReadyForNextSetupRcvd), func() {
						newSetup(games[code])
					})

				case "remove tree": // row // col
					mutex.Lock()
					for _, p := range games[code].players { // tell non-waiting players 
						if p.waiting { continue }
						p.connChan <- string(rawMsg)
					}
					mutex.Unlock()
				case "start":
					mutex.Lock()
					games[code].inRound = true
					newSetup(games[code])
					mutex.Unlock()

				} // switch end
			}
		}()

		// *** Receive messages from other players (internal)
		for {
			rawMsg := <-connChan
			//msg := strings.Split(string(rawMsg), "\n")
			log.Printf("\n%s/%s: internal msg received:\n%s\n", code, name, rawMsg)

			switch string(rawMsg) { // most of these simply relay the msg to the client
			case "close":
				conn.Close()
				closeHandler(code, name)
			default:
				sendMsg(conn, code, name, rawMsg)
			}
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "client.html")
	})

	http.ListenAndServe(":8080", nil)

}

func sendMsg(conn *websocket.Conn, code string, name string, msg string) error {
	if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		log.Printf("\nconn.WriteMessage failed:\nconnection:\n%s\nto: %s/%s\nmsg:\n%s\n", conn.RemoteAddr().String(), code, name, msg)
		return err
	}
	log.Printf("\n📝 message sent to %s/%s:\n%s\n", code, name, msg)
	return nil
}

func closeHandler(code, name string) { // NO MUTEX
	if name == "" || code == "" { return }

	row := games[code].players[name].row
	col := games[code].players[name].col
	emoji := games[code].players[name].emoji
	wasSeeker := games[code].players[name].seeker
	gonePlayer := profilePlayer(games[code].players[name])

	delete(games[code].players, name)
	log.Printf("\nPlayer deleted: %s/%s\n", code, name)

	actives, founds, waitings, waitingAndFounds := profilePlayers(games[code])
	totalPlayers := actives + founds + waitings + waitingAndFounds
	if waitingAndFounds > 0 {
		log.Printf("\nBUG: some players are both waiting and found.\n")
	}
	waitings += waitingAndFounds

	if len(games[code].players) == 0 {
		delete(games, code)
		log.Printf("\nGame deleted: %s\n", code)
		return
	}

	if !games[code].inRound {
		if wasSeeker {
			_, p := randomlyAppointSeeker(games[code])
			p.connChan <- "seeker left\nyou are now seeker"
		} else {
			for _, p := range games[code].players {
				p.connChan <- fmt.Sprintf("left\n%s\n%s", emoji, name)
			}
		}
		return
	}

	switch gonePlayer {
	case active:
		if wasSeeker { // seeker left
			_, seeker := randomlyAppointSeeker(games[code])
			if totalPlayers == 1 {
				seeker.connChan <- "round over\nseeker left\nyou are now seeker\ntoo few hiders to start next round"
				games[code].inRound = false
			} else {
				for _, p := range games[code].players {
					if p.seeker {
						p.connChan <- "round over\nseeker left\nyou are now seeker"
					} else {
						p.connChan <- "round over\nseeker left"
					}
				}
			}
		} else { // hider left
			switch actives {
			case 0: // should be an impossible case
				log.Printf("\nBUG: Impossible case. Round continued with 1 active player and then they left.\n")
				if totalPlayers > 1 {
					for _, p := range games[code].players {
						p.seeker = false
					}
					randomlyAppointSeeker(games[code])
					for _, p := range games[code].players {
						if p.seeker {
							p.connChan <- "round over\nseeker left\nyou are now seeker"
						} else {
							p.connChan <- "round over\nseeker left"
						}
					}
				} else {
					for _, p := range games[code].players {
						p.seeker = true
						p.connChan <- "round over\nseeker left\nyou are now seeker\ntoo few hiders to start next round"
					}
					games[code].inRound = false
				}
			case 1: // seeker is alone
				if (founds + waitings) > 0 {
					for _, p := range games[code].players {
						p.connChan <- "round over\ntoo few hiders"
					}
					// note: seeker does not change
				} else {
					for _, p := range games[code].players { // only 1 player
						p.connChan <- "round over\ntoo few hiders\ntoo few hiders to start next round"
					}
					games[code].inRound = false
				}
			default: // seeker is still in the round, and there's at least 1 hider
				winner := reportWinnerIfThereIsOne(games[code])
				if winner == "" { // there may be an automatic winner (multiHiderRound and only 1 hider left)
					for _, p := range games[code].players {
						p.connChan <- fmt.Sprintf("left\n%s\n%s\n%d\n%d", emoji, name, row, col)
					}
				}
			}
		}
	case found, waiting, waitingAndFound:
		for _, p := range games[code].players {
			p.connChan <- fmt.Sprintf("left\n%s\n%s", emoji, name)
		}
	}
	return
}

func readyMsgs(desc string, code string, name string, firstReadyMsgRcvd *bool, f func()) {
	mutex.Lock()

	if !*firstReadyMsgRcvd {
		*firstReadyMsgRcvd = true
		log.Printf("\n%s: first \"%s\" msg received.\n", code, desc)
		cancelBoot := new(bool)
		*cancelBoot = false
		time.AfterFunc(10*time.Second, func() {
			mutex.Lock()
			bootNotReadyPlayers(desc, code, firstReadyMsgRcvd, cancelBoot, f)
			mutex.Unlock()
		})
		games[code].bootCancels = append(games[code].bootCancels, cancelBoot)
		printBootCancels(code)
	}

	games[code].players[name].ready[desc] = true

	if everyonesReady(desc, games[code]) {
		log.Printf("\n%s: everyone responded \"%s\".\n      booting not-ready players CANCELLED.\n", code, desc)
		last := len(games[code].bootCancels) - 1
		*(games[code].bootCancels[last]) = true
		*firstReadyMsgRcvd = false
		f()
	}

	mutex.Unlock()
}

func bootNotReadyPlayers(desc string, code string, firstReadyMsgRcvd *bool, cancelBoot *bool, f func()) {
	printBootCancels(code)
	if *cancelBoot {
		log.Printf("\n%s: bootNotReadyPlayers: cancelled. (%s)\n", code, desc)
		games[code].bootCancels = games[code].bootCancels[1:]
		printBootCancels(code)
		return
	}
	log.Printf("\n%s: bootNotReadyPlayers: BOOTING. (%s)\n", code, desc)

	oldNumberOfPlayers := len(games[code].players)
	newNumberOfPlayers := oldNumberOfPlayers
	for n, p := range games[code].players {
		if !p.ready[desc] {
			newNumberOfPlayers--
			log.Printf("\nbooting: %s/%s\n", code, n)
			p.connChan <- "close"
		}
	}
	// idle as players are deleted
	for len(games[code].players) != newNumberOfPlayers {
		time.Sleep(500 * time.Millisecond)
	}
	f()

	*firstReadyMsgRcvd = false
	games[code].bootCancels = games[code].bootCancels[1:]
	printBootCancels(code)
}

func printBootCancels(code string) {
	result := fmt.Sprintf("%s: current bootCancels = [", code)
	for i, p := range games[code].bootCancels {
		if i > 0 { result += ", " }
		result += strconv.FormatBool(*p)
	}
	result += "]"
	log.Printf("\n%s\n", result)
}

func seekerEmoji(g *game) string {
	for n := range g.players {
		if g.players[n].seeker {
			return g.players[n].emoji
		}
	}
	return ""
}

func occupant(row int, col int, g *game) string {
	for n := range g.players {
		if !g.players[n].found && !g.players[n].waiting && g.players[n].row == row && g.players[n].col == col {
				return n
		}
	}
	return ""
}

func onlyOneHiderLeft(g *game) string {
	notFound := 0
	last := ""

	for n, p := range g.players {
		if !p.seeker && !p.found && !p.waiting {
			notFound++
			last = n
		}
	}

	if notFound == 1 {
		return last
	} else {
		return ""
	}
}

func everyonesReady(desc string, g *game) bool {
	for _, p := range g.players {
		if !p.ready[desc] {
			return false
		}
	}
	return true
}

func everyonesFound(g *game) bool {
	for _, p := range g.players {
		if p.seeker || p.waiting { continue }
		if !p.found {
			return false
		}
	}
	return true
}

func numberOfWaitingToJoinPlayers(g *game) int {
	waiting := 0
	for _, p := range g.players {
		if p.waiting { waiting++ }
	}
	return waiting
}

func newSetup(g *game) {

	if len(g.players) < 2 {
		for _, p := range g.players { // tell only player
			p.connChan <- "too few hiders"
			p.seeker = true // not sure if this is redundant
		}
		return
	}

	g.multiHiderRound = len(g.players) > 2

	//if there's no seeker (seeker left)
	if noSeeker(g) { randomlyAppointSeeker(g) }

	g.wood = growForest(g.players)

	populateForest(g) // everyone's given a random row and col

	/* DEBUG
	for _, s := range g.wood {
		log.Println(string(s))
	}
	for n, p := range g.players {
		log.Printf("%s (%d, %d)", n, p.x, p.y)
	}
	*/

	reply := fmt.Sprintf("setup\nseeker %s", seekerEmoji(g))

	reply += fmt.Sprintf("\nforest\n%d\n", len(g.wood[0]))
	for _, treeLine := range g.wood {
		reply += string(treeLine)
	}

	for n, p := range g.players {
		p.found = false;
		p.ready["ready for next setup"] = false;
		p.waiting = false;
		p.movesThisRound = 0
		if p.seeker {
			p.numberOfTimesHasBeenSeeker++
		} else {
			p.numberOfTimesHasBeenHider++
		}
		reply += fmt.Sprintf("\n%s\n%s\n%d\n%d\n%d", p.emoji, n, p.row, p.col, p.score)
	}

	for _, v := range g.players { // tell everyone
		v.connChan <- reply
	}

	return
}

func noSeeker(g *game) bool {
	s, seeker, seekers := 0, "", ""

	for n, p := range g.players {
		if p.seeker {
			s++
			seeker = n
			seekers += fmt.Sprintf("%s\n", n)
		}
	}

	switch {
	case s > 1:
		log.Fatalf("CRASH: too many seekers!\n%s", seekers)
	case s == 0:
		return true
	}

	if g.players[seeker].waiting {
		log.Fatalf("CRASH: seeker is waiting to join!\n%s", seeker)
	}
	return false
}

func randomlyAppointSeeker(g *game) (string, *player) {
	log.Println("Randomly appointing seeker!")
	r := random.Intn(len(g.players))
	for n, p := range g.players {
		if r == 0 {
			p.seeker = true
			return n, p
		}
		r--
	}
	return "", nil // this line will never be reached
}

func reportWinnerIfThereIsOne(g *game) string {

	if g.multiHiderRound {
		last := onlyOneHiderLeft(g)
		if last != "" {
			for _, p := range g.players {
				if p.seeker { p.seeker = false }
				p.connChan <- fmt.Sprintf("winner\n%s\n%s", g.players[last].emoji, last)
			}
			g.players[last].seeker = true
			return last
		}
	} else {
		if everyonesFound(g) {
			var seeker, hider string
			for n, p := range g.players {
				if p.seeker { seeker = n }
				if p.found  { hider  = n }
				p.connChan <- "round over\n2 player game"
			}
			g.players[seeker].seeker = false
			g.players[hider].seeker = true
			return hider
		}
	}
	return ""
}

func profilePlayer(p *player) int {
	switch {
	case !p.found && !p.waiting:
		return active
	case !p.found &&  p.waiting:
		return waiting
	case  p.found && !p.waiting:
		return found
	}
	//    p.found &&  p.waiting:
	return waitingAndFound
}

func profilePlayers(g *game) (int, int, int, int) {
	var actives, waitings, founds, waitingAndFounds int
	for _, p := range g.players {
		switch profilePlayer(p) {
		case active:
			actives++
		case waiting:
			waitings++
		case found:
			founds++
		case waitingAndFound:
			waitingAndFounds++
		}
	}
	return actives, founds, waitings, waitingAndFounds
}
