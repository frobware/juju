// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// +build go1.3

package lxd

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/juju/errors"

	"github.com/juju/utils/packaging/config"
	"github.com/juju/utils/packaging/manager"

	"github.com/juju/juju/container"
)

const lxdBridgeFile = "/etc/default/lxd-bridge"

var requiredPackages = []string{
	"lxd",
}

var xenialPackages = []string{
	"zfsutils-linux",
}

type containerInitialiser struct {
	series string
}

// containerInitialiser implements container.Initialiser.
var _ container.Initialiser = (*containerInitialiser)(nil)

// NewContainerInitialiser returns an instance used to perform the steps
// required to allow a host machine to run a LXC container.
func NewContainerInitialiser(series string) container.Initialiser {
	return &containerInitialiser{series}
}

// Initialise is specified on the container.Initialiser interface.
func (ci *containerInitialiser) Initialise() error {
	err := ensureDependencies(ci.series)
	if err != nil {
		return err
	}

	err = configureLXDBridge()
	if err != nil {
		return err
	}

	if ci.series >= "xenial" {
		configureZFS()
	}

	return nil
}

// getPackageManager is a helper function which returns the
// package manager implementation for the current system.
func getPackageManager(series string) (manager.PackageManager, error) {
	return manager.NewPackageManager(series)
}

// getPackagingConfigurer is a helper function which returns the
// packaging configuration manager for the current system.
func getPackagingConfigurer(series string) (config.PackagingConfigurer, error) {
	return config.NewPackagingConfigurer(series)
}

var configureZFS = func() {
	/* create a 100 GB pool by default (sparse, so it won't actually fill
	 * that immediately)
	 */
	output, err := exec.Command(
		"lxd",
		"init",
		"--auto",
		"--storage-backend", "zfs",
		"--storage-pool", "lxd",
		"--storage-create-loop", "100",
	).CombinedOutput()

	if err != nil {
		logger.Warningf("configuring zfs failed with %s: %s", err, string(output))
	}
}

var configureLXDBridge = func() error {
	isBridgeConfigured, err := isLXDBridgeFileConfigured(lxdBridgeFile)

	if err != nil {
		return errors.Annotatef(err, "failed to read LXD configuration from %s", lxdBridgeFile)
	}

	if isBridgeConfigured {
		logger.Infof("LXD bridge configuration is complete")
		return nil
	}

	cmd := "dpkg-reconfigure"
	cmdArgs := []string{"--frontend", "noninteractive", "--priority", "med", "lxd"}

	logger.Warningf("Running %s to setup LXD bridge configuration: ", cmd, strings.Join(cmdArgs, " "))

	output, err := exec.Command(cmd, cmdArgs...).CombinedOutput()

	if err != nil {
		logger.Errorf("%s %q failed with %s: %q", cmd, strings.Join(cmdArgs, " "), err, string(output))
		return nil
	}

	isBridgeConfigured, err = isLXDBridgeFileConfigured(lxdBridgeFile)

	if err != nil {
		return errors.Annotatef(err, "failed to read LXD configuration from %s", lxdBridgeFile)
	}

	if !isBridgeConfigured {
		msg := fmt.Sprintf("LXD bridge configuration incomplete post %s %q", cmd, strings.Join(cmdArgs, " "))
		return errors.New(msg)
	}

	/* non-systemd systems don't have the lxd-bridge service, so this always fails */
	_ = exec.Command("service", "lxd-bridge", "restart").Run()
	return exec.Command("service", "lxd", "restart").Run()
}

var interfaceAddrs = func() ([]net.Addr, error) {
	return net.InterfaceAddrs()
}

// ensureDependencies creates a set of install packages using
// apt.GetPreparePackages and runs each set of packages through
// apt.GetInstall.
func ensureDependencies(series string) error {
	if series == "precise" {
		return fmt.Errorf("LXD is not supported in precise.")
	}

	pacman, err := getPackageManager(series)
	if err != nil {
		return err
	}
	pacconfer, err := getPackagingConfigurer(series)
	if err != nil {
		return err
	}

	for _, pack := range requiredPackages {
		pkg := pack
		if config.SeriesRequiresCloudArchiveTools(series) &&
			pacconfer.IsCloudArchivePackage(pack) {
			pkg = strings.Join(pacconfer.ApplyCloudArchiveTarget(pack), " ")
		}

		if config.RequiresBackports(series, pack) {
			pkg = fmt.Sprintf("--target-release %s-backports %s", series, pkg)
		}

		if err := pacman.Install(pkg); err != nil {
			return err
		}
	}

	if series >= "xenial" {
		for _, pack := range xenialPackages {
			pacman.Install(fmt.Sprintf("--no-install-recommends %s", pack))
		}
	}

	return err
}

func parseLXDBridgeConfigValues(input string) map[string]string {
	values := make(map[string]string)

	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}

		tokens := strings.Split(line, "=")

		if tokens[0] == "" {
			continue // no key
		}

		value := ""

		if len(tokens) > 1 {
			value = tokens[1]
			if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
				value = strings.Trim(value, `"`)
			}
		}

		values[tokens[0]] = value
	}
	return values
}

func isLXDBridgeConfigured(input string) bool {
	var haveIPv4Address = false
	var useLXDBridgeIsTrue = false
	var haveLXDBridgeDevice = false

	values := parseLXDBridgeConfigValues(input)

	if val, found := values["LXD_IPV4_ADDR"]; found {
		ipAddr := net.ParseIP(val)
		haveIPv4Address = ipAddr != nil && ipAddr.To4() != nil
	}

	if val, found := values["USE_LXD_BRIDGE"]; found {
		useLXDBridgeIsTrue = val == "true"
	}

	if val, found := values["LXD_BRIDGE"]; found {
		haveLXDBridgeDevice = len(val) > 0
	}

	return haveIPv4Address && useLXDBridgeIsTrue && haveLXDBridgeDevice
}

func isLXDBridgeFileConfigured(filename string) (bool, error) {
	content, err := readBridgeConfiguration(filename)

	if err != nil {
		return false, errors.Trace(err)
	}

	return isLXDBridgeConfigured(content), nil
}

func readBridgeConfiguration(filename string) (string, error) {
	f, err := os.OpenFile(filename, os.O_RDONLY, 0777)

	if err != nil {
		return "", errors.Trace(err)
	}

	defer f.Close()

	content, err := ioutil.ReadAll(f)

	if err != nil {
		return "", errors.Trace(err)
	}

	return string(content), nil
}
