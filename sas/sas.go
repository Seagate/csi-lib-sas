/*
Copyright 2018 The Kubernetes Authors.

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

	"errors"
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

//Connector provides a struct to hold all of the needed parameters to make our SAS connection
type Connector struct {
	VolumeName string   `json:"volume_name"`
	TargetWWNs []string `json:"target_wwns"`
	Lun        string   `json:"lun"`
	WWIDs      []string `json:"wwids"`
	Multipath  bool     `json:"multipath"`
	DevicePath string   `json:"device_path"`
	IoHandler  ioHandler
}

//OSioHandler is a wrapper that includes all the necessary io functions used for (Should be used as default io handler)
type OSioHandler struct{}

// PUBLIC

// Attach attempts to attach a sas volume to a node using the provided Connector info
func Attach(ctx context.Context, c Connector, io ioHandler) (string, error) {
	logger := klog.FromContext(ctx)

	if io == nil {
		io = &OSioHandler{}
	}

	logger.V(1).Info("Attaching SAS volume")
	devicePath, err := searchDisk(logger, c, io)

	if err != nil {
		logger.V(1).Info("unable to find disk given WWNN or WWIDs")
		return "", err
	}

	c.DevicePath = devicePath

	return devicePath, nil
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
		devices = FindSlaveDevicesOnMultipath(dstPath, io)
	} else {
		// Add single devicepath to devices
		devices = append(devices, dstPath)
	}

	logger.V(1).Info("sas: DetachDisk", "devicePath", devicePath, "dstPath", dstPath, "devices", devices)

	var lastErr error

	for _, device := range devices {
		err := detachSasDisk(logger, device, io)
		if err != nil {
			logger.Error(err, "sas: detachSasDisk failed", "device", device)
			lastErr = fmt.Errorf("sas: detachSasDisk failed. device: %v err: %v", device, err)
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
		return fmt.Errorf("error creating iSCSI persistence file %s: %s", filePath, err)
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

// FindMultipathDeviceForDevice given a device name like /dev/sdx, find the devicemapper parent
func FindMultipathDeviceForDevice(device string, io ioHandler) (string, error) {
	disk, err := findDeviceForPath(device, io)
	if err != nil {
		return "", err
	}
	sysPath := "/sys/block/"
	if dirs, err2 := io.ReadDir(sysPath); err2 == nil {
		for _, f := range dirs {
			name := f.Name()
			if strings.HasPrefix(name, "dm-") {
				if _, err1 := io.Lstat(sysPath + name + "/slaves/" + disk); err1 == nil {
					return "/dev/" + name, nil
				}
			}
		}
	} else {
		return "", err2
	}

	return "", nil
}

// findDeviceForPath Find the underlaying disk for a linked path such as /dev/disk/by-path/XXXX or /dev/mapper/XXXX
// will return sdX or hdX etc, if /dev/sdX is passed in then sdX will be returned
func findDeviceForPath(path string, io ioHandler) (string, error) {
	devicePath, err := io.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	// if path /dev/hdX split into "", "dev", "hdX" then we will
	// return just the last part
	parts := strings.Split(devicePath, "/")
	if len(parts) == 3 && strings.HasPrefix(parts[1], "dev") {
		return parts[2], nil
	}
	return "", errors.New("Illegal path for device " + devicePath)
}

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

func searchDisk(logger klog.Logger, c Connector, io ioHandler) (string, error) {
	var diskIds []string
	var disk string
	var dm string

	if len(c.TargetWWNs) != 0 {
		diskIds = c.TargetWWNs
	} else {
		diskIds = c.WWIDs
	}

	c.Multipath = false

	rescaned := false

	// two-phase search:
	// first phase, search existing device path, if a multipath dm is found, exit loop
	// otherwise, in second phase, rescan scsi bus and search again, return with any findings
	for true {

		for _, diskID := range diskIds {
			logger.V(2).Info("search for disk", "diskID", diskID)

			if len(c.TargetWWNs) != 0 {
				disk, dm = findDisk(logger, diskID, c.Lun, io)
			} else {
				disk, dm = findDiskWWIDs(logger, diskID, io)
			}
			// if multipath device is found, break
			if dm != "" {
				break
			}
		}
		// if a dm is found, exit loop
		if rescaned || dm != "" {
			break
		}
		// rescan and search again
		// rescan scsi bus
		logger.V(2).Info("scsi rescan host")
		scsiHostRescan(logger, io)
		rescaned = true
	}
	// if no disk matches input wwn and lun, exit
	if disk == "" && dm == "" {
		return "", fmt.Errorf("no SAS disk found")
	}

	// if multipath devicemapper device is found, use it; otherwise use raw disk
	if dm != "" {
		c.Multipath = true
		logger.V(1).Info("multipath device was discovered", "dm", dm)
		return dm, nil
	}

	return disk, nil
}

// given a wwn and lun, find the device and associated devicemapper parent
func findDisk(logger klog.Logger, wwn, lun string, io ioHandler) (string, string) {
	logger.V(4).Info("find disk", "wwn", wwn, "lun", lun)
	wwnPath := "wwn-0x" + wwn
	DevPath := "/dev/disk/by-id/"
	if dirs, err := io.ReadDir(DevPath); err == nil {
		for _, f := range dirs {
			name := f.Name()
			logger.V(4).Info("checking", "contains", strings.Contains(name, wwnPath), "wwnPath", wwnPath, "name", name)
			if strings.Contains(name, wwnPath) {
				logger.V(4).Info("evaluate symbolic links", "DevPath+name", DevPath+name)
				if disk, err1 := io.EvalSymlinks(DevPath + name); err1 == nil {
					logger.V(4).Info("find multipath device", "disk", disk+name)
					if dm, err2 := FindMultipathDeviceForDevice(disk, io); err2 == nil {
						logger.V(1).Info("found disk", "disk", disk, "dm", dm)
						return disk, dm
					}
				}
			}
		}
	}
	return "", ""
}

// given a wwid, find the device and associated devicemapper parent
func findDiskWWIDs(logger klog.Logger, wwid string, io ioHandler) (string, string) {
	// Example wwid format:
	//   3600508b400105e210000900000490000
	//   <VENDOR NAME> <IDENTIFIER NUMBER>
	// Example of symlink under by-id:
	//   /dev/by-id/scsi-3600508b400105e210000900000490000
	//   /dev/by-id/scsi-<VENDOR NAME>_<IDENTIFIER NUMBER>
	// The wwid could contain white space and it will be replaced
	// underscore when wwid is exposed under /dev/by-id.

	logger.V(4).Info("find disk wwids", "wwid", wwid)
	sasPath := "scsi-" + wwid
	DevID := "/dev/disk/by-id/"
	if dirs, err := io.ReadDir(DevID); err == nil {
		for _, f := range dirs {
			name := f.Name()
			logger.V(4).Info("find disk wwids, searching...", "name", name, "sasPath", sasPath)
			if name == sasPath {
				disk, err := io.EvalSymlinks(DevID + name)
				if err != nil {
					logger.Error(err, "sas: failed to find a corresponding disk from symlink", "DevID", DevID, "name", name, "DevID+name", DevID+name)
					return "", ""
				}
				if dm, err1 := FindMultipathDeviceForDevice(disk, io); err1 != nil {
					logger.V(4).Info("find disk wwids, found", "disk", disk, "dm", dm)
					return disk, dm
				}
			}
		}
	}
	logger.Info("sas: failed to find a disk", "DevID", DevID, "sasPath", sasPath)
	return "", ""
}

//FindSlaveDevicesOnMultipath returns all slaves on the multipath device given the device path
func FindSlaveDevicesOnMultipath(dm string, io ioHandler) []string {
	var devices []string
	// Split path /dev/dm-1 into "", "dev", "dm-1"
	parts := strings.Split(dm, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "dev") {
		return devices
	}
	disk := parts[2]
	slavesPath := path.Join("/sys/block/", disk, "/slaves/")
	if files, err := io.ReadDir(slavesPath); err == nil {
		for _, f := range files {
			devices = append(devices, path.Join("/dev/", f.Name()))
		}
	}
	return devices
}

// detachSasDisk removes scsi device file such as /dev/sdX from the node.
func detachSasDisk(logger klog.Logger, devicePath string, io ioHandler) error {
	// Remove scsi device from the node.
	if !strings.HasPrefix(devicePath, "/dev/") {
		return fmt.Errorf("sas detach disk: invalid device name: %s", devicePath)
	}
	arr := strings.Split(devicePath, "/")
	dev := arr[len(arr)-1]
	removeFromScsiSubsystem(logger, dev, io)
	return nil
}

// Removes a scsi device based upon /dev/sdX name
func removeFromScsiSubsystem(logger klog.Logger, deviceName string, io ioHandler) {
	fileName := "/sys/block/" + deviceName + "/device/delete"
	logger.Info("sas: remove device from scsi-subsystem", "path", fileName)
	data := []byte("1")
	io.WriteFile(fileName, data, 0666)
}
