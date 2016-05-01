// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// +build go1.3

package lxd

import (
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/packaging/commands"
	"github.com/juju/utils/packaging/manager"
	"github.com/juju/utils/series"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/testing"
)

type InitialiserSuite struct {
	testing.BaseSuite
	calledCmds []string
}

var _ = gc.Suite(&InitialiserSuite{})

const lxdBridgeContent = `# WARNING: Don't modify this file by hand, it is generated by debconf!
# To update those values, please run "dpkg-reconfigure lxd"

# Whether to setup a new bridge
USE_LXD_BRIDGE="true"
EXISTING_BRIDGE=""

# Bridge name
LXD_BRIDGE="lxdbr0"

# dnsmasq configuration path
LXD_CONFILE=""

# dnsmasq domain
LXD_DOMAIN="lxd"

# IPv4
LXD_IPV4_ADDR="10.0.4.1"
LXD_IPV4_NETMASK="255.255.255.0"
LXD_IPV4_NETWORK="10.0.4.1/24"
LXD_IPV4_DHCP_RANGE="10.0.4.2,10.0.4.100"
LXD_IPV4_DHCP_MAX="50"
LXD_IPV4_NAT="true"

# IPv6
LXD_IPV6_ADDR="2001:470:b2b5:9999::1"
LXD_IPV6_MASK="64"
LXD_IPV6_NETWORK="2001:470:b2b5:9999::1/64"
LXD_IPV6_NAT="true"

# Proxy server
LXD_IPV6_PROXY="true"
`

// getMockRunCommandWithRetry is a helper function which returns a function
// with an identical signature to manager.RunCommandWithRetry which saves each
// command it recieves in a slice and always returns no output, error code 0
// and a nil error.
func getMockRunCommandWithRetry(calledCmds *[]string) func(string, func(string) error) (string, int, error) {
	return func(cmd string, fatalError func(string) error) (string, int, error) {
		*calledCmds = append(*calledCmds, cmd)
		return "", 0, nil
	}
}

func (s *InitialiserSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.calledCmds = []string{}
	s.PatchValue(&manager.RunCommandWithRetry, getMockRunCommandWithRetry(&s.calledCmds))
	s.PatchValue(&configureZFS, func() {})
	s.PatchValue(&configureLXDBridge, func() error { return nil })
}

func (s *InitialiserSuite) TestLTSSeriesPackages(c *gc.C) {
	// Momentarily, the only series with a dedicated cloud archive is precise,
	// which we will use for the following test:
	paccmder, err := commands.NewPackageCommander("trusty")
	c.Assert(err, jc.ErrorIsNil)

	s.PatchValue(&series.HostSeries, func() string { return "trusty" })
	container := NewContainerInitialiser("trusty")

	err = container.Initialise()
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(s.calledCmds, gc.DeepEquals, []string{
		paccmder.InstallCmd("--target-release", "trusty-backports", "lxd"),
	})
}

func (s *InitialiserSuite) TestNoSeriesPackages(c *gc.C) {
	// Here we want to test for any other series whilst avoiding the
	// possibility of hitting a cloud archive-requiring release.
	// As such, we simply pass an empty series.
	paccmder, err := commands.NewPackageCommander("xenial")
	c.Assert(err, jc.ErrorIsNil)

	container := NewContainerInitialiser("")

	err = container.Initialise()
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(s.calledCmds, gc.DeepEquals, []string{
		paccmder.InstallCmd("lxd"),
	})
}

func (s *InitialiserSuite) TestParseLXDBridgeFileValues(c *gc.C) {
	insignificantContent := `
# Comment 1, followed by empty line.

# Comment 2, followed by empty line.

  And a line that has content, but is not a comment, nor a key/value pair.
`
	for i, test := range []struct {
		desc     string
		content  string
		expected map[string]string
	}{{
		desc:     "empty content",
		content:  "",
		expected: map[string]string{},
	}, {
		desc:     "only comments and empty lines",
		content:  insignificantContent,
		expected: map[string]string{},
	}, {
		desc:     "missing key",
		content:  "=a",
		expected: map[string]string{},
	}, {
		desc:    "empty value",
		content: "a=",
		expected: map[string]string{
			"a": "",
		},
	}, {
		desc:    "value defined, but empty",
		content: `a=""`,
		expected: map[string]string{
			"a": "",
		},
	}, {
		desc:    "multiple entries",
		content: "a=b\nc=d\ne=f",
		expected: map[string]string{
			"a": "b",
			"c": "d",
			"e": "f",
		},
	}, {
		desc:    "comment with leading whitespace",
		content: " #a=b\nc=d\ne=f",
		expected: map[string]string{
			"c": "d",
			"e": "f",
		},
	}, {
		desc:    "key/value pairs with leading and trailing whitespace",
		content: " a=b\n c=d \ne=f ",
		expected: map[string]string{
			"a": "b",
			"c": "d",
			"e": "f",
		},
	}} {
		c.Logf("test #%d - %s", i, test.desc)
		values := parseLXDBridgeConfigValues(test.content)
		c.Check(values, gc.DeepEquals, test.expected)
	}
}

