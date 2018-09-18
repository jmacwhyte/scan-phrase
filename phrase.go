package main

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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
)

// Phrase represents a phrase we are examining
type Phrase struct {
	master     string
	brdBtc     *hdkeychain.ExtendedKey
	brdEthAddr string
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
	xprv, err := hdkeychain.NewKeyFromString(p.master)
	if err != nil {
		return
	}

	// Set up our BIP32 bitcoin path
	// Path: m/0H
	p.brdBtc, err = xprv.Child(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return
	}

	// Set up our BIP44 Eth path
	// Path: m/44H
	purpose, err := xprv.Child(hdkeychain.HardenedKeyStart + 44)
	if err != nil {
		return
	}

	// Path: m/44H/60H
	coin, err := purpose.Child(hdkeychain.HardenedKeyStart + 60)
	if err != nil {
		return
	}

	// Path: m/44H/60H/0H
	account, err := coin.Child(hdkeychain.HardenedKeyStart)
	if err != nil {
		return
	}

	// Path: m/44H/60H/0H/0
	change, err := account.Child(0)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Path: m/44H/60H/0H/0/0
	addidx, err := change.Child(0)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Generate our Ethereum address
	xpub, err := addidx.ECPubKey()
	if err != nil {
		fmt.Println(err)
		return
	}

	pubBytes := xpub.SerializeUncompressed()
	ethadd := common.BytesToAddress(crypto.Keccak256(pubBytes[1:])[12:])
	p.brdEthAddr = ethadd.String()

	return
}

// getBitcoinAddress by specifying the chain number (0 for normal, 1 for change), child number (address number),
// and whether testnet or not. Count specifies how many addresses to return.
func (p Phrase) getBitcoinAddresses(chainNo uint32, childNo uint32, count int, testnet bool) (addresses []Address, err error) {
	if count < 1 {
		count = 1
	}

	if count > 100 {
		count = 100
	}

	// Using our saved BIP32 starting point, generate the target address
	// Path: m/0H/[chain]
	chain, err := p.brdBtc.Child(chainNo)
	if err != nil {
		return
	}

	for i := 0; i < count; i++ {
		// Path: m/0H/[chain]/[child] (e.g. m/0H/0/0)
		child, err := chain.Child(childNo)
		if err != nil {
			return nil, err
		}

		// Generate address based on testnet or mainnet
		params := &chaincfg.MainNetParams

		if testnet {
			params = &chaincfg.TestNet3Params
		}

		pkh, err := child.Address(params)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, Address{Address: pkh.EncodeAddress()})

		childNo++
	}

	return
}

// LookupBTC looks up one or more bitcoin addresses, starting with the child specified and continuing for the number of
// addresses specified with "count"
func (p Phrase) LookupBTC(chain uint32, child uint32, count int, isTestnet bool) (addresses []Address, err error) {

	addresses, err = p.getBitcoinAddresses(chain, child, count, isTestnet)
	if err != nil {
		return
	}

	domain := ""
	if isTestnet {
		domain = "testnet."
	}

	var addylist string

	for i, v := range addresses {
		if isTestnet {
			addresses[i].IsTest = true
		}
		addylist += v.Address + "|"
	}

	var BCi map[string]struct {
		Balance  int64 `json:"final_balance"`
		TxCount  int   `json:"n_tx"`
		Received int64 `json:"total_received"`
	}

	err = callAPI("https://"+domain+"blockchain.info/balance?active="+addylist, &BCi)
	if err != nil {
		return
	}

	for i, v := range addresses {
		addresses[i].TxCount = BCi[v.Address].TxCount
		addresses[i].Balance = float64(float64(BCi[v.Address].Balance) / 100000000)
	}
	return

}

// LookupBCH looks up one or more bitcoin addresses, starting with the child specified and continuing for the number of
// addresses specified with "count"
func (p Phrase) LookupBCH(chain uint32, child uint32, count int) (addresses []Address, err error) {

	addresses, err = p.getBitcoinAddresses(chain, child, count, false)
	if err != nil {
		return
	}

	// Hack: the API will return an array if we request 2+ addresses but only return the single object if we request only 1.
	// So, to avoid defining two structures, we will just add another address to force the response to be an array.
	if len(addresses) == 1 {
		addresses = append(addresses, addresses[0])
	}

	var addylist string
	for i, v := range addresses {
		addylist += v.Address
		if i < len(addresses)-1 {
			addylist += ","
		}
	}

	var BTCcom struct {
		Error int `json:"err_no"`
		Data  []struct {
			Address  string `json:"address"`
			Balance  int64  `json:"balance"`
			TxCount  int    `json:"tx_count"`
			Received int64  `json:"received"`
		} `json:"data"`
	}

	err = callAPI("https://bch-chain.api.btc.com/v3/address/"+addylist, &BTCcom)
	if err != nil {
		return
	}

	if BTCcom.Error != 0 {
		err = errors.New("BTC.com error: " + strconv.Itoa(BTCcom.Error))
		return
	}

	for i, v := range addresses {
		for _, d := range BTCcom.Data {
			if d.Address == v.Address {
				addresses[i].TxCount = d.TxCount
				addresses[i].Balance = float64(float64(d.Balance) / 100000000)
				break
			}
		}
	}
	return
}

