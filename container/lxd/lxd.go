// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// +build go1.3

package lxd

import (
	"fmt"
	"os/exec"

	"github.com/juju/errors"
	"github.com/juju/loggo"

	"github.com/juju/juju/cloudconfig/containerinit"
	"github.com/juju/juju/cloudconfig/instancecfg"
	"github.com/juju/juju/constraints"
	"github.com/juju/juju/container"
	"github.com/juju/juju/instance"
	"github.com/juju/juju/network"
	"github.com/juju/juju/status"
	"github.com/juju/juju/tools/lxdclient"
)

var (
	logger = loggo.GetLogger("juju.container.lxd")
)

type interfaceArity int

const (
	noNIC interfaceArity = iota
	singleNIC
	multiNIC
)

const lxdDefaultProfileName = "default"

// XXX: should we allow managing containers on other hosts? this is
// functionality LXD gives us and from discussion juju would use eventually for
// the local provider, so the APIs probably need to be changed to pass extra
// args around. I'm punting for now.
type containerManager struct {
	modelUUID string
	namespace instance.Namespace
	// A cached client.
	client *lxdclient.Client
}

// containerManager implements container.Manager.
var _ container.Manager = (*containerManager)(nil)

func ConnectLocal() (*lxdclient.Client, error) {
	cfg := lxdclient.Config{
		Remote: lxdclient.Local,
	}

	cfg, err := cfg.WithDefaults()
	if err != nil {
		return nil, errors.Trace(err)
	}

	client, err := lxdclient.Connect(cfg)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return client, nil
}

// NewContainerManager creates the entity that knows how to create and manage
// LXD containers.
// TODO(jam): This needs to grow support for things like LXC's ImageURLGetter
// functionality.
func NewContainerManager(conf container.ManagerConfig) (container.Manager, error) {
	modelUUID := conf.PopValue(container.ConfigModelUUID)
	if modelUUID == "" {
		return nil, errors.Errorf("model UUID is required")
	}
	namespace, err := instance.NewNamespace(modelUUID)
	if err != nil {
		return nil, errors.Trace(err)
	}

	conf.WarnAboutUnused()
	return &containerManager{
		modelUUID: modelUUID,
		namespace: namespace,
	}, nil
}

// Namespace implements container.Manager.
func (manager *containerManager) Namespace() instance.Namespace {
	return manager.namespace
}

func (manager *containerManager) CreateContainer(
	instanceConfig *instancecfg.InstanceConfig,
	cons constraints.Value,
	series string,
	networkConfig *container.NetworkConfig,
	storageConfig *container.StorageConfig,
	callback container.StatusCallback,
) (inst instance.Instance, _ *instance.HardwareCharacteristics, err error) {

	defer func() {
		if err != nil {
			callback(status.StatusProvisioningError, fmt.Sprintf("Creating container: %v", err), nil)
		}
	}()

	if manager.client == nil {
		manager.client, err = ConnectLocal()
		if err != nil {
			err = errors.Annotatef(err, "failed to connect to local LXD")
			return
		}
	}

	err = manager.client.EnsureImageExists(series,
		lxdclient.DefaultImageSources,
		func(progress string) {
			callback(status.StatusProvisioning, progress, nil)
		})
	if err != nil {
		err = errors.Annotatef(err, "failed to ensure LXD image")
		return
	}

	name, err := manager.namespace.Hostname(instanceConfig.MachineId)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	userData, err := containerinit.CloudInitUserData(instanceConfig, networkConfig)
	if err != nil {
		return
	}

	metadata := map[string]string{
		lxdclient.UserdataKey: string(userData),
		// An extra piece of info to let people figure out where this
		// thing came from.
		"user.juju-model": manager.modelUUID,

		// Make sure these come back up on host reboot.
		"boot.autostart": "true",
	}

	profiles := []string{}
	files := make(lxdclient.Files, 0)
	devices := make(lxdclient.Devices)
	interfaceArity := networkConfiguration(networkConfig)

	if interfaceArity == noNIC || interfaceArity == singleNIC {
		err = errors.Annotatef(err, "no network configuration")
		return
	}

	devices, err = networkDevices(networkConfig)

	if err != nil {
		return
	}

	switch interfaceArity {
	case multiNIC:
		files = append(files,
			lxdclient.File{
				Content: []byte("auto lo\niface lo inet loopback\n"),
				Path:    "/etc/network/interfaces",
				Mode:    0644,
			},
			lxdclient.File{
				Content: []byte("# Content removed by Juju.\n"),
				Path:    "/etc/network/interfaces.d/50-cloud-init.cfg",
				Mode:    0644,
			},
			lxdclient.File{
				Content: []byte("# Content removed by Juju.\n"),
				Path:    "/etc/network/interfaces.d/eth0.cfg",
				Mode:    0644,
			},
			lxdclient.File{
				Content: []byte("network: {config: disabled}\n"),
				Path:    "/etc/cloud/cloud.cfg.d/99-juju-no-cloud-init-networking.cfg",
				Mode:    0644,
			},
		)
	}

	logger.Infof("instance %q configured with %v network devices", name, devices)

	spec := lxdclient.InstanceSpec{
		Name:     name,
		Image:    manager.client.ImageNameForSeries(series),
		Metadata: metadata,
		Devices:  devices,
		Profiles: profiles,
		Files:    files,
	}

	logger.Infof("starting instance %q (image %q)...", spec.Name, spec.Image)
	callback(status.StatusProvisioning, "Starting container", nil)

	_, err = manager.client.AddInstance(spec, func(spec lxdclient.InstanceSpec) error {
		switch interfaceArity {
		case multiNIC:
			return applyPatchForLP1590104(spec)
		default:
			return nil
		}
	})

	if err != nil {
		return
	}

	callback(status.StatusRunning, "Container started", nil)
	inst = &lxdInstance{name, manager.client}
	return
}

