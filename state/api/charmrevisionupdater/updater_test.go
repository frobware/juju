// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmrevisionupdater_test

import (
	"github.com/juju/charm"
	"github.com/juju/utils"
	gc "launchpad.net/gocheck"

	jujutesting "github.com/juju/juju/juju/testing"
	"github.com/juju/juju/state"
	"github.com/juju/juju/state/api/charmrevisionupdater"
	"github.com/juju/juju/state/apiserver/charmrevisionupdater/testing"
)

type versionUpdaterSuite struct {
	jujutesting.JujuConnSuite
	testing.CharmSuite

	updater *charmrevisionupdater.State
}

var _ = gc.Suite(&versionUpdaterSuite{})

func (s *versionUpdaterSuite) SetUpSuite(c *gc.C) {
	s.JujuConnSuite.SetUpSuite(c)
	s.CharmSuite.SetUpSuite(c, &s.JujuConnSuite)
}

func (s *versionUpdaterSuite) TearDownSuite(c *gc.C) {
	s.CharmSuite.TearDownSuite(c)
	s.JujuConnSuite.TearDownSuite(c)
}

func (s *versionUpdaterSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	s.CharmSuite.SetUpTest(c)

	machine, err := s.State.AddMachine("quantal", state.JobManageEnviron)
	c.Assert(err, gc.IsNil)
	password, err := utils.RandomPassword()
	c.Assert(err, gc.IsNil)
	err = machine.SetPassword(password)
	c.Assert(err, gc.IsNil)
	err = machine.SetProvisioned("i-manager", "fake_nonce", nil)
	c.Assert(err, gc.IsNil)
	st := s.OpenAPIAsMachine(c, machine.Tag().String(), password, "fake_nonce")
	c.Assert(st, gc.NotNil)

	s.updater = charmrevisionupdater.NewState(st)
	c.Assert(s.updater, gc.NotNil)
}

func (s *versionUpdaterSuite) TearDownTest(c *gc.C) {
	s.CharmSuite.TearDownTest(c)
	s.JujuConnSuite.TearDownTest(c)
}

func (s *versionUpdaterSuite) TestUpdateRevisions(c *gc.C) {
	s.SetupScenario(c)
	err := s.updater.UpdateLatestRevisions()
	c.Assert(err, gc.IsNil)

	curl := charm.MustParseURL("cs:quantal/mysql")
	pending, err := s.State.LatestPlaceholderCharm(curl)
	c.Assert(err, gc.IsNil)
	c.Assert(pending.String(), gc.Equals, "cs:quantal/mysql-23")
}
