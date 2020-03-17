// VulcanizeDB
// Copyright © 2019 Vulcanize

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package ipld

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	mh "github.com/multiformats/go-multihash"
)

// FromBlock takes a block and processes it
// to return it a set of IPLD nodes for further processing.
func FromBlock(block *types.Block, receipts []*types.Receipt) (*EthHeader, []*EthTx, []*EthTxTrie, []*EthReceipt, []*EthRctTrie, error) {
	// process the eth-header object
	headerRawData := getRLP(block.Header())
	cid, err := RawdataToCid(MEthHeader, headerRawData, mh.KECCAK_256)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	ethHeader := &EthHeader{
		Header:  block.Header(),
		cid:     cid,
		rawdata: headerRawData,
	}

	// Process the found eth-tx objects
	ethTxNodes, ethTxTrieNodes, err := processTransactions(block.Transactions(),
		block.Header().TxHash[:])
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	// process the eth-rct objects
	ethRctNodes, ethRctTrieNodes, err := processReceipts(receipts,
		block.Header().ReceiptHash[:])

	return ethHeader, ethTxNodes, ethTxTrieNodes, ethRctNodes, ethRctTrieNodes, nil
}

// processTransactions will take the found transactions in a parsed block body
// to return IPLD node slices for eth-tx and eth-tx-trie
func processTransactions(txs []*types.Transaction, expectedTxRoot []byte) ([]*EthTx, []*EthTxTrie, error) {
	var ethTxNodes []*EthTx
	transactionTrie := newTxTrie()

	for idx, tx := range txs {
		ethTx, err := NewEthTx(tx)
		if err != nil {
			return nil, nil, err
		}
		ethTxNodes = append(ethTxNodes, ethTx)
		transactionTrie.add(idx, ethTx.RawData())
	}

	if !bytes.Equal(transactionTrie.rootHash(), expectedTxRoot) {
		return nil, nil, fmt.Errorf("wrong transaction hash computed")
	}

	return ethTxNodes, transactionTrie.getNodes(), nil
}

// processReceipts will take in receipts
// to return IPLD node slices for eth-rct and eth-rct-trie
func processReceipts(rcts []*types.Receipt, expectedRctRoot []byte) ([]*EthReceipt, []*EthRctTrie, error) {
	var ethRctNodes []*EthReceipt
	receiptTrie := newRctTrie()

	for idx, rct := range rcts {
		ethRct, err := NewReceipt(rct)
		if err != nil {
			return nil, nil, err
		}
		ethRctNodes = append(ethRctNodes, ethRct)
		receiptTrie.add(idx, ethRct.RawData())
	}

	if !bytes.Equal(receiptTrie.rootHash(), expectedRctRoot) {
		return nil, nil, fmt.Errorf("wrong receipt hash computed")
	}

	return ethRctNodes, receiptTrie.getNodes(), nil
}
