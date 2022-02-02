package main

//TODO: add in checking for bip44 btc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
)

// Phrase represents a phrase we are examining
type Phrase struct {
	master string
	xprv   *hdkeychain.ExtendedKey
}

// Address represents a crypto address generated from a phrase, including details about how/if it has been used
type Address struct {
	Address string
	TxCount int
	Balance float64
	IsTest  bool
	Tokens  []Token
}

// Token represents a balance of ERC20 tokens tied to an Ethereum address. Name is the ticker of the token, and
// Address is the contract address for the token.
type Token struct {
	Name    string
	Ticker  string
	Address string
	Balance float64
	TxCount int
}

// NewPhrase returns a phrase object with the master key generated (and the eth address, since that is static)
func NewPhrase(phrase string) (p *Phrase, err error) {
	p = new(Phrase)

	// Populate our BIP39 seed
	seed := bip39.NewSeed(phrase, "")
	masterKey, _ := bip32.NewMasterKey(seed)
	p.master = masterKey.String()

	// Get our master xprv
	// Path: m
	p.xprv, err = hdkeychain.NewKeyFromString(p.master)

	return
}

// getBitcoinAddress by specifying the chain number (0 for normal, 1 for change), child number (address number),
// and whether testnet or not. Count specifies how many addresses to return.
func (p Phrase) getBitcoinAddresses(purpose uint32, coin uint32, chainNo uint32, childNo uint32, count int, testnet bool) (addresses []*Address) {
	if count < 1 {
		count = 1
	}

	if count > 100 {
		count = 100
	}

	for i := 0; i < count; i++ {
		// Path: m/0H/[chain]/[child] (e.g. m/0H/0/0)
		child, err := deriveHDKey(p.xprv, purpose, coin, 0, chainNo, childNo)
		if err != nil {
			fmt.Println("Uh-oh! HD derivation error:", err)
			return
		}

		// Generate address based on testnet or mainnet
		params := &chaincfg.MainNetParams

		if testnet {
			params = &chaincfg.TestNet3Params
		}

		pkh, err := child.Address(params)
		if err != nil {
			fmt.Println("Uh-oh! HD derivation error:", err)
			return
		}
		addresses = append(addresses, &Address{Address: pkh.EncodeAddress()})

		childNo++
	}

	return
}

// LookupBTC takes a slice of addresses and fills in the details
func (p Phrase) LookupBTC(addresses []*Address, isTestnet bool) (err error) {

	domain := ""
	if isTestnet {
		domain = "testnet."
	}

	var addylist string

	for i, v := range addresses {
		if isTestnet {
			addresses[i].IsTest = true
		}
		addylist += v.Address + ","
	}

	var BCi map[string]struct {
		Balance  int64 `json:"final_balance"`
		TxCount  int   `json:"n_tx"`
		Received int64 `json:"total_received"`
	}

	err = callAPI("https://"+domain+"blockchain.info/balance?active="+addylist, &BCi, btcRate)
	if err != nil {
		return
	}

	for i, v := range addresses {
		addresses[i].TxCount = BCi[v.Address].TxCount
		addresses[i].Balance = float64(float64(BCi[v.Address].Balance) / 100000000)
	}
	return
}

// LookupBCH takes a slice of addresses and fills in the details
func (p Phrase) LookupBCH(addresses []*Address) (err error) {

	// NOTE: the API no longer supports looking up multiple addresses at once, so we will have to do them one at a time

	for i, v := range addresses {
		var BTCcom struct {
			Error int `json:"err_no"`
			Data  struct {
				Address  string `json:"address"`
				Balance  int64  `json:"balance"`
				TxCount  int    `json:"tx_count"`
				Received int64  `json:"received"`
			} `json:"data"`
		}

		url := "https://bch-chain.api.btc.com/v3/address/" + v.Address
		err = callAPI(url, &BTCcom, btcRate)
		if err != nil {
			return
		}

		if BTCcom.Error != 0 {
			err = errors.New("BTC.com error: " + strconv.Itoa(BTCcom.Error))
			return
		}

		// We'll stop at the first unused address
		if BTCcom.Data.TxCount == 0 {
			return
		}

		addresses[i].TxCount = BTCcom.Data.TxCount
		addresses[i].Balance = float64(float64(BTCcom.Data.Balance) / 100000000)
	}
	return
}