// LookupETH looks up the details for this phrase's ethereum address. Returns an array of Addresses to be consistent
// with other methods, but only populates the first item in the array.
func (p Phrase) LookupETH(isTestnet bool) (addresses []Address, err error) {

	addresses = []Address{
		Address{Address: p.brdEthAddr, IsTest: isTestnet},
	}

	domain := "api"
	if isTestnet {
		domain = "api-ropsten"
	}

	// Lookup activity
	var ethact struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Result  []struct {
			Hash string `json:"hash"`
		} `json:"result"`
	}

	err = callAPI("https://"+domain+".etherscan.io/api?module=account&action=txlist&address="+addresses[0].Address, &ethact)
	if err != nil {
		return
	}

	if ethact.Status != "1" && ethact.Message != "No transactions found" {
		err = errors.New("etherscan error: " + ethact.Message)
		return
	}

	addresses[0].TxCount = len(ethact.Result)

	if len(ethact.Result) > 0 {
		// Ethereumn balance lookup (only if we've seen activity)
		var ethbal struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Result  string `json:"result"`
		}

		err = callAPI("https://"+domain+".etherscan.io/api?module=account&action=balance&address="+addresses[0].Address, &ethbal)
		if err != nil {
			return
		}

		if ethbal.Status != "1" {
			err = errors.New("etherscan error: " + ethbal.Message)
			return
		}

		addresses[0].Balance, err = snipEth(ethbal.Result, 18)
		if err != nil {
			return
		}
	}

	// ERC20 token lookup
	var erc20bal struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Result  []struct {
			Address string `json:"contractAddress"`
			To      string `json:"to"`
			Value   string `json:"value"`
			Name    string `json:"tokenName"`
			Ticker  string `json:"tokenSymbol"`
			Decimal string `json:"tokenDecimal"`
			Hash    string `json:"hash"`
		} `json:"result"`
	}

	err = callAPI("https://"+domain+".etherscan.io/api?module=account&action=tokentx&address="+addresses[0].Address, &erc20bal)
	if err != nil {
		return
	}

	if erc20bal.Status != "1" {
		return
	}

	if len(erc20bal.Result) >= 10000 {
		fmt.Println("WARN: Eth address has at least 10k transactions--any more than that will be omitted.")
	}

	txlist := make(map[string]int)
	for _, v := range erc20bal.Result {
		if v.Ticker == "" {
			continue
		}

		idx, e := txlist[v.Ticker]

		if !e {
			// Create a new entry if we haven't seen it yet
			txlist[v.Ticker] = len(addresses[0].Tokens)
			addresses[0].Tokens = append(addresses[0].Tokens, Token{
				Name:    v.Name,
				Ticker:  v.Ticker,
				Address: v.Address,
			})
		}

		// Convert number of decimal places this token uses
		var dec int
		dec, err = strconv.Atoi(v.Decimal)
		if err != nil {
			return
		}

		// Parse the value and snip it down based on decimal places
		var val float64
		val, err = snipEth(v.Value, dec)
		if err != nil {
			return
		}

		// Etherscan returns addresses all lower case...
		if v.To != strings.ToLower(addresses[0].Address) {
			// This must be a send, not a receive
			val *= -1
		}

		nicebal := float64(val)
		if dec > 0 {
			nicebal = float64(val / float64(10^dec))
		}

		addresses[0].Tokens[idx].Balance += nicebal
		addresses[0].Tokens[idx].TxCount++
	}
	return
}

// LookupBTCBal follows the entire btc/bch chain and finds out the remaining balance for the entire wallet.
func (p Phrase) LookupBTCBal(coin string) (balance float64, addresses []Address, err error) {

	batch := 50 // How many addresses to fetch at one time
	skips := 0  // How many empty addresses in a row we've found

	// chain 0 = main, chain 1 = change
	for chain := uint32(0); chain < 2; chain++ {

		child := uint32(0)

		for skips < 10 { // Go until we find 10 in a row that are unused
			var addr []Address
			switch coin {
			case "btc":
				addr, err = p.LookupBTC(chain, child, batch, false)
			case "tbt":
				addr, err = p.LookupBTC(chain, child, batch, true)
			case "bch":
				addr, err = p.LookupBCH(chain, child, batch)
			}

			addresses = append(addresses, addr...)

			for _, v := range addr {
				balance += v.Balance
				if v.TxCount > 0 {
					skips = 0
				} else {
					skips++
				}
			}

			child += uint32(batch)
		}
	}
	return
}

func callAPI(url string, target interface{}) (err error) {
	//rate limit
	if time.Since(lastcall) < time.Millisecond*200 {
		time.Sleep(lastcall.Add(time.Millisecond * 200).Sub(time.Now()))
	}

	var res *http.Response
	res, err = http.Get(url)
	if err != nil {
		return
	}

	var data []byte
	data, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	lastcall = time.Now()
	err = json.Unmarshal(data, target)
	if err != nil {
		fmt.Printf("\n%s\n", string(data))
	}
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
