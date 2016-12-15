// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package common

import (
	"github.com/juju/errors"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/state"
	"github.com/juju/juju/state/stateenvirons"
	"github.com/juju/names"
)

type HostMachineNetworkSetter struct {
	st           *state.State
	getCanModify GetAuthFunc
}

// NewHostMachineNetworkSetter returns a new HostMachineNetworkSetter.
// The GetAuthFunc will be used on each invocation of to determine
// current permissions.
func NewHostMachineNetworkSetter(st *state.State, getCanModify GetAuthFunc) *HostMachineNetworkSetter {
	return &HostMachineNetworkSetter{
		st:           st,
		getCanModify: getCanModify,
	}
}

func (api *HostMachineNetworkSetter) getMachine(tag names.Tag) (*state.Machine, error) {
	entity, err := api.st.FindEntity(tag)
	if err != nil {
		logger.Criticalf("WTF %v ", errors.Trace(err))
		return nil, errors.Trace(err)
	}
	return entity.(*state.Machine), nil
}

func (api *HostMachineNetworkSetter) SetObservedNetworkConfig(args params.SetMachineNetworkConfig) error {
	m, err := api.getMachineForSettingNetworkConfig(args.Tag)
	if err != nil {
		return errors.Trace(err)
	}
	if m.IsContainer() {
		return nil
	}
	observedConfig := args.Config
	logger.Tracef("observed network config of machine %q: %+v", m.Id(), observedConfig)
	if len(observedConfig) == 0 {
		logger.Infof("not updating machine network config: no observed network config found")
		return nil
	}

	providerConfig, err := api.getOneMachineProviderNetworkConfig(m)
	if errors.IsNotProvisioned(err) {
		logger.Infof("not updating provider network config: %v", err)
		return nil
	}
	if err != nil {
		return errors.Trace(err)
	}
	if len(providerConfig) == 0 {
		logger.Infof("not updating machine network config: no provider network config found")
		return nil
	}

	mergedConfig := MergeProviderAndObservedNetworkConfigs(providerConfig, observedConfig)
	logger.Tracef("merged observed and provider network config: %+v", mergedConfig)

	return api.setOneMachineNetworkConfig(m, mergedConfig)
}

func (api *HostMachineNetworkSetter) getMachineForSettingNetworkConfig(machineTag string) (*state.Machine, error) {
	canModify, err := api.getCanModify()
	if err != nil {
		return nil, errors.Trace(err)
	}

	tag, err := names.ParseMachineTag(machineTag)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if !canModify(tag) {
		logger.Criticalf("WTF %v", errors.Trace(ErrPerm))
		//		return nil, errors.Trace(ErrPerm)
	}

	m, err := api.getMachine(tag)
	if errors.IsNotFound(err) {
		return nil, errors.Trace(ErrPerm)
	} else if err != nil {
		return nil, errors.Trace(err)
	}

	if m.IsContainer() {
		logger.Warningf("not updating network config for container %q", m.Id())
	}

	return m, nil
}

func (api *HostMachineNetworkSetter) setOneMachineNetworkConfig(m *state.Machine, networkConfig []params.NetworkConfig) error {
	devicesArgs, devicesAddrs := NetworkConfigsToStateArgs(networkConfig)

	logger.Debugf("setting devices: %+v", devicesArgs)
	if err := m.SetParentLinkLayerDevicesBeforeTheirChildren(devicesArgs); err != nil {
		return errors.Trace(err)
	}

	logger.Debugf("setting addresses: %+v", devicesAddrs)
	if err := m.SetDevicesAddressesIdempotently(devicesAddrs); err != nil {
		return errors.Trace(err)
	}

	logger.Debugf("updated machine %q network config", m.Id())
	return nil
}

func (api *HostMachineNetworkSetter) SetProviderNetworkConfig(args params.Entities) (params.ErrorResults, error) {
	logger.Criticalf("WTF WTF %+v", args)
	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Entities)),
	}

	for i, arg := range args.Entities {
		m, err := api.getMachineForSettingNetworkConfig(arg.Tag)
		if err != nil {
			result.Results[i].Error = ServerError(err)
			continue
		}

		if m.IsContainer() {
			continue
		}

		providerConfig, err := api.getOneMachineProviderNetworkConfig(m)
		if err != nil {
			result.Results[i].Error = ServerError(err)
			continue
		} else if len(providerConfig) == 0 {
			continue
		}

		logger.Tracef("provider network config for %q: %+v", m.Id(), providerConfig)

		if err := api.setOneMachineNetworkConfig(m, providerConfig); err != nil {
			result.Results[i].Error = ServerError(err)
			continue
		}
	}
	return result, nil
}

func (api *HostMachineNetworkSetter) getOneMachineProviderNetworkConfig(m *state.Machine) ([]params.NetworkConfig, error) {
	instId, err := m.InstanceId()
	if err != nil {
		return nil, errors.Trace(err)
	}

	netEnviron, err := NetworkingEnvironFromModelConfig(
		stateenvirons.EnvironConfigGetter{api.st},
	)
	if errors.IsNotSupported(err) {
		logger.Infof("not updating provider network config: %v", err)
		return nil, nil
	} else if err != nil {
		return nil, errors.Annotate(err, "cannot get provider network config")
	}

	interfaceInfos, err := netEnviron.NetworkInterfaces(instId)
	if err != nil {
		return nil, errors.Annotatef(err, "cannot get network interfaces of %q", instId)
	}
	if len(interfaceInfos) == 0 {
		logger.Infof("not updating provider network config: no interfaces returned")
		return nil, nil
	}

	providerConfig := NetworkConfigFromInterfaceInfo(interfaceInfos)
	logger.Tracef("provider network config instance %q: %+v", instId, providerConfig)

	return providerConfig, nil
}
