## 12-Word Recovery Phrase Activity Scanner

This simple tool takes a 12-word phrase, such as those used by the BRD cryptocurrency wallet, and scans for wallet activity. It will search for both BIP32 and BIP44 wallets, however 24-word phrases are not currently supported.

It will list out the following:

1. If the phrase has ever been used for for BTC, BCH, or ETH.
1. If a remaining balance exists for BTC, BCH, or ETH.
1. If the Ethereum address has any remaining ERC20 token balances (if not, nothing will be displayed)

**Note 1:** This tool is also capable of scanning for Testnet BTC and ETH, however this feature is currently disabled to API unavailability. If you want to try it out, download the source code and change "showTestnet" to "true" at the top of main.go.

**Note 2:** BIP44 support only looks at account 0.

### Usage

Either build the project with Go or download the binary, then...

**Single phrase:** Run the binary followed by a 12-word phrase: `scan-phrase one two [...] twelve`

**Multiple phrases:** List up multiple phrases, one per line, in a file titled `phrase.txt`. In the same directory as this file, run the binary with no arguments: `phrase-scan`

**Single currency:** By default this tool will search for BTC, BCH, and ETH. If you want to search for just one of these, use the `-coin` flag followed by the currency you want to search for. For example, `scan-phrase -coin=btc` will only look for bitcoin. This will work with either single- or multi-phrase mode, but in the case of single-phrase mode, the flag must always come before the phrase (e.g. `scan-phrase -coin=btc [12 word phrase]`).

If you downloaded the binary, remember to give the file execure permissions with `chmod` and add `./` before the commands listed above.

### Block explorers

This tool uses the following block explorers:

* Blockchain.info: Bitcoin, Bitcoin Testnet
* Etherscan.io: Ethereum, Ethereum Testnet (Ropsten - not implemented for this tool yet)
* btc.com: Bitcoin Cash (Mainnet only)
