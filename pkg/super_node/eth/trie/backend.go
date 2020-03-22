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

package trie

import (
	"github.com/vulcanize/vulcanizedb/pkg/postgres"
	"github.com/vulcanize/vulcanizedb/pkg/super_node/eth"
)

// Backend is the struct for performing top-level trie processes
type Backend struct {
	Retriever *eth.CIDRetriever
	Fetcher   *eth.IPLDFetcher
	DB        *postgres.DB
}

func NewEthBackend(db *postgres.DB, ipfsPath string) (*Backend, error) {
	r := eth.NewCIDRetriever(db)
	f, err := eth.NewIPLDFetcher(ipfsPath)
	if err != nil {
		return nil, err
	}
	return &Backend{
		Retriever: r,
		Fetcher:   f,
		DB:        db,
	}, nil
}