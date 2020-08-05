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

package btc

import (
	"fmt"
	"strconv"

	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/ipfs"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/ipfs/dag_putters"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/ipfs/ipld"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/shared"
)

// IPLDPublisher satisfies the IPLDPublisher for ethereum
type IPLDPublisher struct {
	HeaderPutter          ipfs.DagPutter
	TransactionPutter     ipfs.DagPutter
	TransactionTriePutter ipfs.DagPutter
}

// NewIPLDPublisher creates a pointer to a new Publisher which satisfies the IPLDPublisher interface
func NewIPLDPublisher(ipfsPath string) (*IPLDPublisher, error) {
	node, err := ipfs.InitIPFSNode(ipfsPath)
	if err != nil {
		return nil, err
	}
	return &IPLDPublisher{
		HeaderPutter:          dag_putters.NewBtcHeaderDagPutter(node),
		TransactionPutter:     dag_putters.NewBtcTxDagPutter(node),
		TransactionTriePutter: dag_putters.NewBtcTxTrieDagPutter(node),
	}, nil
}

// Publish publishes an IPLDPayload to IPFS and returns the corresponding CIDPayload
func (pub *IPLDPublisher) Publish(payload shared.ConvertedData) (shared.CIDsForIndexing, error) {
	ipldPayload, ok := payload.(ConvertedPayload)
	if !ok {
		return nil, fmt.Errorf("eth publisher expected payload type %T got %T", &ConvertedPayload{}, payload)
	}
	// Generate nodes
	headerNode, txNodes, txTrieNodes, err := ipld.FromHeaderAndTxs(ipldPayload.Header, ipldPayload.Txs)
	if err != nil {
		return nil, err
	}
	// Process and publish headers
	headerCid, err := pub.publishHeader(headerNode)
	if err != nil {
		return nil, err
	}
	mhKey, _ := shared.MultihashKeyFromCIDString(headerCid)
	header := HeaderModel{
		CID:         headerCid,
		MhKey:       mhKey,
		ParentHash:  ipldPayload.Header.PrevBlock.String(),
		BlockNumber: strconv.Itoa(int(ipldPayload.BlockPayload.BlockHeight)),
		BlockHash:   ipldPayload.Header.BlockHash().String(),
		Timestamp:   ipldPayload.Header.Timestamp.UnixNano(),
		Bits:        ipldPayload.Header.Bits,
	}
	// Process and publish transactions
	transactionCids, err := pub.publishTransactions(txNodes, txTrieNodes, ipldPayload.TxMetaData)
	if err != nil {
		return nil, err
	}
	// Package CIDs and their metadata into a single struct
	return &CIDPayload{
		HeaderCID:       header,
		TransactionCIDs: transactionCids,
	}, nil
}

func (pub *IPLDPublisher) publishHeader(header *ipld.BtcHeader) (string, error) {
	cid, err := pub.HeaderPutter.DagPut(header)
	if err != nil {
		return "", err
	}
	return cid, nil
}

func (pub *IPLDPublisher) publishTransactions(transactions []*ipld.BtcTx, txTrie []*ipld.BtcTxTrie, trxMeta []TxModelWithInsAndOuts) ([]TxModelWithInsAndOuts, error) {
	txCids := make([]TxModelWithInsAndOuts, len(transactions))
	for i, tx := range transactions {
		cid, err := pub.TransactionPutter.DagPut(tx)
		if err != nil {
			return nil, err
		}
		mhKey, _ := shared.MultihashKeyFromCIDString(cid)
		txCids[i] = TxModelWithInsAndOuts{
			CID:         cid,
			MhKey:       mhKey,
			Index:       trxMeta[i].Index,
			TxHash:      trxMeta[i].TxHash,
			SegWit:      trxMeta[i].SegWit,
			WitnessHash: trxMeta[i].WitnessHash,
			TxInputs:    trxMeta[i].TxInputs,
			TxOutputs:   trxMeta[i].TxOutputs,
		}
	}
	for _, txNode := range txTrie {
		// We don't do anything with the tx trie cids atm
		if _, err := pub.TransactionTriePutter.DagPut(txNode); err != nil {
			return nil, err
		}
	}
	return txCids, nil
}
