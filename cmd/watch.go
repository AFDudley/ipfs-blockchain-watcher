// Copyright © 2020 Vulcanize, Inc
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"os"
	"os/signal"
	"sync"

	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/ipfs"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/shared"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/watcher"
	v "github.com/vulcanize/ipfs-blockchain-watcher/version"
)

// superNodeCmd represents the superNode command
var superNodeCmd = &cobra.Command{
	Use:   "superNode",
	Short: "sync chain data into PG-IPFS",
	Long: `This command configures a VulcanizeDB ipfs-blockchain-watcher.

The Sync process streams all chain data from the appropriate chain, processes this data into IPLD objects
and publishes them to IPFS. It then indexes the CIDs against useful data fields/metadata in Postgres. 

The Serve process creates and exposes a rpc subscription server over ws and ipc. Transformers can subscribe to
these endpoints to stream

The BackFill process spins up a background process which periodically probes the Postgres database to identify
and fill in gaps in the data
`,
	Run: func(cmd *cobra.Command, args []string) {
		subCommand = cmd.CalledAs()
		logWithCommand = *log.WithField("SubCommand", subCommand)
		superNode()
	},
}

func superNode() {
	logWithCommand.Infof("running vdb version: %s", v.VersionWithMeta)
	logWithCommand.Debug("loading super node configuration variables")
	superNodeConfig, err := watcher.NewSuperNodeConfig()
	if err != nil {
		logWithCommand.Fatal(err)
	}
	logWithCommand.Infof("super node config: %+v", superNodeConfig)
	if superNodeConfig.IPFSMode == shared.LocalInterface {
		if err := ipfs.InitIPFSPlugins(); err != nil {
			logWithCommand.Fatal(err)
		}
	}
	wg := &sync.WaitGroup{}
	logWithCommand.Debug("initializing new super node service")
	superNode, err := watcher.NewSuperNode(superNodeConfig)
	if err != nil {
		logWithCommand.Fatal(err)
	}
	var forwardPayloadChan chan shared.ConvertedData
	if superNodeConfig.Serve {
		logWithCommand.Info("starting up super node servers")
		forwardPayloadChan = make(chan shared.ConvertedData, watcher.PayloadChanBufferSize)
		superNode.Serve(wg, forwardPayloadChan)
		if err := startServers(superNode, superNodeConfig); err != nil {
			logWithCommand.Fatal(err)
		}
	}
	if superNodeConfig.Sync {
		logWithCommand.Info("starting up super node sync process")
		if err := superNode.Sync(wg, forwardPayloadChan); err != nil {
			logWithCommand.Fatal(err)
		}
	}
	var backFiller watcher.BackFillInterface
	if superNodeConfig.BackFill {
		logWithCommand.Debug("initializing new super node backfill service")
		backFiller, err = watcher.NewBackFillService(superNodeConfig, forwardPayloadChan)
		if err != nil {
			logWithCommand.Fatal(err)
		}
		logWithCommand.Info("starting up super node backfill process")
		backFiller.BackFill(wg)
	}
	shutdown := make(chan os.Signal)
	signal.Notify(shutdown, os.Interrupt)
	<-shutdown
	if superNodeConfig.BackFill {
		backFiller.Stop()
	}
	superNode.Stop()
	wg.Wait()
}

func startServers(superNode watcher.SuperNode, settings *watcher.Config) error {
	logWithCommand.Debug("starting up IPC server")
	_, _, err := rpc.StartIPCEndpoint(settings.IPCEndpoint, superNode.APIs())
	if err != nil {
		return err
	}
	logWithCommand.Debug("starting up WS server")
	_, _, err = rpc.StartWSEndpoint(settings.WSEndpoint, superNode.APIs(), []string{"vdb"}, nil, true)
	if err != nil {
		return err
	}
	logWithCommand.Debug("starting up HTTP server")
	_, _, err = rpc.StartHTTPEndpoint(settings.HTTPEndpoint, superNode.APIs(), []string{settings.Chain.API()}, nil, nil, rpc.HTTPTimeouts{})
	return err
}

