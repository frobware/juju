// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"github.com/juju/errors"
	"gopkg.in/juju/charm.v6-unstable"
	charmresource "gopkg.in/juju/charm.v6-unstable/resource"

	"github.com/juju/juju/resource"
)

// CharmstoreClient has the charm store API methods needed by resource
// state.
type CharmstoreClient interface {
	// ListResources lists the resources for each of the identified charms.
	ListResources(charmURLs []charm.URL) ([][]charmresource.Resource, error)

	// Close closes the client.
	Close() error
}

func getCharmstoreResources(client CharmstoreClient, cURL charm.URL, named map[string]resource.Resource) error {
	results, err := client.ListResources([]charm.URL{cURL})
	if err != nil {
		return errors.Trace(err)
	}
	if len(results) == 0 {
		return errors.Errorf("got bad results from charm store")
	}
	if len(results) > 1 {
		return errors.Errorf("got too many results from charm store")
	}
	csResources := results[0]

	for name, res := range named {
		for _, chRes := range csResources {
			if name == chRes.Name {
				res.Resource = chRes
				break
			}
		}
		// TODO(ericsnow) Fail if not found?
	}
	return nil
}
