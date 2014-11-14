// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package diskmanager

import (
	"reflect"

	"time"

	"github.com/juju/loggo"

	"github.com/juju/juju/storage"
	"github.com/juju/juju/version"
	"github.com/juju/juju/worker"
)

var logger = loggo.GetLogger("juju.worker.diskmanager")

const (
	// listBlockDevicesPeriod is the time period between block device listings.
	// Unfortunately Linux's inotify does not work with virtual filesystems, so
	// polling it is.
	listBlockDevicesPeriod = time.Second * 30

	// bytesInMiB is the number of bytes in a MiB.
	bytesInMiB = 1024 * 1024
)

var newNoOpWorker = worker.NewNoOpWorker

// BlockDeviceSetter is an interface that is supplied to
// NewWorker for setting block devices for the local host.
type BlockDeviceSetter interface {
	SetMachineBlockDevices([]storage.BlockDevice) error
}

// NewWorker returns a worker that lists block devices
// attached to the machine, and records them in state.
func NewWorker(b BlockDeviceSetter) worker.Worker {
	switch version.Current.OS {
	default:
		logger.Infof(
			"block device support has not been implemented for %s",
			version.Current.OS,
		)
		// Eventually we should support listing disks attached to
		// a Windows machine. For now, return a no-op worker.
		return newNoOpWorker()
	case version.Ubuntu:
		var old []storage.BlockDevice
		f := func(stop <-chan struct{}) error {
			return doWork(b, &old)
		}
		return worker.NewPeriodicWorker(f, listBlockDevicesPeriod)
	}
}

func doWork(b BlockDeviceSetter, old *[]storage.BlockDevice) error {
	blockDevices, err := listBlockDevices()
	if err != nil {
		return err
	}
	storage.SortBlockDevices(blockDevices)
	if reflect.DeepEqual(blockDevices, *old) {
		logger.Debugf("no changes to block devices detected")
		return nil
	}
	logger.Debugf("block devices changed: %v", blockDevices)
	if err := b.SetMachineBlockDevices(blockDevices); err != nil {
		return err
	}
	*old = blockDevices
	return nil
}