func (s *InitialiserSuite) TestParseLXDBridgeFileValuesWithRealWorldContent(c *gc.C) {
	expected := map[string]string{
		"USE_LXD_BRIDGE":      "true",
		"EXISTING_BRIDGE":     "",
		"LXD_BRIDGE":          "lxdbr0",
		"LXD_CONFILE":         "",
		"LXD_DOMAIN":          "lxd",
		"LXD_IPV4_ADDR":       "10.0.4.1",
		"LXD_IPV4_NETMASK":    "255.255.255.0",
		"LXD_IPV4_NETWORK":    "10.0.4.1/24",
		"LXD_IPV4_DHCP_RANGE": "10.0.4.2,10.0.4.100",
		"LXD_IPV4_DHCP_MAX":   "50",
		"LXD_IPV4_NAT":        "true",
		"LXD_IPV6_ADDR":       "2001:470:b2b5:9999::1",
		"LXD_IPV6_MASK":       "64",
		"LXD_IPV6_NETWORK":    "2001:470:b2b5:9999::1/64",
		"LXD_IPV6_NAT":        "true",
		"LXD_IPV6_PROXY":      "true",
	}
	values := parseLXDBridgeConfigValues(lxdBridgeContent)
	c.Check(values, gc.DeepEquals, expected)
}

func (s *InitialiserSuite) TestIsBridgeConfigured(c *gc.C) {
	for i, test := range []struct {
		desc     string
		content  string
		expected bool
	}{{
		desc:     "All missing",
		content:  "",
		expected: false,
	}, {
		desc: "missing USE_LXD_BRIDGE and LXD_BRIDGE",
		content: `
LXD_IPV4_ADDR=10.20.30.1`,
		expected: false,
	}, {
		desc: "missing LXD_BRIDGE missing",
		content: `
LXD_IPV4_ADDR=10.20.30.1
USE_LXD_BRIDGE=true`,
		expected: false,
	}, {
		desc: "bad IPv4 address",
		content: `
LXD_IPV4_ADDR="::1"
USE_LXD_BRIDGE="true"
LXD_BRIDGE="lxdbr0"`,
		expected: false,
	}, {
		desc: "USE_LXD_BRIDGE value != true",
		content: `
LXD_IPV4_ADDR="10.20.30.1"
USE_LXD_BRIDGE="nope"
LXD_BRIDGE="lxdbr0"`,
		expected: false,
	}, {
		desc: "USE_LXD_BRIDGE value not set",
		content: `
LXD_IPV4_ADDR="10.20.30.1"
USE_LXD_BRIDGE="
LXD_BRIDGE="lxdbr0"`,
		expected: false,
	}, {
		desc: "LXD_BRIDGE value not set",
		content: `
LXD_IPV4_ADDR="10.20.30.1"
USE_LXD_BRIDGE=true"
LXD_BRIDGE=""`,
		expected: false,
	}, {
		desc: "USE_LXD_BRIDGE, LXD_BRIDGE present, LXD_IPV4_ADDR missing",
		content: `
USE_LXD_BRIDGE=true"
LXD_BRIDGE=""`,
		expected: false,
	}, {
		desc: "All good",
		content: `
LXD_IPV4_ADDR="10.20.30.1"
USE_LXD_BRIDGE="true"
LXD_BRIDGE="lxdbr0"`,
		expected: true,
	}} {
		c.Logf("test #%d - %s", i, test.desc)
		result := isLXDBridgeConfigured(test.content)
		c.Check(result, gc.Equals, test.expected)
	}
}
