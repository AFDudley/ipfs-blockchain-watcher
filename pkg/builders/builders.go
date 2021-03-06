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

package builders

import (
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/btc"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/eth"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/postgres"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/shared"
)

// NewResponseFilterer constructs a ResponseFilterer for the provided chain type
func NewResponseFilterer(chain shared.ChainType) (shared.ResponseFilterer, error) {
	switch chain {
	case shared.Ethereum:
		return eth.NewResponseFilterer(), nil
	case shared.Bitcoin:
		return btc.NewResponseFilterer(), nil
	default:
		return nil, fmt.Errorf("invalid chain %s for filterer constructor", chain.String())
	}
}

// NewCIDIndexer constructs a CIDIndexer for the provided chain type
func NewCIDIndexer(chain shared.ChainType, db *postgres.DB, ipfsMode shared.IPFSMode) (shared.CIDIndexer, error) {
	switch chain {
	case shared.Ethereum:
		switch ipfsMode {
		case shared.LocalInterface, shared.RemoteClient:
			return eth.NewCIDIndexer(db), nil
		case shared.DirectPostgres:
			return eth.NewIPLDPublisherAndIndexer(db), nil
		default:
			return nil, fmt.Errorf("ethereum CIDIndexer unexpected ipfs mode %s", ipfsMode.String())
		}
	case shared.Bitcoin:
		switch ipfsMode {
		case shared.LocalInterface, shared.RemoteClient:
			return btc.NewCIDIndexer(db), nil
		case shared.DirectPostgres:
			return eth.NewIPLDPublisherAndIndexer(db), nil
		default:
			return nil, fmt.Errorf("bitcoin CIDIndexer unexpected ipfs mode %s", ipfsMode.String())
		}
	default:
		return nil, fmt.Errorf("invalid chain %s for indexer constructor", chain.String())
	}
}

// NewCIDRetriever constructs a CIDRetriever for the provided chain type
func NewCIDRetriever(chain shared.ChainType, db *postgres.DB) (shared.CIDRetriever, error) {
	switch chain {
	case shared.Ethereum:
		return eth.NewCIDRetriever(db), nil
	case shared.Bitcoin:
		return btc.NewCIDRetriever(db), nil
	default:
		return nil, fmt.Errorf("invalid chain %s for retriever constructor", chain.String())
	}
}

// NewPayloadStreamer constructs a PayloadStreamer for the provided chain type
func NewPayloadStreamer(chain shared.ChainType, clientOrConfig interface{}) (shared.PayloadStreamer, chan shared.RawChainData, error) {
	switch chain {
	case shared.Ethereum:
		ethClient, ok := clientOrConfig.(*rpc.Client)
		if !ok {
			return nil, nil, fmt.Errorf("ethereum payload streamer constructor expected client type %T got %T", &rpc.Client{}, clientOrConfig)
		}
		streamChan := make(chan shared.RawChainData, eth.PayloadChanBufferSize)
		return eth.NewPayloadStreamer(ethClient), streamChan, nil
	case shared.Bitcoin:
		btcClientConn, ok := clientOrConfig.(*rpcclient.ConnConfig)
		if !ok {
			return nil, nil, fmt.Errorf("bitcoin payload streamer constructor expected client config type %T got %T", rpcclient.ConnConfig{}, clientOrConfig)
		}
		streamChan := make(chan shared.RawChainData, btc.PayloadChanBufferSize)
		return btc.NewHTTPPayloadStreamer(btcClientConn), streamChan, nil
	default:
		return nil, nil, fmt.Errorf("invalid chain %s for streamer constructor", chain.String())
	}
}

// NewPaylaodFetcher constructs a PayloadFetcher for the provided chain type
func NewPaylaodFetcher(chain shared.ChainType, client interface{}, timeout time.Duration) (shared.PayloadFetcher, error) {
	switch chain {
	case shared.Ethereum:
		batchClient, ok := client.(*rpc.Client)
		if !ok {
			return nil, fmt.Errorf("ethereum payload fetcher constructor expected client type %T got %T", &rpc.Client{}, client)
		}
		return eth.NewPayloadFetcher(batchClient, timeout), nil
	case shared.Bitcoin:
		connConfig, ok := client.(*rpcclient.ConnConfig)
		if !ok {
			return nil, fmt.Errorf("bitcoin payload fetcher constructor expected client type %T got %T", &rpcclient.Client{}, client)
		}
		return btc.NewPayloadFetcher(connConfig)
	default:
		return nil, fmt.Errorf("invalid chain %s for payload fetcher constructor", chain.String())
	}
}