func init() {
	rootCmd.AddCommand(superNodeCmd)

	// flags for all config variables
	superNodeCmd.PersistentFlags().String("ipfs-path", "", "ipfs repository path")

	superNodeCmd.PersistentFlags().String("supernode-chain", "", "which chain to support, options are currently Ethereum or Bitcoin.")
	superNodeCmd.PersistentFlags().Bool("supernode-server", false, "turn vdb server on or off")
	superNodeCmd.PersistentFlags().String("supernode-ws-path", "", "vdb server ws path")
	superNodeCmd.PersistentFlags().String("supernode-http-path", "", "vdb server http path")
	superNodeCmd.PersistentFlags().String("supernode-ipc-path", "", "vdb server ipc path")
	superNodeCmd.PersistentFlags().Bool("supernode-sync", false, "turn vdb sync on or off")
	superNodeCmd.PersistentFlags().Int("supernode-workers", 0, "how many worker goroutines to publish and index data")
	superNodeCmd.PersistentFlags().Bool("supernode-back-fill", false, "turn vdb backfill on or off")
	superNodeCmd.PersistentFlags().Int("supernode-frequency", 0, "how often (in seconds) the backfill process checks for gaps")
	superNodeCmd.PersistentFlags().Int("supernode-batch-size", 0, "data fetching batch size")
	superNodeCmd.PersistentFlags().Int("supernode-batch-number", 0, "how many goroutines to fetch data concurrently")
	superNodeCmd.PersistentFlags().Int("supernode-validation-level", 0, "backfill will resync any data below this level")
	superNodeCmd.PersistentFlags().Int("supernode-timeout", 0, "timeout used for backfill http requests")

	superNodeCmd.PersistentFlags().String("btc-ws-path", "", "ws url for bitcoin node")
	superNodeCmd.PersistentFlags().String("btc-http-path", "", "http url for bitcoin node")
	superNodeCmd.PersistentFlags().String("btc-password", "", "password for btc node")
	superNodeCmd.PersistentFlags().String("btc-username", "", "username for btc node")
	superNodeCmd.PersistentFlags().String("btc-node-id", "", "btc node id")
	superNodeCmd.PersistentFlags().String("btc-client-name", "", "btc client name")
	superNodeCmd.PersistentFlags().String("btc-genesis-block", "", "btc genesis block hash")
	superNodeCmd.PersistentFlags().String("btc-network-id", "", "btc network id")

	superNodeCmd.PersistentFlags().String("eth-ws-path", "", "ws url for ethereum node")
	superNodeCmd.PersistentFlags().String("eth-http-path", "", "http url for ethereum node")
	superNodeCmd.PersistentFlags().String("eth-node-id", "", "eth node id")
	superNodeCmd.PersistentFlags().String("eth-client-name", "", "eth client name")
	superNodeCmd.PersistentFlags().String("eth-genesis-block", "", "eth genesis block hash")
	superNodeCmd.PersistentFlags().String("eth-network-id", "", "eth network id")

	// and their bindings
	viper.BindPFlag("ipfs.path", superNodeCmd.PersistentFlags().Lookup("ipfs-path"))

	viper.BindPFlag("superNode.chain", superNodeCmd.PersistentFlags().Lookup("supernode-chain"))
	viper.BindPFlag("superNode.server", superNodeCmd.PersistentFlags().Lookup("supernode-server"))
	viper.BindPFlag("superNode.wsPath", superNodeCmd.PersistentFlags().Lookup("supernode-ws-path"))
	viper.BindPFlag("superNode.httpPath", superNodeCmd.PersistentFlags().Lookup("supernode-http-path"))
	viper.BindPFlag("superNode.ipcPath", superNodeCmd.PersistentFlags().Lookup("supernode-ipc-path"))
	viper.BindPFlag("superNode.sync", superNodeCmd.PersistentFlags().Lookup("supernode-sync"))
	viper.BindPFlag("superNode.workers", superNodeCmd.PersistentFlags().Lookup("supernode-workers"))
	viper.BindPFlag("superNode.backFill", superNodeCmd.PersistentFlags().Lookup("supernode-back-fill"))
	viper.BindPFlag("superNode.frequency", superNodeCmd.PersistentFlags().Lookup("supernode-frequency"))
	viper.BindPFlag("superNode.batchSize", superNodeCmd.PersistentFlags().Lookup("supernode-batch-size"))
	viper.BindPFlag("superNode.batchNumber", superNodeCmd.PersistentFlags().Lookup("supernode-batch-number"))
	viper.BindPFlag("superNode.validationLevel", superNodeCmd.PersistentFlags().Lookup("supernode-validation-level"))
	viper.BindPFlag("superNode.timeout", superNodeCmd.PersistentFlags().Lookup("supernode-timeout"))

	viper.BindPFlag("bitcoin.wsPath", superNodeCmd.PersistentFlags().Lookup("btc-ws-path"))
	viper.BindPFlag("bitcoin.httpPath", superNodeCmd.PersistentFlags().Lookup("btc-http-path"))
	viper.BindPFlag("bitcoin.pass", superNodeCmd.PersistentFlags().Lookup("btc-password"))
	viper.BindPFlag("bitcoin.user", superNodeCmd.PersistentFlags().Lookup("btc-username"))
	viper.BindPFlag("bitcoin.nodeID", superNodeCmd.PersistentFlags().Lookup("btc-node-id"))
	viper.BindPFlag("bitcoin.clientName", superNodeCmd.PersistentFlags().Lookup("btc-client-name"))
	viper.BindPFlag("bitcoin.genesisBlock", superNodeCmd.PersistentFlags().Lookup("btc-genesis-block"))
	viper.BindPFlag("bitcoin.networkID", superNodeCmd.PersistentFlags().Lookup("btc-network-id"))

	viper.BindPFlag("ethereum.wsPath", superNodeCmd.PersistentFlags().Lookup("eth-ws-path"))
	viper.BindPFlag("ethereum.httpPath", superNodeCmd.PersistentFlags().Lookup("eth-http-path"))
	viper.BindPFlag("ethereum.nodeID", superNodeCmd.PersistentFlags().Lookup("eth-node-id"))
	viper.BindPFlag("ethereum.clientName", superNodeCmd.PersistentFlags().Lookup("eth-client-name"))
	viper.BindPFlag("ethereum.genesisBlock", superNodeCmd.PersistentFlags().Lookup("eth-genesis-block"))
	viper.BindPFlag("ethereum.networkID", superNodeCmd.PersistentFlags().Lookup("eth-network-id"))
}
