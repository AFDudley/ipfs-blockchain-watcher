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

package dag_putters

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/vulcanize/vulcanizedb/pkg/ipfs"
	"github.com/vulcanize/vulcanizedb/pkg/ipfs/ipld"
)

type EthReceiptDagPutter struct {
	adder *ipfs.IPFS
}

func NewEthReceiptDagPutter(adder *ipfs.IPFS) *EthReceiptDagPutter {
	return &EthReceiptDagPutter{adder: adder}
}

func (erdp *EthReceiptDagPutter) DagPut(raw interface{}) ([]string, error) {
	receipts, ok := raw.(types.Receipts)
	if !ok {
		return nil, fmt.Errorf("EthReceiptDagPutter expected input type %T got type %T", types.Receipts{}, raw)
	}
	cids := make([]string, len(receipts))
	for i, receipt := range receipts {
		node, err := ipld.NewReceipt(receipt)
		if err != nil {
			return nil, err
		}
		if err := erdp.adder.Add(node); err != nil {
			return nil, err
		}
		cids[i] = node.Cid().String()
	}
	return cids, nil
}
