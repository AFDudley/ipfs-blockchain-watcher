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

package eth_test

import (
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vulcanize/ipfs-chain-watcher/pkg/eth"
	"github.com/vulcanize/ipfs-chain-watcher/pkg/eth/mocks"
	mocks2 "github.com/vulcanize/ipfs-chain-watcher/pkg/ipfs/mocks"
)

var (
	mockHeaderDagPutter  *mocks2.MappedDagPutter
	mockTrxDagPutter     *mocks2.MappedDagPutter
	mockTrxTrieDagPutter *mocks2.DagPutter
	mockRctDagPutter     *mocks2.MappedDagPutter
	mockRctTrieDagPutter *mocks2.DagPutter
	mockStateDagPutter   *mocks2.MappedDagPutter
	mockStorageDagPutter *mocks2.MappedDagPutter
)

var _ = Describe("Publisher", func() {
	BeforeEach(func() {
		mockHeaderDagPutter = new(mocks2.MappedDagPutter)
		mockTrxDagPutter = new(mocks2.MappedDagPutter)
		mockTrxTrieDagPutter = new(mocks2.DagPutter)
		mockRctDagPutter = new(mocks2.MappedDagPutter)
		mockRctTrieDagPutter = new(mocks2.DagPutter)
		mockStateDagPutter = new(mocks2.MappedDagPutter)
		mockStorageDagPutter = new(mocks2.MappedDagPutter)
	})

	Describe("Publish", func() {
		It("Publishes the passed IPLDPayload objects to IPFS and returns a CIDPayload for indexing", func() {
			mockHeaderDagPutter.CIDsToReturn = map[common.Hash]string{
				common.BytesToHash(mocks.HeaderIPLD.RawData()): mocks.HeaderCID.String(),
			}
			mockTrxDagPutter.CIDsToReturn = map[common.Hash]string{
				common.BytesToHash(mocks.Trx1IPLD.RawData()): mocks.Trx1CID.String(),
				common.BytesToHash(mocks.Trx2IPLD.RawData()): mocks.Trx2CID.String(),
				common.BytesToHash(mocks.Trx3IPLD.RawData()): mocks.Trx3CID.String(),
			}
			mockRctDagPutter.CIDsToReturn = map[common.Hash]string{
				common.BytesToHash(mocks.Rct1IPLD.RawData()): mocks.Rct1CID.String(),
				common.BytesToHash(mocks.Rct2IPLD.RawData()): mocks.Rct2CID.String(),
				common.BytesToHash(mocks.Rct3IPLD.RawData()): mocks.Rct3CID.String(),
			}
			mockStateDagPutter.CIDsToReturn = map[common.Hash]string{
				common.BytesToHash(mocks.State1IPLD.RawData()): mocks.State1CID.String(),
				common.BytesToHash(mocks.State2IPLD.RawData()): mocks.State2CID.String(),
			}
			mockStorageDagPutter.CIDsToReturn = map[common.Hash]string{
				common.BytesToHash(mocks.StorageIPLD.RawData()): mocks.StorageCID.String(),
			}
			publisher := eth.IPLDPublisher{
				HeaderPutter:          mockHeaderDagPutter,
				TransactionPutter:     mockTrxDagPutter,
				TransactionTriePutter: mockTrxTrieDagPutter,
				ReceiptPutter:         mockRctDagPutter,
				ReceiptTriePutter:     mockRctTrieDagPutter,
				StatePutter:           mockStateDagPutter,
				StoragePutter:         mockStorageDagPutter,
			}
			payload, err := publisher.Publish(mocks.MockConvertedPayload)
			Expect(err).ToNot(HaveOccurred())
			cidPayload, ok := payload.(*eth.CIDPayload)
			Expect(ok).To(BeTrue())
			Expect(cidPayload.HeaderCID.TotalDifficulty).To(Equal(mocks.MockConvertedPayload.TotalDifficulty.String()))
			Expect(cidPayload.HeaderCID.BlockNumber).To(Equal(mocks.MockCIDPayload.HeaderCID.BlockNumber))
			Expect(cidPayload.HeaderCID.BlockHash).To(Equal(mocks.MockCIDPayload.HeaderCID.BlockHash))
			Expect(cidPayload.HeaderCID.Reward).To(Equal(mocks.MockCIDPayload.HeaderCID.Reward))
			Expect(cidPayload.UncleCIDs).To(Equal(mocks.MockCIDPayload.UncleCIDs))
			Expect(cidPayload.HeaderCID).To(Equal(mocks.MockCIDPayload.HeaderCID))
			Expect(len(cidPayload.TransactionCIDs)).To(Equal(3))
			Expect(cidPayload.TransactionCIDs[0]).To(Equal(mocks.MockCIDPayload.TransactionCIDs[0]))
			Expect(cidPayload.TransactionCIDs[1]).To(Equal(mocks.MockCIDPayload.TransactionCIDs[1]))
			Expect(cidPayload.TransactionCIDs[2]).To(Equal(mocks.MockCIDPayload.TransactionCIDs[2]))
			Expect(len(cidPayload.ReceiptCIDs)).To(Equal(3))
			Expect(cidPayload.ReceiptCIDs[mocks.MockTransactions[0].Hash()]).To(Equal(mocks.MockCIDPayload.ReceiptCIDs[mocks.MockTransactions[0].Hash()]))
			Expect(cidPayload.ReceiptCIDs[mocks.MockTransactions[1].Hash()]).To(Equal(mocks.MockCIDPayload.ReceiptCIDs[mocks.MockTransactions[1].Hash()]))
			Expect(cidPayload.ReceiptCIDs[mocks.MockTransactions[2].Hash()]).To(Equal(mocks.MockCIDPayload.ReceiptCIDs[mocks.MockTransactions[2].Hash()]))
			Expect(len(cidPayload.StateNodeCIDs)).To(Equal(2))
			Expect(cidPayload.StateNodeCIDs[0]).To(Equal(mocks.MockCIDPayload.StateNodeCIDs[0]))
			Expect(cidPayload.StateNodeCIDs[1]).To(Equal(mocks.MockCIDPayload.StateNodeCIDs[1]))
			Expect(cidPayload.StorageNodeCIDs).To(Equal(mocks.MockCIDPayload.StorageNodeCIDs))
		})
	})
})
