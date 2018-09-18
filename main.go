package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	bip39 "github.com/tyler-smith/go-bip39"
)

var gfx = map[string]string{
	"head1":    "╒═══════════════════════════════════╤═════════════╤═════════════╤═════════════╤═════════════╕",
	"head2":    "│               Phrase              │     BTC     │    T.BTC    │     BCH     │     ETH     │",
	"div":      "╞═══════════════════════════════════╪═════════════╪═════════════╪═════════════╪═════════════╡",
	"ercstart": "╞═══════════════════════════════════╧═════════════╧═════════════╧═════════════╛             │",
	"ercend":   "╞═══════════════════════════════════╤═════════════╤═════════════╤═════════════╤═════════════╡",
	"end":      "╘═══════════════════════════════════╧═════════════╧═════════════╧═════════════╧═════════════╛",
}

// Lastcall is used as a timestamp for the last api call (for rate limiting)
var lastcall = time.Now()

// Save lists of addresses for verbose logging if in single address mode
var addlists = make(map[string][]Address)

func main() {

	var phrases []string

	// If a phrase is provided, load that in as the only one. Otherwise, load up the "phrases.txt" file.
	// All phrases will later be validated by the phrase library
	if len(os.Args) == 13 {
		var phrase string
		for i, v := range os.Args[1:] {
			phrase += v
			if i < 11 {
				phrase += " "
			}
		}
		phrases = []string{phrase}
	} else {
		if _, e := os.Stat("phrases.txt"); os.IsNotExist(e) {
			fmt.Println("Please create a phrases.txt file or call this command followed by a valid 12-word phrase.")
			return
		}

		data, err := ioutil.ReadFile("phrases.txt")
		if err != nil {
			fmt.Println("data load error: ", err)
		}

		splits := strings.Split(string(data), "\n")
		for _, v := range splits {
			// Skip any invalid phrases so we can number them accurately in the UI
			if bip39.IsMnemonicValid(v) {
				phrases = append(phrases, v)
			}
		}
	}

	// Write out the UI header
	fmt.Println()
	fmt.Println(gfx["head1"])
	fmt.Println(gfx["head2"])
	fmt.Println(gfx["div"])

	// Process each phrase
	for i, v := range phrases {

		// Record which ones have been used, then look up balances
		currencies := []string{"btc", "tbt", "bch", "eth"}
		firstrun := make(map[string]Address)

		// Prepare phrase
		p, err := NewPhrase(v)
		if err != nil {
			fmt.Println("phrase error: ", err)
			return
		}

		// Display address (width incl. pipes: 37)
		fmt.Printf("│ #%-4v %s... │", strconv.Itoa(i+1), v[:24])

		// Lookup which currencies have been used
		for _, v := range currencies {

			var addr []Address
			var err error
			switch v {
			case "btc":
				addr, err = p.LookupBTC(0, 0, 1, false)
			case "tbt":
				addr, err = p.LookupBTC(0, 0, 1, true)
			case "bch":
				addr, err = p.LookupBCH(0, 0, 1)
			case "eth":
				addr, err = p.LookupETH(false)
			case "tet":
				addr, err = p.LookupETH(true)
			}
			if err != nil {
				fmt.Println("lookup error: ", err)
				return
			}

			if addr[0].TxCount > 0 {
				fmt.Print(centerText("Used", 13))
			} else {
				fmt.Print(centerText("Not used", 13))
			}

			firstrun[v] = addr[0]
			fmt.Print("│")
			time.Sleep(time.Millisecond * 100)
		}

		fmt.Printf("\n│                                   │")

		// Lookup balances
		for _, v := range currencies {
			if firstrun[v].TxCount > 0 || firstrun[v].Balance > 0 {

				var bal float64
				if v == "eth" {
					// No need to do all that again
					bal = firstrun[v].Balance
				} else {
					var err error
					bal, addlists[v], err = p.LookupBTCBal(v)
					if err != nil {
						fmt.Println("full bal lookup error: ", err)
						return
					}
				}

				fmt.Print(centerBalance(bal, 13))
				fmt.Print("│")
			} else {
				fmt.Print("             │")
			}
		}

		// End of balance row
		fmt.Printf("\n")

		//If we had any tokens...
		if len(firstrun["eth"].Tokens) > 0 {
			fmt.Println(gfx["ercstart"])

			for _, v := range firstrun["eth"].Tokens {
				fmt.Println("│" + strings.Repeat(" ", 40) + centerText(v.Name, 20) + ":" + rightBalance(v.Balance, 15) + " " + v.Ticker + strings.Repeat(" ", 11) + "│")
			}

			fmt.Println(gfx["ercend"])

		} else if i < len(phrases)-1 {
			// Start a new line if this isn't the last phrase
			fmt.Println(gfx["div"])
		}
	}

	// End
	fmt.Println(gfx["end"])
	fmt.Println()

	if len(os.Args) > 1 {
		// Show some verbose balance logging if only looking up a single phrase
		listAddBals(addlists)
	}

	fmt.Println(centerText("You're welcome!", 76))
	fmt.Println()
}

// Center some text inside a given width
func centerText(label string, width int) string {
	if len(label) >= width {
		return label[:width]
	}
	l := len(label)
	n := (width - l) / 2

	return strings.Repeat(" ", n+((width-len(label))%2)) + label + strings.Repeat(" ", n)
}

// Center a number (balance) inside a given width
func centerBalance(amount float64, width int) string {
	return centerText(fmt.Sprintf("%.8g", amount), width)
}

// Right-align a number (balance) inside a given width
func rightBalance(amount float64, width int) string {
	v := fmt.Sprintf("%f", amount)
	if len(v) >= width {
		return v[:width]
	}

	return strings.Repeat(" ", width-len(v)) + v
}

func listAddBals(adds map[string][]Address) {
	order := []string{"btc", "tbt", "bch"}

	for _, cur := range order {

		var line string
		for i, v := range adds[cur] {
			if v.Balance > 0 {
				line += fmt.Sprintf("Child %d (%s) balance: %f\n", i, v.Address, v.Balance)
			}
		}

		if line != "" {
			fmt.Printf("\n  %s:\n%s\n", strings.ToUpper(cur), line)
		}
	}
}
