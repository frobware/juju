// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package subnets

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/names"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/network"
)

var logger = loggo.GetLogger("juju.api.subnets")

const subnetsFacade = "Subnets"

// API provides access to the Subnets API facade.
type API struct {
	facade base.FacadeCaller
}

// NewAPI creates a new client-side Subnets facade.
func NewAPI(caller base.APICaller) *API {
	if caller == nil {
		panic("caller is nil")
	}
	facadeCaller := base.NewFacadeCaller(caller, subnetsFacade)
	return &API{
		facade: facadeCaller,
	}
}

// AddSubnet adds an existing subnet to the environment.
func (api *API) AddSubnet(subnet names.SubnetTag, providerId network.Id, space names.SpaceTag, zones []string) error {
	var response params.ErrorResults
	params := params.AddSubnetsParams{
		Subnets: []params.AddSubnetParams{{
			SubnetTag:        subnet.String(),
			SubnetProviderId: string(providerId),
			SpaceTag:         space.String(),
			Zones:            zones,
		}},
	}
	err := api.facade.FacadeCall("AddSubnets", params, &response)
	if err != nil {
		return errors.Trace(err)
	}
	return response.OneError()
}

// CreateSubnet creates a new subnet with the provider.
func (api *API) CreateSubnet(subnet names.SubnetTag, space names.SpaceTag, zones []string, isPublic bool) error {
	var response params.ErrorResults
	params := params.CreateSubnetsParams{
		Subnets: []params.CreateSubnetParams{{
			SubnetTag: subnet.String(),
			SpaceTag:  space.String(),
			Zones:     zones,
			IsPublic:  isPublic,
		}},
	}
	err := api.facade.FacadeCall("CreateSubnets", params, &response)
	if err != nil {
		return errors.Trace(err)
	}
	return response.OneError()
}

// ListSubnets fetches all the subnets known by the environment.
func (api *API) ListSubnets(spaceTag *names.SpaceTag, zone string) ([]params.Subnet, error) {
	var response params.ListSubnetsResults
	params := params.ListSubnetsParams{
		Filters: []params.ListSubnetsFilterParams{
			{SpaceTag: spaceTag.String(), Zone: zone},
		},
	}
	err := api.facade.FacadeCall("ListSubnets", params, &response)
	return response.Results, err
}