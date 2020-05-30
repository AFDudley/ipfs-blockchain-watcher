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

package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	log "github.com/sirupsen/logrus"

	"github.com/vulcanize/ipfs-chain-watcher/pkg/ipfs"
	"github.com/vulcanize/ipfs-chain-watcher/pkg/shared"
)

var (
	errUnexpectedNumberOfIPLDs = errors.New("ipfs batch fetch returned unexpected number of IPLDs")
)

// IPLDFetcher satisfies the IPLDFetcher interface for ethereum
type IPLDFetcher struct {
	BlockService blockservice.BlockService
}

// NewIPLDFetcher creates a pointer to a new IPLDFetcher
func NewIPLDFetcher(ipfsPath string) (*IPLDFetcher, error) {
	blockService, err := ipfs.InitIPFSBlockService(ipfsPath)
	if err != nil {
		return nil, err
	}
	return &IPLDFetcher{
		BlockService: blockService,
	}, nil
}

// Fetch is the exported method for fetching and returning all the IPLDS specified in the CIDWrapper
func (f *IPLDFetcher) Fetch(cids shared.CIDsForFetching) (shared.IPLDs, error) {
	cidWrapper, ok := cids.(*CIDWrapper)
	if !ok {
		return nil, fmt.Errorf("eth fetcher: expected cids type %T got %T", &CIDWrapper{}, cids)
	}
	log.Debug("fetching iplds")
	var err error
	iplds := IPLDs{}
	iplds.TotalDifficulty, ok = new(big.Int).SetString(cidWrapper.Header.TotalDifficulty, 10)
	if !ok {
		return nil, errors.New("eth fetcher: unable to set total difficulty")
	}
	iplds.BlockNumber = cidWrapper.BlockNumber
	iplds.Header, err = f.FetchHeader(cidWrapper.Header)
	if err != nil {
		return nil, err
	}
	iplds.Uncles, err = f.FetchUncles(cidWrapper.Uncles)
	if err != nil {
		return nil, err
	}
	iplds.Transactions, err = f.FetchTrxs(cidWrapper.Transactions)
	if err != nil {
		return nil, err
	}
	iplds.Receipts, err = f.FetchRcts(cidWrapper.Receipts)
	if err != nil {
		return nil, err
	}
	iplds.StateNodes, err = f.FetchState(cidWrapper.StateNodes)
	if err != nil {
		return nil, err
	}
	iplds.StorageNodes, err = f.FetchStorage(cidWrapper.StorageNodes)
	if err != nil {
		return nil, err
	}
	return iplds, nil
}

// FetchHeaders fetches headers
// It uses the f.fetch method
func (f *IPLDFetcher) FetchHeader(c HeaderModel) (ipfs.BlockModel, error) {
	log.Debug("fetching header ipld")
	dc, err := cid.Decode(c.CID)
	if err != nil {
		return ipfs.BlockModel{}, err
	}
	header, err := f.fetch(dc)
	if err != nil {
		return ipfs.BlockModel{}, err
	}
	return ipfs.BlockModel{
		Data: header.RawData(),
		CID:  header.Cid().String(),
	}, nil
}

// FetchUncles fetches uncles
// It uses the f.fetchBatch method
func (f *IPLDFetcher) FetchUncles(cids []UncleModel) ([]ipfs.BlockModel, error) {
	log.Debug("fetching uncle iplds")
	uncleCids := make([]cid.Cid, len(cids))
	for i, c := range cids {
		dc, err := cid.Decode(c.CID)
		if err != nil {
			return nil, err
		}
		uncleCids[i] = dc
	}
	uncles := f.fetchBatch(uncleCids)
	uncleIPLDs := make([]ipfs.BlockModel, len(uncles))
	for i, uncle := range uncles {
		uncleIPLDs[i] = ipfs.BlockModel{
			Data: uncle.RawData(),
			CID:  uncle.Cid().String(),
		}
	}
	if len(uncleIPLDs) != len(uncleCids) {
		log.Errorf("ipfs fetcher: number of uncle blocks returned (%d) does not match number expected (%d)", len(uncles), len(uncleCids))
		return uncleIPLDs, errUnexpectedNumberOfIPLDs
	}
	return uncleIPLDs, nil
}