func (manager *containerManager) DestroyContainer(id instance.Id) error {
	if manager.client == nil {
		var err error
		manager.client, err = ConnectLocal()
		if err != nil {
			return err
		}
	}
	return errors.Trace(manager.client.RemoveInstances(manager.namespace.Prefix(), string(id)))
}

func (manager *containerManager) ListContainers() (result []instance.Instance, err error) {
	result = []instance.Instance{}
	if manager.client == nil {
		manager.client, err = ConnectLocal()
		if err != nil {
			return
		}
	}

	lxdInstances, err := manager.client.Instances(manager.namespace.Prefix())
	if err != nil {
		return
	}

	for _, i := range lxdInstances {
		result = append(result, &lxdInstance{i.Name, manager.client})
	}

	return
}

func (manager *containerManager) IsInitialized() bool {
	if manager.client != nil {
		return true
	}

	// NewClient does a roundtrip to the server to make sure it understands
	// the versions, so all we need to do is connect above and we're done.
	var err error
	manager.client, err = ConnectLocal()
	return err == nil
}

// HasLXDSupport returns false when this juju binary was not built with LXD
// support (i.e. it was built on a golang version < 1.2
func HasLXDSupport() bool {
	return true
}

func nicDevice(deviceName, parentDevice, hwAddr string, mtu int) (lxdclient.Device, error) {
	device := make(lxdclient.Device)

	device["type"] = "nic"
	device["nictype"] = "bridged"

	if deviceName == "" {
		return nil, errors.Errorf("invalid device name")
	}
	device["name"] = deviceName

	if parentDevice == "" {
		return nil, errors.Errorf("invalid parent device name")
	}
	device["parent"] = parentDevice

	if hwAddr != "" {
		device["hwaddr"] = hwAddr
	}

	if mtu > 0 {
		device["mtu"] = fmt.Sprintf("%v", mtu)
	}

	return device, nil
}

func networkDevices(networkConfig *container.NetworkConfig) (lxdclient.Devices, error) {
	nics := make(lxdclient.Devices)

	switch networkConfiguration(networkConfig) {
	case multiNIC:
		for _, v := range networkConfig.Interfaces {
			if v.InterfaceType == network.LoopbackInterface {
				continue
			}
			if v.InterfaceType != network.EthernetInterface {
				return nil, errors.Errorf("interface type %q not supported", v.InterfaceType)
			}
			parentDevice := v.ParentInterfaceName
			if parentDevice == "" {
				// This happens on AWS when the
				// address-allocation feature flag is
				// enabled.
				parentDevice = networkConfig.Device
			}
			device, err := nicDevice(v.InterfaceName, parentDevice, v.MACAddress, v.MTU)
			if err != nil {
				return nil, errors.Trace(err)
			}
			nics[v.InterfaceName] = device
		}
	case singleNIC:
		device, err := nicDevice("eth0", networkConfig.Device, "", networkConfig.MTU)
		if err != nil {
			return nil, errors.Trace(err)
		}
		nics["eth0"] = device
	}
	return nics, nil
}

func networkConfiguration(networkConfig *container.NetworkConfig) interfaceArity {
	switch {
	case len(networkConfig.Interfaces) > 0:
		return multiNIC
	case networkConfig.Device != "":
		return singleNIC
	default:
		return noNIC
	}
}

func applyPatchForLP1590104(spec lxdclient.InstanceSpec) error {
	script := `set -eux
if [ $(lsb_release -cs) == "xenial" ]; then
  name="$1"
  local_f=$(mktemp)
  trap "rm -f $local_f $local_f.dist" EXIT
  f="/usr/lib/python3/dist-packages/cloudinit/stages.py"
  line="for loc, ncfg in (cmdline_cfg, dscfg, sys_cfg)"
  lxc file pull "$name/$f" "$local_f"
# If patch is unsuccessful then it may have landed in cloud-init.
  sed -i.dist "/$line/s/dscfg, sys_cfg/sys_cfg, dscfg/" "$local_f" || exit 0
  diff -u "$local_f.dist" "$local_f" && { echo "patch failed"; exit 1; }
  lxc file push "$local_f" "$name/$f"
fi
`
	out, err := exec.Command("sudo",
		"/bin/bash",
		"-c",
		script,
		"--",
		spec.Name,
	).CombinedOutput()

	if err != nil {
		logger.Infof("Failed to fix LP1590104: %q", string(out))
	} else {
		logger.Infof("Successfully fixed LP1590104: %q", string(out))
	}

	return err
}
