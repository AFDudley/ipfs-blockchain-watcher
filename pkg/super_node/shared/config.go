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

package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"

	"github.com/vulcanize/vulcanizedb/pkg/config"
	"github.com/vulcanize/vulcanizedb/pkg/eth"
	"github.com/vulcanize/vulcanizedb/pkg/eth/client"
	vRpc "github.com/vulcanize/vulcanizedb/pkg/eth/converters/rpc"
	"github.com/vulcanize/vulcanizedb/pkg/eth/core"
	"github.com/vulcanize/vulcanizedb/pkg/eth/node"
	"github.com/vulcanize/vulcanizedb/pkg/postgres"
	"github.com/vulcanize/vulcanizedb/utils"
)

// SuperNodeConfig struct
type SuperNodeConfig struct {
	// Ubiquitous fields
	Chain    ChainType
	IPFSPath string
	DB       *postgres.DB
	DBConfig config.Database
	Quit     chan bool
	// Server fields
	Serve        bool
	WSEndpoint   string
	HTTPEndpoint string
	IPCEndpoint  string
	// Sync params
	Sync     bool
	Workers  int
	WSClient interface{}
	NodeInfo core.Node
	// Backfiller params
	BackFill   bool
	HTTPClient interface{}
	Frequency  time.Duration
	BatchSize  uint64
}

// NewSuperNodeConfigs is used to initialize multiple SuperNode configs from a single config .toml file
// Separate chain supernode instances need to be ran in the same process in order to avoid lock contention on the ipfs repository
func NewSuperNodeConfigs() ([]*SuperNodeConfig, error) {
	chains := viper.GetStringSlice("superNode.chains")
	configs := make([]*SuperNodeConfig, len(chains))
	var err error
	ipfsPath := viper.GetString("superNode.ipfsPath")
	if ipfsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		ipfsPath = filepath.Join(home, ".ipfs")
	}
	for i, chain := range chains {
		sn := new(SuperNodeConfig)
		sn.Chain, err = NewChainType(chain)
		if err != nil {
			return nil, err
		}
		sn.DBConfig = config.Database{
			Name:     viper.GetString(fmt.Sprintf("superNode.%s.database.name", chain)),
			Hostname: viper.GetString(fmt.Sprintf("superNode.%s.database.hostname", chain)),
			Port:     viper.GetInt(fmt.Sprintf("superNode.%s.database.port", chain)),
			User:     viper.GetString(fmt.Sprintf("superNode.%s.database.user", chain)),
			Password: viper.GetString(fmt.Sprintf("superNode.%s.database.password", chain)),
		}
		sn.IPFSPath = ipfsPath
		sn.Serve = viper.GetBool(fmt.Sprintf("superNode.%s.server.on", chain))
		sn.Sync = viper.GetBool(fmt.Sprintf("superNode.%s.sync.on", chain))
		if sn.Sync {
			workers := viper.GetInt("superNode.sync.workers")
			if workers < 1 {
				workers = 1
			}
			sn.Workers = workers
			switch sn.Chain {
			case Ethereum:
				sn.NodeInfo, sn.WSClient, err = getEthNodeAndClient(viper.GetString("superNode.ethereum.sync.wsPath"))
			case Bitcoin:
				sn.NodeInfo = core.Node{
					ID:           viper.GetString("superNode.bitcoin.node.nodeID"),
					ClientName:   viper.GetString("superNode.bitcoin.node.clientName"),
					GenesisBlock: viper.GetString("superNode.bitcoin.node.genesisBlock"),
					NetworkID:    viper.GetString("superNode.bitcoin.node.networkID"),
				}
				// For bitcoin we load in node info from the config because there is no RPC endpoint to retrieve this from the node
				sn.WSClient = &rpcclient.ConnConfig{
					Host:         viper.GetString("superNode.bitcoin.sync.wsPath"),
					HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
					DisableTLS:   true, // Bitcoin core does not provide TLS by default
					Pass:         viper.GetString("superNode.bitcoin.sync.pass"),
					User:         viper.GetString("superNode.bitcoin.sync.user"),
				}
			}
		}
		if sn.Serve {
			wsPath := viper.GetString(fmt.Sprintf("superNode.%s.server.wsPath", chain))
			if wsPath == "" {
				wsPath = "ws://127.0.0.1:8546"
			}
			sn.WSEndpoint = wsPath
			ipcPath := viper.GetString(fmt.Sprintf("superNode.%s.server.ipcPath", chain))
			if ipcPath == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return nil, err
				}
				ipcPath = filepath.Join(home, ".vulcanize/vulcanize.ipc")
			}
			sn.IPCEndpoint = ipcPath
			httpPath := viper.GetString(fmt.Sprintf("superNode.%s.server.httpPath", chain))
			if httpPath == "" {
				httpPath = "http://127.0.0.1:8545"
			}
			sn.HTTPEndpoint = httpPath
		}
		db := utils.LoadPostgres(sn.DBConfig, sn.NodeInfo)
		sn.DB = &db
		sn.Quit = make(chan bool)
		if viper.GetBool(fmt.Sprintf("superNode.%s.backFill.on", chain)) {
			if err := sn.BackFillFields(chain); err != nil {
				return nil, err
			}
		}
		configs[i] = sn
	}
	return configs, err
}

// BackFillFields is used to fill in the BackFill fields of the config
func (sn *SuperNodeConfig) BackFillFields(chain string) error {
	sn.BackFill = true
	var httpClient interface{}
	var err error
	switch sn.Chain {
	case Ethereum:
		_, httpClient, err = getEthNodeAndClient(viper.GetString("superNode.ethereum.backFill.httpPath"))
		if err != nil {
			return err
		}
	case Bitcoin:
		httpClient = &rpcclient.ConnConfig{
			Host:         viper.GetString("superNode.bitcoin.backFill.httpPath"),
			HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
			DisableTLS:   true, // Bitcoin core does not provide TLS by default
			Pass:         viper.GetString("superNode.bitcoin.backFill.pass"),
			User:         viper.GetString("superNode.bitcoin.backFill.user"),
		}
	}
	sn.HTTPClient = httpClient
	freq := viper.GetInt(fmt.Sprintf("superNode.%s.backFill.frequency", chain))
	var frequency time.Duration
	if freq <= 0 {
		frequency = time.Second * 30
	} else {
		frequency = time.Second * time.Duration(freq)
	}
	sn.Frequency = frequency
	sn.BatchSize = uint64(viper.GetInt64(fmt.Sprintf("superNode.%s.backFill.batchSize", chain)))
	return nil
}

func getEthNodeAndClient(path string) (core.Node, interface{}, error) {
	rawRPCClient, err := rpc.Dial(path)
	if err != nil {
		return core.Node{}, nil, err
	}
	rpcClient := client.NewRPCClient(rawRPCClient, path)
	ethClient := ethclient.NewClient(rawRPCClient)
	vdbEthClient := client.NewEthClient(ethClient)
	vdbNode := node.MakeNode(rpcClient)
	transactionConverter := vRpc.NewRPCTransactionConverter(ethClient)
	blockChain := eth.NewBlockChain(vdbEthClient, rpcClient, vdbNode, transactionConverter)
	return blockChain.Node(), rpcClient, nil
}