// FetchTrxs fetches transactions
// It uses the f.fetchBatch method
func (f *IPLDFetcher) FetchTrxs(cids []TxModel) ([]ipfs.BlockModel, error) {
	log.Debug("fetching transaction iplds")
	trxCids := make([]cid.Cid, len(cids))
	for i, c := range cids {
		dc, err := cid.Decode(c.CID)
		if err != nil {
			return nil, err
		}
		trxCids[i] = dc
	}
	trxs := f.fetchBatch(trxCids)
	trxIPLDs := make([]ipfs.BlockModel, len(trxs))
	for i, trx := range trxs {
		trxIPLDs[i] = ipfs.BlockModel{
			Data: trx.RawData(),
			CID:  trx.Cid().String(),
		}
	}
	if len(trxIPLDs) != len(trxCids) {
		log.Errorf("ipfs fetcher: number of transaction blocks returned (%d) does not match number expected (%d)", len(trxs), len(trxCids))
		return trxIPLDs, errUnexpectedNumberOfIPLDs
	}
	return trxIPLDs, nil
}

// FetchRcts fetches receipts
// It uses the f.fetchBatch method
func (f *IPLDFetcher) FetchRcts(cids []ReceiptModel) ([]ipfs.BlockModel, error) {
	log.Debug("fetching receipt iplds")
	rctCids := make([]cid.Cid, len(cids))
	for i, c := range cids {
		dc, err := cid.Decode(c.CID)
		if err != nil {
			return nil, err
		}
		rctCids[i] = dc
	}
	rcts := f.fetchBatch(rctCids)
	rctIPLDs := make([]ipfs.BlockModel, len(rcts))
	for i, rct := range rcts {
		rctIPLDs[i] = ipfs.BlockModel{
			Data: rct.RawData(),
			CID:  rct.Cid().String(),
		}
	}
	if len(rctIPLDs) != len(rctCids) {
		log.Errorf("ipfs fetcher: number of receipt blocks returned (%d) does not match number expected (%d)", len(rcts), len(rctCids))
		return rctIPLDs, errUnexpectedNumberOfIPLDs
	}
	return rctIPLDs, nil
}

// FetchState fetches state nodes
// It uses the single f.fetch method instead of the batch fetch, because it
// needs to maintain the data's relation to state keys
func (f *IPLDFetcher) FetchState(cids []StateNodeModel) ([]StateNode, error) {
	log.Debug("fetching state iplds")
	stateNodes := make([]StateNode, 0, len(cids))
	for _, stateNode := range cids {
		if stateNode.CID == "" {
			continue
		}
		dc, err := cid.Decode(stateNode.CID)
		if err != nil {
			return nil, err
		}
		state, err := f.fetch(dc)
		if err != nil {
			return nil, err
		}
		stateNodes = append(stateNodes, StateNode{
			IPLD: ipfs.BlockModel{
				Data: state.RawData(),
				CID:  state.Cid().String(),
			},
			StateLeafKey: common.HexToHash(stateNode.StateKey),
			Type:         ResolveToNodeType(stateNode.NodeType),
			Path:         stateNode.Path,
		})
	}
	return stateNodes, nil
}

// FetchStorage fetches storage nodes
// It uses the single f.fetch method instead of the batch fetch, because it
// needs to maintain the data's relation to state and storage keys
func (f *IPLDFetcher) FetchStorage(cids []StorageNodeWithStateKeyModel) ([]StorageNode, error) {
	log.Debug("fetching storage iplds")
	storageNodes := make([]StorageNode, 0, len(cids))
	for _, storageNode := range cids {
		if storageNode.CID == "" || storageNode.StateKey == "" {
			continue
		}
		dc, err := cid.Decode(storageNode.CID)
		if err != nil {
			return nil, err
		}
		storage, err := f.fetch(dc)
		if err != nil {
			return nil, err
		}
		storageNodes = append(storageNodes, StorageNode{
			IPLD: ipfs.BlockModel{
				Data: storage.RawData(),
				CID:  storage.Cid().String(),
			},
			StateLeafKey:   common.HexToHash(storageNode.StateKey),
			StorageLeafKey: common.HexToHash(storageNode.StorageKey),
			Type:           ResolveToNodeType(storageNode.NodeType),
			Path:           storageNode.Path,
		})
	}
	return storageNodes, nil
}

// fetch is used to fetch a single cid
func (f *IPLDFetcher) fetch(cid cid.Cid) (blocks.Block, error) {
	return f.BlockService.GetBlock(context.Background(), cid)
}

// fetchBatch is used to fetch a batch of IPFS data blocks by cid
// There is no guarantee all are fetched, and no error in such a case, so
// downstream we will need to confirm which CIDs were fetched in the result set
func (f *IPLDFetcher) fetchBatch(cids []cid.Cid) []blocks.Block {
	fetchedBlocks := make([]blocks.Block, 0, len(cids))
	blockChan := f.BlockService.GetBlocks(context.Background(), cids)
	for block := range blockChan {
		fetchedBlocks = append(fetchedBlocks, block)
	}
	return fetchedBlocks
}
