// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package common

import (
	"net"

	"github.com/juju/errors"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/network"
	"github.com/juju/juju/state"
	"github.com/juju/utils/set"
)

// NetworkConfigFromInterfaceInfo converts a slice of network.InterfaceInfo into
// the equivalent params.NetworkConfig slice.
func NetworkConfigFromInterfaceInfo(interfaceInfos []network.InterfaceInfo) []params.NetworkConfig {
	result := make([]params.NetworkConfig, len(interfaceInfos))
	for i, v := range interfaceInfos {
		var dnsServers []string
		for _, nameserver := range v.DNSServers {
			dnsServers = append(dnsServers, nameserver.Value)
		}
		result[i] = params.NetworkConfig{
			DeviceIndex:         v.DeviceIndex,
			MACAddress:          v.MACAddress,
			CIDR:                v.CIDR,
			MTU:                 v.MTU,
			ProviderId:          string(v.ProviderId),
			ProviderSubnetId:    string(v.ProviderSubnetId),
			ProviderSpaceId:     string(v.ProviderSpaceId),
			ProviderVLANId:      string(v.ProviderVLANId),
			ProviderAddressId:   string(v.ProviderAddressId),
			VLANTag:             v.VLANTag,
			InterfaceName:       v.InterfaceName,
			ParentInterfaceName: v.ParentInterfaceName,
			InterfaceType:       string(v.InterfaceType),
			Disabled:            v.Disabled,
			NoAutoStart:         v.NoAutoStart,
			ConfigType:          string(v.ConfigType),
			Address:             v.Address.Value,
			DNSServers:          dnsServers,
			DNSSearchDomains:    v.DNSSearchDomains,
			GatewayAddress:      v.GatewayAddress.Value,
		}
	}
	return result
}

// NetworkConfigsToStateArgs splits the given networkConfig into a slice of
// state.LinkLayerDeviceArgs and a slice of state.LinkLayerDeviceAddress. The
// input is expected to come from MergeProviderAndObservedNetworkConfigs and to
// be sorted.
func NetworkConfigsToStateArgs(networkConfig []params.NetworkConfig) (
	[]state.LinkLayerDeviceArgs,
	[]state.LinkLayerDeviceAddress,
) {
	var devicesArgs []state.LinkLayerDeviceArgs
	var devicesAddrs []state.LinkLayerDeviceAddress

	logger.Tracef("transforming network config to state args: %+v", networkConfig)
	seenDeviceNames := set.NewStrings()
	for _, netConfig := range networkConfig {
		logger.Tracef("transforming device %q", netConfig.InterfaceName)
		if !seenDeviceNames.Contains(netConfig.InterfaceName) {
			// First time we see this, add it to devicesArgs.
			seenDeviceNames.Add(netConfig.InterfaceName)
			var mtu uint
			if netConfig.MTU >= 0 {
				mtu = uint(netConfig.MTU)
			}
			args := state.LinkLayerDeviceArgs{
				Name:        netConfig.InterfaceName,
				MTU:         mtu,
				ProviderID:  network.Id(netConfig.ProviderId),
				Type:        state.LinkLayerDeviceType(netConfig.InterfaceType),
				MACAddress:  netConfig.MACAddress,
				IsAutoStart: !netConfig.NoAutoStart,
				IsUp:        !netConfig.Disabled,
				ParentName:  netConfig.ParentInterfaceName,
			}
			logger.Tracef("state device args for device: %+v", args)
			devicesArgs = append(devicesArgs, args)
		}

		if netConfig.CIDR == "" || netConfig.Address == "" {
			logger.Tracef(
				"skipping empty CIDR %q and/or Address %q of %q",
				netConfig.CIDR, netConfig.Address, netConfig.InterfaceName,
			)
			continue
		}
		_, ipNet, err := net.ParseCIDR(netConfig.CIDR)
		if err != nil {
			logger.Warningf("FIXME: ignoring unexpected CIDR format %q: %v", netConfig.CIDR, err)
			continue
		}
		ipAddr := net.ParseIP(netConfig.Address)
		if ipAddr == nil {
			logger.Warningf("FIXME: ignoring unexpected Address format %q", netConfig.Address)
			continue
		}
		ipNet.IP = ipAddr
		cidrAddress := ipNet.String()

		var derivedConfigMethod state.AddressConfigMethod
		switch method := state.AddressConfigMethod(netConfig.ConfigType); method {
		case state.StaticAddress, state.DynamicAddress,
			state.LoopbackAddress, state.ManualAddress:
			derivedConfigMethod = method
		case "dhcp": // awkward special case
			derivedConfigMethod = state.DynamicAddress
		default:
			derivedConfigMethod = state.StaticAddress
		}

		addr := state.LinkLayerDeviceAddress{
			DeviceName:       netConfig.InterfaceName,
			ProviderID:       network.Id(netConfig.ProviderAddressId),
			ConfigMethod:     derivedConfigMethod,
			CIDRAddress:      cidrAddress,
			DNSServers:       netConfig.DNSServers,
			DNSSearchDomains: netConfig.DNSSearchDomains,
			GatewayAddress:   netConfig.GatewayAddress,
		}
		logger.Tracef("state address args for device: %+v", addr)
		devicesAddrs = append(devicesAddrs, addr)
	}
	logger.Tracef("seen devices: %+v", seenDeviceNames.SortedValues())
	logger.Tracef("network config transformed to state args:\n%+v\n%+v", devicesArgs, devicesAddrs)
	return devicesArgs, devicesAddrs
}