// LookupBTCBal follows the entire btc/bch chain and finds out the remaining balance for the entire wallet.
func (p Phrase) LookupBTCBal(coin string) (balance float64, isUsed bool, addresses []*Address, err error) {

	batch := 50 // How many addresses to fetch at one time
	skips := 0  // How many empty addresses in a row we've found

	// chain 0 = main, chain 1 = change
	for chain := uint32(0); chain < 2; chain++ {

		child := uint32(0)

		for skips < 10 { // Go until we find 10 in a row that are unused
			var addr []*Address
			switch coin {
			case "btc32":
				addr = p.getBitcoinAddresses(0, 0, chain, child, batch, false)
				err = p.LookupBTC(addr, false)
			case "btc44":
				addr = p.getBitcoinAddresses(44, 0, chain, child, batch, false)
				err = p.LookupBTC(addr, false)
			case "tbt32":
				addr = p.getBitcoinAddresses(0, 0, chain, child, batch, true)
				err = p.LookupBTC(addr, true)
			case "tbt44":
				addr = p.getBitcoinAddresses(44, 1, chain, child, batch, true)
				err = p.LookupBTC(addr, true)
			case "bch32":
				addr = p.getBitcoinAddresses(0, 0, chain, child, batch, false)
				err = p.LookupBCH(addr)
			case "bch440":
				addr = p.getBitcoinAddresses(44, 0, chain, child, batch, false)
				err = p.LookupBCH(addr)
			case "bch44145":
				addr = p.getBitcoinAddresses(44, 145, chain, child, batch, false)
				err = p.LookupBCH(addr)
			}

			for _, v := range addr {
				balance += v.Balance
				if v.TxCount > 0 {
					isUsed = true
					skips = 0
				} else {
					skips++
				}
			}

			addresses = append(addresses, addr...)

			child += uint32(batch)
		}
	}
	return
}

// Derive a key for a specific location in a BIP44 wallet
func deriveHDKey(xprv *hdkeychain.ExtendedKey, purpose uint32, coin uint32, account uint32, chain uint32, address uint32) (pubkey *hdkeychain.ExtendedKey, err error) {

	// Path: m/44H
	purp, err := xprv.Derive(hdkeychain.HardenedKeyStart + purpose)
	if err != nil {
		return
	}

	if purpose == 0 {
		// This is a bip32 path
		// Path: m/0H/[chain]
		var cha *hdkeychain.ExtendedKey
		cha, err = purp.Derive(chain)
		if err != nil {
			return
		}

		// Path: m/0H/[chain]/[child] (e.g. m/0H/0/0)
		pubkey, err = cha.Derive(address)
		return
	}

	// Coin number
	// Path: m/44H/60H
	co, err := purp.Derive(hdkeychain.HardenedKeyStart + coin)
	if err != nil {
		return
	}

	// Path: m/44H/60H/0H
	acc, err := co.Derive(hdkeychain.HardenedKeyStart + account)
	if err != nil {
		return
	}

	// Path: m/44H/60H/0H/0
	cha, err := acc.Derive(chain)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Path: m/44H/60H/0H/0/0
	pubkey, err = cha.Derive(address)
	if err != nil {
		fmt.Println(err)
		return
	}

	return
}

func callAPI(url string, target interface{}, rate int64) (err error) {

	prnt := fmt.Sprintf("Please wait: %s...%s", url[8:30], url[len(url)-10:])
	fmt.Printf("%s\r", prnt)

	//rate limit
	time.Sleep(time.Until(lastcall.Add(time.Millisecond * time.Duration(rate))))

	var res *http.Response
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res, err = client.Get(url)
	if err != nil {
		return
	}

	var data []byte
	data, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(data, target)
	if err != nil {
		return errors.New("Invalid server response: " + string(data))
	}
	lastcall = time.Now()

	fmt.Printf("%s\r", strings.Repeat(" ", len(prnt)))
	return
}

// Truncate any eth value to 8 decimals of precision (make large numbers easier)
func snipEth(input string, decimal int) (output float64, err error) {
	tocut := decimal - 8

	if tocut > 0 && len(input)-tocut > 0 {
		start := len(input) - tocut
		input = input[:start]
		decimal = 8
	}

	output, err = strconv.ParseFloat(input, 64)
	if decimal > 0 {
		output = output / math.Pow(10, float64(decimal))
	}
	return
}
