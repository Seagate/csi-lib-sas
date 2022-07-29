/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package sas

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"path"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

type ioHandler interface {
	ReadDir(dirname string) ([]os.FileInfo, error)
	Lstat(name string) (os.FileInfo, error)
	EvalSymlinks(path string) (string, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
}

// Connector provides a struct to hold all of the needed parameters to make our SAS connection
type Connector struct {
	VolumeName   string   `json:"volume_name"`
	TargetWWN    string   `json:"target_wwn"`
	Multipath    bool     `json:"multipath"`
	TargetDevice string   `json:"target_device"`
	SCSIDevices  []string `json:"scsi_devices"`
	IoHandler    ioHandler
}

// OSioHandler is a wrapper that includes all the necessary io functions used for (Should be used as default io handler)
type OSioHandler struct{}

// PUBLIC

// Attach attempts to attach a sas volume to a node using the provided Connector info
func Attach(ctx context.Context, c *Connector, io ioHandler) (string, error) {
	logger := klog.FromContext(ctx)

	if io == nil {
		io = &OSioHandler{}
	}

	logger.V(1).Info("Attaching SAS storage volume")
	err := discoverDevices(logger, c, io)

	if err != nil {
		logger.V(1).Info("unable to find device", "err", err)
		return "", err
	}

	return c.TargetDevice, nil
}

// Detach performs a detach operation on a volume
func Detach(ctx context.Context, devicePath string, io ioHandler) error {
	logger := klog.FromContext(ctx)

	if io == nil {
		io = &OSioHandler{}
	}

	logger.V(1).Info("Detaching SAS volume", "devicePath", devicePath)
	var devices []string
	dstPath, err := io.EvalSymlinks(devicePath)

	if err != nil {
		return err
	}

	if strings.HasPrefix(dstPath, "/dev/dm-") {
		devices = FindLinkedDevicesOnMultipath(logger, dstPath, io)
	} else {
		// Add single devicepath to devices
		devices = append(devices, dstPath)
	}

	logger.V(1).Info("sas: DetachDisk", "devicePath", devicePath, "dstPath", dstPath, "devices", devices)

	var lastErr error

	for _, device := range devices {
		err := detachDevice(logger, device, io)
		if err != nil {
			logger.Error(err, "sas: detach failed", "device", device)
			lastErr = fmt.Errorf("sas: detach failed. device: %v err: %v", device, err)
		}
	}

	if lastErr != nil {
		logger.Error(lastErr, "sas: last error occurred during detach disk")
		return lastErr
	}

	return nil
}

// Persist persists the Connector to the specified file (ie /var/lib/pfile/myConnector.json)
func (c *Connector) Persist(ctx context.Context, filePath string) error {
	logger := klog.FromContext(ctx)
	f, err := os.Create(filePath)
	if err != nil {
		logger.Error(err, "Could not create file", "filePath", filePath)
		return fmt.Errorf("error creating transport persistence file %s: %s", filePath, err)
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	if err = encoder.Encode(c); err != nil {
		logger.Error(err, "Could not encode the connector")
		return fmt.Errorf("error encoding connector: %v", err)
	}
	return nil
}

// GetConnectorFromFile attempts to create a Connector using the specified json file (ie /var/lib/pfile/myConnector.json)
func GetConnectorFromFile(filePath string) (*Connector, error) {
	f, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	c := Connector{}
	err = json.Unmarshal([]byte(f), &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// ResizeMultipathDevice: resize a multipath device based on its underlying devices
func ResizeMultipathDevice(ctx context.Context, devicePath string) error {
	logger := klog.FromContext(ctx)
	logger.V(1).Info("Resizing multipath device", "device", devicePath)

	if output, err := exec.Command("multipathd", "resize", "map", devicePath).CombinedOutput(); err != nil {
		return fmt.Errorf("could not resize multipath device: %s (%v)", output, err)
	}

	return nil
}

// PRIVATE

//ReadDir calls the ReadDir function from ioutil package
func (handler *OSioHandler) ReadDir(dirname string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(dirname)
}

//Lstat calls the Lstat function from os package
func (handler *OSioHandler) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(name)
}

//EvalSymlinks calls EvalSymlinks from filepath package
func (handler *OSioHandler) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

//WriteFile calls WriteFile from ioutil package
func (handler *OSioHandler) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}

// scsiHostRescan: Rescan all SCSI hosts
func scsiHostRescan(logger klog.Logger, io ioHandler) {
	scsiPath := "/sys/class/scsi_host/"
	logger.V(4).Info("scsi host rescan", "scsiPath", scsiPath)
	if dirs, err := io.ReadDir(scsiPath); err == nil {
		for _, f := range dirs {
			name := scsiPath + f.Name() + "/scan"
			data := []byte("- - -")
			logger.V(4).Info("io write file", "name", name, "data", data)
			io.WriteFile(name, data, 0666)
		}
	}
}

// discoverDevices: Attempt to discover a multipath device and all linked SCSI devices for a storage volume using WWN
func discoverDevices(logger klog.Logger, c *Connector, io ioHandler) error {
	var dm string
	var devices []string

	c.Multipath = false
	c.TargetDevice = ""
	rescaned := false

	// two-phase search:
	// first phase, search existing device paths, if a multipath dm is found, exit loop
	// otherwise, in second phase, rescan scsi bus and search again, return with any findings
	for true {

		// Find the multipath device using WWN
		dm, devices = findDiskById(logger, c.TargetWWN, io)
		logger.V(1).Info("find disk by id returned", "dm", dm, "devices", devices)

		for _, device := range devices {
			logger.V(3).Info("add scsi device", "device", device)
			if device != "" {
				c.SCSIDevices = append(c.SCSIDevices, device)
			}
		}

		// if multipath device is found, break
		if dm != "" && len(c.SCSIDevices) > 0 {
			c.Multipath = true
			c.TargetDevice = dm
			logger.V(1).Info("multipath device was discovered", "dm", dm, "SCSIDevices", c.SCSIDevices)
			break
		}

		// if a dm is found, exit loop
		if rescaned || dm != "" {
			break
		}

		// rescan scsi hosts and search again
		logger.V(2).Info("scsi rescan host")
		scsiHostRescan(logger, io)
		rescaned = true
	}

	// if no disk matches input wwn, return error
	c.TargetDevice = dm
	if dm == "" {
		err := fmt.Errorf("no SAS disk found")
		logger.Error(err, "no device discovered", "dm", dm)
		return err
	}

	return nil
}

// findDiskById: given a wwn of the storage volume, find the multipath device and associated scsi devices
func findDiskById(logger klog.Logger, wwn string, io ioHandler) (string, []string) {

	// Example multipath device naming:
	// Under /dev/disk/by-id:
	//   dm-name-3600c0ff0005460670a4ae16201000000 -> ../../dm-5
	//   dm-uuid-mpath-3600c0ff0005460670a4ae16201000000 -> ../../dm-5
	//   scsi-3600c0ff0005460670a4ae16201000000 -> ../../dm-5
	//   wwn-0x600c0ff0005460670a4ae16201000000 -> ../../dm-5

	var devices []string
	wwnPath := "wwn-0x" + wwn
	devPath := "/dev/disk/by-id/"
	logger.V(2).Info("find disk by id", "wwnPath", wwnPath, "devPath", devPath)

	if dirs, err := io.ReadDir(devPath); err == nil {
		for _, f := range dirs {
			name := f.Name()
			logger.V(4).Info("checking", "contains", strings.Contains(name, wwnPath), "name", name)
			if strings.Contains(name, wwnPath) {
				logger.V(2).Info("found device, evaluate symbolic links", "devPath", devPath, "name", name)
				if dm, err1 := io.EvalSymlinks(devPath + name); err1 == nil {
					devices = FindLinkedDevicesOnMultipath(logger, dm, io)
					return dm, devices
				}
			}
		}
	}
	return "", devices
}

// FindLinkedDevicesOnMultipath: returns all slaves on the multipath device given the device path
func FindLinkedDevicesOnMultipath(logger klog.Logger, dm string, io ioHandler) []string {

	// Example:
	// $ ls -l /sys/block/dm-5/slaves
	// sdb -> ../../../../pci0000:00/0000:00:03.0/0000:05:00.0/0000:06:09.0/0000:09:00.0/host8/port-8:0/end_device-8:0/target8:0:0/8:0:0:2/block/sdb
	// sdc -> ../../../../pci0000:00/0000:00:03.0/0000:05:00.0/0000:06:09.0/0000:09:00.0/host8/port-8:2/end_device-8:2/target8:0:2/8:0:2:2/block/sdc

	logger.V(2).Info("find linked devices on multipath", "dm", dm)

	var devices []string
	// Split path /dev/dm-1 into "", "dev", "dm-1"
	parts := strings.Split(dm, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "dev") {
		return devices
	}
	disk := parts[2]
	linkPath := path.Join("/sys/block/", disk, "/slaves/")
	logger.V(3).Info("linked scsi devices path", "linkPath", linkPath)
	if files, err := io.ReadDir(linkPath); err == nil {
		for _, f := range files {
			devices = append(devices, path.Join("/dev/", f.Name()))
		}
	}
	logger.V(2).Info("linked scsi devices found", "devices", devices)
	return devices
}

// detachDevice removes scsi device file such as /dev/sdX from the node.
func detachDevice(logger klog.Logger, devicePath string, io ioHandler) error {
	// Remove scsi device from the node.
	if !strings.HasPrefix(devicePath, "/dev/") {
		return fmt.Errorf("detach device: invalid device name: %s", devicePath)
	}
	arr := strings.Split(devicePath, "/")
	dev := arr[len(arr)-1]
	removeFromScsiSubsystem(logger, dev, io)
	return nil
}

// Removes a scsi device based upon /dev/sdX name
func removeFromScsiSubsystem(logger klog.Logger, deviceName string, io ioHandler) {
	fileName := "/sys/block/" + deviceName + "/device/delete"
	logger.V(2).Info("remove device from scsi-subsystem", "path", fileName)
	data := []byte("1")
	io.WriteFile(fileName, data, 0666)
}
