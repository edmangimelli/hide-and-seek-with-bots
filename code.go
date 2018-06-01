package main

import (
	"errors"
	//"fmt"
)

const letters = "ACEFHJKMNPRTWXY"
const digits = "23456789"
const either = letters + digits
const maxCodes = len(letters)*len(either)*len(digits)*len(digits)
var codesProduced = 0

func newGameCode() (string, error) {
	if codesProduced == maxCodes { // see comments below
		return "", errors.New("All possible game codes are in use!")
	}

	var r = func(s string) string {
		return string(s[random.Intn(len(s))])
	}

	var randomCode string
	for {
		randomCode = r(letters)+r(either)+r(digits)+r(digits)
		if _, exists := games[randomCode]; !exists {
			break
		}
	}
	codesProduced++
	return randomCode, nil
}


/*


DESCRIPTION:

A game code is 4 characters long
(L = letter, D = digit, E = either letter or digit)
  LEDD
Certain letters and digits have been excluded as they
can be ambiguous depending on fonts, letter case, and
the characters to the left and right. For instance:
  lO01
is not a possible game code.
A game code can have at most 2 letters in a row (LE)
--I'm hoping that this will preclude any rude words
from being inside a code.




NOTE:
This code is less than ideal.

Assume: 

letters = "ACEFHJKMNPRTWXY"
 digits = "23456789"

and the format is LEDD

That gives you 22,080 unique game codes.
Now, assume that there are 22,075 games currently.
That means that 99.98% of the possible game codes have
been used, and that the random string generator only
has a 5 in 22080 chance (0.02%) of generating a usable
game code.

So--grossly inefficient. BUT, on a modern computer,
generating all 22,080 possible codes, in random order,
only takes around 150ms - 300ms. In fact, this is
exactly how I would sort a deck of cards on a TI-85
when I was a kid (and that /did/ take a while).

One idea for a faster generator would be to instead
generate a random number 0 -> 22079 (inclusive). And
also have a array of 22080 bools handy ([22080]bool).
When you generate a number, say, 273, if that index in
your array is false, make it true (code has been
generated) and then convert the number into our 4
character code.  Then, the next time through, if you
randomly generated 273 again, you go to 273 in the
array, see that it's true (used), and you simply go
forward in the array until you find an index that is
false, and use that number instead. This prevents the
constant collisions that occur with my algorithm above.

You can test the speed of the generator above with
the following code:
( just uncomment it, add "fmt" to the imports above,
  and `go run thisFile` )


var games = make(map[string]struct{})

func main() {
   start := time.Now()
   i := 0
   for {
      start := time.Now()
      code, err := newGameCode()
      stop := time.Now()
      if err != nil {
         break
      }
      games[code] = struct{}{}
      i++
      fmt.Printf("%5d  %s  (%fms elapsed)\n", i, code, float64(stop.Sub(start))/1000000)
   }
   stop := time.Now()
   fmt.Printf("%v elapsed\n", stop.Sub(start))
}

*/

