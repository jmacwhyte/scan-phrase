## 12-Word Recovery Phrase Activity Scanner

This simple tool takes a 12-word phrase, such as those used by the BRD cryptocurrency wallet, and scans for wallet activity. It will list out, in order, the following:

1. If the phrase has ever been used for for BTC, Test BTC, BCH, or ETH.
1. If a remaining balance exists for BTC, Test BTC, BCH, or ETH.
1. If the Ethereum address has any remaining ERC20 token balances (if not, nothing will be displayed)

### Usage

Either build the project with Go or download the binary, then...

**Single phrase:** Run the binary followed by a 12-word phrase: `./scan-phrase one two [...] twelve`

**Multiple phrases:** List up multiple phrases, one per line, in a file titled `phrase.txt`. In the same directory as this file, run the binary with no arguments: `./phrase-scan`

### Block explorers

This tool uses the following block explorers:

* Blockchain.info: Bitcoin, Bitcoin Testnet
* Etherscan.io: Ethereum, Ethereum Testnet (Ropsten - not implemented for this tool yet)
* btc.com: Bitcoin Cash (Mainnet only)
