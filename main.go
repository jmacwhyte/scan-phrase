package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	bip39 "github.com/tyler-smith/go-bip39"
)

var showTestnet = false

var gfx = map[string]string{
	"start":      "╒═════════════════════════════════════╕\n",
	"phrase1":    "╒═══╧═════════════════════════════════╕\n",
	"phrase2":    "│   %s...   │\n", // Show first 28 characters of phrase
	"phrase3":    "╘═══╤═════════════════════════════════╛\n",
	"crypto":     "    ┝ %s : %s\n",
	"subcrypto1": "    │  ┝ %s : %s\n",
	"subcrypto2": "    │  ┕ %s : %s\n",
	"end":        "    ╘═══════════════════════════☐ Done!\n",
}

// Lastcall is used as a timestamp for the last api call (for rate limiting)
var lastcall = time.Now()

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

	fmt.Println()

	// Process each phrase
	for i, v := range phrases {

		// Prepare phrase
		p, err := NewPhrase(v)
		if err != nil {
			fmt.Println("phrase error: ", err)
			return
		}

		// Display phrase header
		if i == 0 {
			fmt.Printf(gfx["start"])
		} else {
			fmt.Printf(gfx["phrase1"])
		}
		fmt.Printf(gfx["phrase2"], v[:28])
		fmt.Printf(gfx["phrase3"])

		// Print each currency
		p.printBTCBalances("BTC", []BTCFormat{BTCFormat{Coin: "btc32", Type: "BIP32"}, BTCFormat{Coin: "btc44", Type: "BIP44"}})
		if showTestnet {
			p.printBTCBalances("TBT", []BTCFormat{BTCFormat{Coin: "tbt32", Type: "BIP32"}, BTCFormat{Coin: "tbt44", Type: "BIP44"}})
		}
		p.printBTCBalances("BCH", []BTCFormat{
			BTCFormat{Coin: "bch32", Type: "BIP32"},
			BTCFormat{Coin: "bch440", Type: "BIP44-coin0"},
			BTCFormat{Coin: "bch44145", Type: "BIP44-coin145"},
		})
		p.printETHBalances("ETH", false)
		if showTestnet {
			p.printETHBalances("TET", true)
		}

	}

	// End
	fmt.Printf(gfx["end"])
	fmt.Println()
}

type BTCFormat struct {
	Coin    string
	Type    string
	isUsed  bool
	balance float64
}

func (p Phrase) printBTCBalances(label string, coins []BTCFormat) {
	numused := 0
	for i, v := range coins {
		var err error
		coins[i].balance, coins[i].isUsed, _, err = p.LookupBTCBal(v.Coin)
		if err != nil {
			fmt.Printf(gfx["crypto"], "There was a problem with "+v.Coin, err)
		}

		if coins[i].isUsed {
			numused++
		}
	}

	var output string

	if numused == 0 {
		output = "Unused"
	} else {
		output = "** Used ** Balance: "
		done := 0
		for _, v := range coins {
			if v.isUsed {
				output += fmt.Sprintf("%.5f", v.balance) + label + " (" + v.Type + ")"
				done++
				if done < numused {
					output += ", "
				}
			}
		}
	}

	fmt.Printf(gfx["crypto"], label, output)
}

func (p Phrase) printETHBalances(label string, testnet bool) {

	addslice, err := p.LookupETH(testnet)
	if err != nil {
		fmt.Printf(gfx["crypto"], "There was a problem with "+label, err)
		return
	}

	add := addslice[0]

	var output string
	if add.TxCount == 0 {
		output = "Unused"
	} else {
		output = fmt.Sprintf("** Used ** Balance: %.5f%s", add.Balance, label)
	}

	fmt.Printf(gfx["crypto"], label, output)

	//If we had any tokens...
	if len(add.Tokens) == 0 {
		return
	}

	for i, v := range add.Tokens {
		output = fmt.Sprintf("%.5f%s", v.Balance, v.Ticker)

		if i < len(add.Tokens)-1 {
			fmt.Printf(gfx["subcrypto1"], v.Name, output)
		} else {
			fmt.Printf(gfx["subcrypto2"], v.Name, output)
		}
	}
}