// NetworkingEnvironFromModelConfig constructs and returns
// environs.NetworkingEnviron using the given configGetter. Returns an error
// satisfying errors.IsNotSupported() if the model config does not support
// networking features.
func NetworkingEnvironFromModelConfig(configGetter environs.EnvironConfigGetter) (environs.NetworkingEnviron, error) {
	modelConfig, err := configGetter.ModelConfig()
	if err != nil {
		return nil, errors.Annotate(err, "failed to get model config")
	}
	if modelConfig.Type() == "dummy" {
		return nil, errors.NotSupportedf("dummy provider network config")
	}
	env, err := environs.GetEnviron(configGetter, environs.New)
	if err != nil {
		return nil, errors.Annotate(err, "failed to construct a model from config")
	}
	netEnviron, supported := environs.SupportsNetworking(env)
	if !supported {
		// " not supported" will be appended to the message below.
		return nil, errors.NotSupportedf("model %q networking", modelConfig.Name())
	}
	return netEnviron, nil
}

// MergeProviderAndObservedNetworkConfigs returns the effective network configs,
// using observedConfigs as a base and selectively updating it using the
// matching providerConfigs for each interface.
func MergeProviderAndObservedNetworkConfigs(providerConfigs, observedConfigs []params.NetworkConfig) []params.NetworkConfig {
	providerConfigByName := networkConfigsByName(providerConfigs)
	logger.Tracef("known provider config by name: %+v", providerConfigByName)

	providerConfigByAddress := networkConfigsByAddress(providerConfigs)
	logger.Tracef("known provider config by address: %+v", providerConfigByAddress)

	var results []params.NetworkConfig
	for _, observed := range observedConfigs {

		name, ipAddress := observed.InterfaceName, observed.Address
		finalConfig := observed

		providerConfig, known := providerConfigByName[name]
		if known {
			finalConfig = mergeObservedAndProviderInterfaceConfig(finalConfig, providerConfig)
			logger.Debugf("updated observed interface config for %q with: %+v", name, providerConfig)
		}

		providerConfig, known = providerConfigByAddress[ipAddress]
		if known {
			finalConfig = mergeObservedAndProviderAddressConfig(finalConfig, providerConfig)
			logger.Debugf("updated observed address config for %q with: %+v", name, providerConfig)
		}

		results = append(results, finalConfig)
		logger.Debugf("merged config for %q: %+v", name, finalConfig)
	}

	return results
}

func networkConfigsByName(input []params.NetworkConfig) map[string]params.NetworkConfig {
	configsByName := make(map[string]params.NetworkConfig, len(input))
	for _, config := range input {
		configsByName[config.InterfaceName] = config
	}
	return configsByName
}

func networkConfigsByAddress(input []params.NetworkConfig) map[string]params.NetworkConfig {
	configsByAddress := make(map[string]params.NetworkConfig, len(input))
	for _, config := range input {
		configsByAddress[config.Address] = config
	}
	return configsByAddress
}

func mergeObservedAndProviderInterfaceConfig(observedConfig, providerConfig params.NetworkConfig) params.NetworkConfig {
	finalConfig := observedConfig

	// The following fields cannot be observed and are only known by the
	// provider.
	finalConfig.ProviderId = providerConfig.ProviderId
	finalConfig.ProviderVLANId = providerConfig.ProviderVLANId
	finalConfig.ProviderSubnetId = providerConfig.ProviderSubnetId

	// The following few fields are only updated if their observed values are
	// empty.

	if observedConfig.InterfaceType == "" {
		finalConfig.InterfaceType = providerConfig.InterfaceType
	}

	if observedConfig.VLANTag == 0 {
		finalConfig.VLANTag = providerConfig.VLANTag
	}

	if observedConfig.ParentInterfaceName == "" {
		finalConfig.ParentInterfaceName = providerConfig.ParentInterfaceName
	}

	return finalConfig
}

func mergeObservedAndProviderAddressConfig(observedConfig, providerConfig params.NetworkConfig) params.NetworkConfig {
	finalConfig := observedConfig

	// The following fields cannot be observed and are only known by the
	// provider.
	finalConfig.ProviderAddressId = providerConfig.ProviderAddressId
	finalConfig.ProviderSubnetId = providerConfig.ProviderSubnetId
	finalConfig.ProviderSpaceId = providerConfig.ProviderSpaceId

	// The following few fields are only updated if their observed values are
	// empty.

	if observedConfig.ProviderVLANId == "" {
		finalConfig.ProviderVLANId = providerConfig.ProviderVLANId
	}

	if observedConfig.VLANTag == 0 {
		finalConfig.VLANTag = providerConfig.VLANTag
	}

	if observedConfig.ConfigType == "" {
		finalConfig.ConfigType = providerConfig.ConfigType
	}

	if observedConfig.CIDR == "" {
		finalConfig.CIDR = providerConfig.CIDR
	}

	if observedConfig.GatewayAddress == "" {
		finalConfig.GatewayAddress = providerConfig.GatewayAddress
	}

	if len(observedConfig.DNSServers) == 0 {
		finalConfig.DNSServers = providerConfig.DNSServers
	}

	if len(observedConfig.DNSSearchDomains) == 0 {
		finalConfig.DNSSearchDomains = providerConfig.DNSSearchDomains
	}

	return finalConfig
}
