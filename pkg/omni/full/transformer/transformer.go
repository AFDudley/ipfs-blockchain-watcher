// VulcanizeDB
// Copyright © 2018 Vulcanize

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

package transformer

import (
	"errors"
	"strings"

	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres/repositories"
	"github.com/vulcanize/vulcanizedb/pkg/omni/full/converter"
	"github.com/vulcanize/vulcanizedb/pkg/omni/full/retriever"
	"github.com/vulcanize/vulcanizedb/pkg/omni/shared/contract"
	"github.com/vulcanize/vulcanizedb/pkg/omni/shared/parser"
	"github.com/vulcanize/vulcanizedb/pkg/omni/shared/poller"
	"github.com/vulcanize/vulcanizedb/pkg/omni/shared/repository"
	"github.com/vulcanize/vulcanizedb/pkg/omni/shared/types"
)

// Requires a fully synced vDB and a running eth node (or infura)
type transformer struct {
	// Database interfaces
	datastore.FilterRepository       // Log filters repo; accepts filters generated by Contract.GenerateFilters()
	datastore.WatchedEventRepository // Watched event log views, created by the log filters
	repository.EventRepository       // Holds transformed watched event log data

	// Pre-processing interfaces
	parser.Parser            // Parses events and methods out of contract abi fetched using contract address
	retriever.BlockRetriever // Retrieves first block for contract and current block height

	// Processing interfaces
	converter.Converter // Converts watched event logs into custom log
	poller.Poller       // Polls methods using contract's token holder addresses and persists them using method datastore

	// Ethereum network name; default "" is mainnet
	Network string

	// Store contract info as mapping to contract address
	Contracts map[string]*contract.Contract

	// Targeted subset of events/methods
	// Stored as map sof contract address to events/method names of interest
	WatchedEvents map[string][]string // Default/empty event list means all are watched
	WantedMethods map[string][]string // Default/empty method list means none are polled

	// Starting block for contracts
	ContractStart map[string]int64

	// Lists of addresses to filter event or method data
	// before persisting; if empty no filter is applied
	EventArgs  map[string][]string
	MethodArgs map[string][]string

	// Whether or not to create a list of emitted address or hashes for the contract in postgres
	CreateAddrList map[string]bool
	CreateHashList map[string]bool

	// Method piping on/off for a contract
	Piping map[string]bool
}

// Transformer takes in config for blockchain, database, and network id
func NewTransformer(network string, BC core.BlockChain, DB *postgres.DB) *transformer {
	return &transformer{
		Poller:                 poller.NewPoller(BC, DB, types.FullSync),
		Parser:                 parser.NewParser(network),
		BlockRetriever:         retriever.NewBlockRetriever(DB),
		Converter:              converter.NewConverter(&contract.Contract{}),
		Contracts:              map[string]*contract.Contract{},
		WatchedEventRepository: repositories.WatchedEventRepository{DB: DB},
		FilterRepository:       repositories.FilterRepository{DB: DB},
		EventRepository:        repository.NewEventRepository(DB, types.FullSync),
		WatchedEvents:          map[string][]string{},
		WantedMethods:          map[string][]string{},
		ContractStart:          map[string]int64{},
		EventArgs:              map[string][]string{},
		MethodArgs:             map[string][]string{},
		CreateAddrList:         map[string]bool{},
		CreateHashList:         map[string]bool{},
		Piping:                 map[string]bool{},
	}
}