// NewPayloadConverter constructs a PayloadConverter for the provided chain type
func NewPayloadConverter(chain shared.ChainType) (shared.PayloadConverter, error) {
	switch chain {
	case shared.Ethereum:
		return eth.NewPayloadConverter(params.MainnetChainConfig), nil
	case shared.Bitcoin:
		return btc.NewPayloadConverter(&chaincfg.MainNetParams), nil
	default:
		return nil, fmt.Errorf("invalid chain %s for converter constructor", chain.String())
	}
}

// NewIPLDFetcher constructs an IPLDFetcher for the provided chain type
func NewIPLDFetcher(chain shared.ChainType, ipfsPath string, db *postgres.DB, ipfsMode shared.IPFSMode) (shared.IPLDFetcher, error) {
	switch chain {
	case shared.Ethereum:
		switch ipfsMode {
		case shared.LocalInterface, shared.RemoteClient:
			return eth.NewIPLDFetcher(ipfsPath)
		case shared.DirectPostgres:
			return eth.NewIPLDPGFetcher(db), nil
		default:
			return nil, fmt.Errorf("ethereum IPLDFetcher unexpected ipfs mode %s", ipfsMode.String())
		}
	case shared.Bitcoin:
		switch ipfsMode {
		case shared.LocalInterface, shared.RemoteClient:
			return btc.NewIPLDFetcher(ipfsPath)
		case shared.DirectPostgres:
			return btc.NewIPLDPGFetcher(db), nil
		default:
			return nil, fmt.Errorf("bitcoin IPLDFetcher unexpected ipfs mode %s", ipfsMode.String())
		}
	default:
		return nil, fmt.Errorf("invalid chain %s for IPLD fetcher constructor", chain.String())
	}
}

// NewIPLDPublisher constructs an IPLDPublisher for the provided chain type
func NewIPLDPublisher(chain shared.ChainType, ipfsPath string, db *postgres.DB, ipfsMode shared.IPFSMode) (shared.IPLDPublisher, error) {
	switch chain {
	case shared.Ethereum:
		switch ipfsMode {
		case shared.LocalInterface, shared.RemoteClient:
			return eth.NewIPLDPublisher(ipfsPath)
		case shared.DirectPostgres:
			return eth.NewIPLDPublisherAndIndexer(db), nil
		default:
			return nil, fmt.Errorf("ethereum IPLDPublisher unexpected ipfs mode %s", ipfsMode.String())
		}
	case shared.Bitcoin:
		switch ipfsMode {
		case shared.LocalInterface, shared.RemoteClient:
			return btc.NewIPLDPublisher(ipfsPath)
		case shared.DirectPostgres:
			return btc.NewIPLDPublisherAndIndexer(db), nil
		default:
			return nil, fmt.Errorf("bitcoin IPLDPublisher unexpected ipfs mode %s", ipfsMode.String())
		}
	default:
		return nil, fmt.Errorf("invalid chain %s for publisher constructor", chain.String())
	}
}

// NewPublicAPI constructs a PublicAPI for the provided chain type
func NewPublicAPI(chain shared.ChainType, db *postgres.DB, ipfsPath string) (rpc.API, error) {
	switch chain {
	case shared.Ethereum:
		backend, err := eth.NewEthBackend(db)
		if err != nil {
			return rpc.API{}, err
		}
		return rpc.API{
			Namespace: eth.APIName,
			Version:   eth.APIVersion,
			Service:   eth.NewPublicEthAPI(backend),
			Public:    true,
		}, nil
	default:
		return rpc.API{}, fmt.Errorf("invalid chain %s for public api constructor", chain.String())
	}
}

// NewCleaner constructs a Cleaner for the provided chain type
func NewCleaner(chain shared.ChainType, db *postgres.DB) (shared.Cleaner, error) {
	switch chain {
	case shared.Ethereum:
		return eth.NewCleaner(db), nil
	case shared.Bitcoin:
		return btc.NewCleaner(db), nil
	default:
		return nil, fmt.Errorf("invalid chain %s for cleaner constructor", chain.String())
	}
}
