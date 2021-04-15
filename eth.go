package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// LookupETH looks up the details for this phrase's ethereum address. Returns an array of Addresses to be consistent
// with other methods, but only populates the first item in the array.
func (p Phrase) LookupETH(isTestnet bool) (addresses []*Address, err error) {

	// Get our ethereum xpub
	ethxpub, err := deriveHDKey(p.xprv, 44, 60, 0, 0, 0)
	if err != nil {
		return
	}

	// Get our ethereum key
	ethkey, err := ethxpub.ECPubKey()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Generate our ethereum address for the request
	pubBytes := ethkey.SerializeUncompressed()
	ethadd := common.BytesToAddress(crypto.Keccak256(pubBytes[1:])[12:])

	addresses = []*Address{
		{Address: ethadd.String(), IsTest: isTestnet},
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

	url := "https://" + domain + ".etherscan.io/api?module=account&action=txlist&address=" + addresses[0].Address
	err = callAPI(url, &ethact, etherscanRate)
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

		url = "https://" + domain + ".etherscan.io/api?module=account&action=balance&address=" + addresses[0].Address
		err = callAPI(url, &ethbal, etherscanRate)
		if err != nil {
			return
		}

		if ethbal.Status != "1" {
			fmt.Printf("%#v\n", ethbal)
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

	err = callAPI("https://"+domain+".etherscan.io/api?module=account&action=tokentx&address="+addresses[0].Address, &erc20bal, etherscanRate)
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