// Use after creating and setting transformer
// Loops over all of the addr => filter sets
// Uses parser to pull event info from abi
// Use this info to generate event filters
func (t *transformer) Init() error {
	for contractAddr, subset := range t.WatchedEvents {
		// Get Abi
		err := t.Parser.Parse(contractAddr)
		if err != nil {
			return err
		}

		// Get first block and most recent block number in the header repo
		firstBlock, err := t.BlockRetriever.RetrieveFirstBlock(contractAddr)
		if err != nil {
			return err
		}
		lastBlock, err := t.BlockRetriever.RetrieveMostRecentBlock()
		if err != nil {
			return err
		}

		// Set to specified range if it falls within the bounds
		if firstBlock < t.ContractStart[contractAddr] {
			firstBlock = t.ContractStart[contractAddr]
		}

		// Get contract name if it has one
		var name = new(string)
		t.FetchContractData(t.Abi(), contractAddr, "name", nil, &name, lastBlock)

		// Remove any potential accidental duplicate inputs in arg filter values
		eventArgs := map[string]bool{}
		for _, arg := range t.EventArgs[contractAddr] {
			eventArgs[arg] = true
		}
		methodArgs := map[string]bool{}
		for _, arg := range t.MethodArgs[contractAddr] {
			methodArgs[arg] = true
		}

		// Aggregate info into contract object
		info := contract.Contract{
			Name:           *name,
			Network:        t.Network,
			Address:        contractAddr,
			Abi:            t.Parser.Abi(),
			ParsedAbi:      t.Parser.ParsedAbi(),
			StartingBlock:  firstBlock,
			LastBlock:      lastBlock,
			Events:         t.Parser.GetEvents(subset),
			Methods:        t.Parser.GetSelectMethods(t.WantedMethods[contractAddr]),
			FilterArgs:     eventArgs,
			MethodArgs:     methodArgs,
			CreateAddrList: t.CreateAddrList[contractAddr],
			CreateHashList: t.CreateHashList[contractAddr],
			Piping:         t.Piping[contractAddr],
		}.Init()

		// Use info to create filters
		err = info.GenerateFilters()
		if err != nil {
			return err
		}

		// Iterate over filters and push them to the repo using filter repository interface
		for _, filter := range info.Filters {
			err = t.CreateFilter(filter)
			if err != nil {
				return err
			}
		}

		// Store contract info for further processing
		t.Contracts[contractAddr] = info
	}

	return nil
}

// Iterates through stored, initialized contract objects
// Iterates through contract's event filters, grabbing watched event logs
// Uses converter to convert logs into custom log type
// Persists converted logs into custuom postgres tables
// Calls selected methods, using token holder address generated during event log conversion
func (tr transformer) Execute() error {
	if len(tr.Contracts) == 0 {
		return errors.New("error: transformer has no initialized contracts to work with")
	}
	// Iterate through all internal contracts
	for _, con := range tr.Contracts {
		// Update converter with current contract
		tr.Update(con)

		// Iterate through contract filters and get watched event logs
		for eventSig, filter := range con.Filters {
			watchedEvents, err := tr.GetWatchedEvents(filter.Name)
			if err != nil {
				return err
			}

			// Iterate over watched event logs
			for _, we := range watchedEvents {
				// Convert them to our custom log type
				cstm, err := tr.Converter.Convert(*we, con.Events[eventSig])
				if err != nil {
					return err
				}
				if cstm == nil {
					continue
				}

				// If log is not empty, immediately persist in repo
				// Run this in seperate goroutine?
				err = tr.PersistLogs([]types.Log{*cstm}, con.Events[eventSig], con.Address, con.Name)
				if err != nil {
					return err
				}
			}
		}

		// After persisting all watched event logs
		// poller polls select contract methods
		// and persists the results into custom pg tables
		// Run this in seperate goroutine?
		if err := tr.PollContract(*con); err != nil {
			return err
		}
	}

	return nil
}

// Used to set which contract addresses and which of their events to watch
func (tr *transformer) SetEvents(contractAddr string, filterSet []string) {
	tr.WatchedEvents[strings.ToLower(contractAddr)] = filterSet
}

// Used to set subset of account addresses to watch events for
func (tr *transformer) SetEventArgs(contractAddr string, filterSet []string) {
	tr.EventArgs[strings.ToLower(contractAddr)] = filterSet
}

// Used to set which contract addresses and which of their methods to call
func (tr *transformer) SetMethods(contractAddr string, filterSet []string) {
	tr.WantedMethods[strings.ToLower(contractAddr)] = filterSet
}

// Used to set subset of account addresses to poll methods on
func (tr *transformer) SetMethodArgs(contractAddr string, filterSet []string) {
	tr.MethodArgs[strings.ToLower(contractAddr)] = filterSet
}

// Used to set the block range to watch for a given address
func (tr *transformer) SetStartingBlock(contractAddr string, start int64) {
	tr.ContractStart[strings.ToLower(contractAddr)] = start
}

// Used to set whether or not to persist an account address list
func (tr *transformer) SetCreateAddrList(contractAddr string, on bool) {
	tr.CreateAddrList[strings.ToLower(contractAddr)] = on
}

// Used to set whether or not to persist an hash list
func (tr *transformer) SetCreateHashList(contractAddr string, on bool) {
	tr.CreateHashList[strings.ToLower(contractAddr)] = on
}

// Used to turn method piping on for a contract
func (tr *transformer) SetPiping(contractAddr string, on bool) {
	tr.Piping[strings.ToLower(contractAddr)] = on
}