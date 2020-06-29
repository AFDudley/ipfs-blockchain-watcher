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

// Streamer is used by watchers to stream eth data from a vulcanizedb super node
package streamer

import (
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/core"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/watch"
)

// SuperNodeStreamer is the underlying struct for the shared.SuperNodeStreamer interface
type SuperNodeStreamer struct {
	Client core.RPCClient
}

// NewSuperNodeStreamer creates a pointer to a new SuperNodeStreamer which satisfies the ISuperNodeStreamer interface
func NewSuperNodeStreamer(client core.RPCClient) *SuperNodeStreamer {
	return &SuperNodeStreamer{
		Client: client,
	}
}

// Stream is the main loop for subscribing to data from a vulcanizedb super node
func (sds *SuperNodeStreamer) Stream(payloadChan chan watcher.SubscriptionPayload, rlpParams []byte) (*rpc.ClientSubscription, error) {
	return sds.Client.Subscribe("vdb", payloadChan, "stream", rlpParams)
}
