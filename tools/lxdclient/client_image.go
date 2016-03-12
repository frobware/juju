// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// +build go1.3

package lxdclient

import (
	"github.com/lxc/lxd"
	"github.com/lxc/lxd/shared"

	"github.com/juju/errors"
)

type rawImageClient interface {
	ListAliases() (shared.ImageAliases, error)
}

type imageClient struct {
	raw    rawImageClient
	config Config
}

func (i *imageClient) EnsureImageExists(series string) error {
	name := i.ImageNameForSeries(series)

	aliases, err := i.raw.ListAliases()
	if err != nil {
		return err
	}

	for _, alias := range aliases {
		if alias.Description == name {
			return nil
		}
	}

	ubuntu, err := lxdClientForCloudImages(i.config)
	if err != nil {
		return err
	}

	client, ok := i.raw.(*lxd.Client)
	if !ok {
		return errors.Errorf("can't use a fake client as target")
	}

	return ubuntu.CopyImage(series, client, false, []string{name}, false, true, nil)
}

// A common place to compute image names (alises) based on the series
func (i imageClient) ImageNameForSeries(series string) string {
	return "ubuntu-" + series
}
