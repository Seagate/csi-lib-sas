# SAS Guidebook

An overview of using SAS and the csi-lib-sas example program for attaching to a SAS storage device. This guide uses Linux commands to display server file system and device information. This guide attempts to use SCSI terminology.

The **sas.go** `Attach()` function of this library uses the following information and device paths to discover SAS devices. The function also performs a SCSI bus rescan if a device cannot be discovered on the first pass. All of the information in this guidebook helps to explain the source code logic used in sas.go.

This guide provides system device details after creating a storage volume and mapping it to a server using a single transport path. That same volume is then mapped to 2nd SAS target port, resulting in the creation of a multipath device. The example program is used to display information using this SAS library.

Table of Contents
* [(1) Storage Volume Creation aka Logical Unit (Single Path)](#section1)
* [(2) Storage Volume Mapping (Dual Path)](#section2)
* [(3) Running the Example Program](#section3)
* [(4) Additional Information](#section4)

[//]: <> (================================================================================================================================================================)

## <a name="section1">Storage Volume Creation aka Logical Unit (Single Path)</a>

The first step is to create and map a storage volume from an external source so that is is visible on an attached computer system. The commands used to create and map a storage volume are vendor specific, so this section only shares the information needed from the server perspective. That information is needed so that the storage volume can be discovered in the host operating system.

For this example, here is the information pertaining to the storage volume created on an external storage array and mapped to a server.

| <div style="width:220px">Item</div> | <div style="width:180px">Value</div> | Notes |
| ------------------------- | -------------------------------- | ----------------------------------------------------------|
| <small>Volume</small>                        | <small>ph_00000000000000000000000000000</small> | <small>The name given to the storage volume created.</small> |
| <small>World Wide Name (WWN)</small>         | <small>600c0ff000546067a079e36201000000</small> | <small>The WWN of the storage volume created.</small> |
| <small>World Wide Port Name (WWPN) 1</small> | <small>500c0ff0afe43000</small>                 | <small>1st SAS port used to connect this volume to the server.</small> |
| <small>World Wide Port Name (WWPN) 2</small> | <small>500c0ff0afe43400</small>                 | <small>2nd SAS port used to connect this volume to the server.</small> |
| <small>Logical Unit Number (LUN)</small>     | <small>2</small>                                | <small>The LUN used to connect this volume to the server.</small> |


## SAS Device Manual Discovery

After the storage volume is created and mapped on the external storage device, a SCSI rescan is performed on the server to discover and map that storage to an operating system device. The following is a table of the new operating system device discovered and created after the SCSI bus rescan.

| Item             | Value   |
| ---------------- | ------- |
| <small>LUN 2 WWPN 2</small>     | <small>sdb</small>     |

### Rescan SCSI Bus

When running a rescan of the SCSI bus, the output indicates old and new devices discovered. In this example, only the scanning of host 8 is included to reduce the lines of output. Host 8 is the SAS host that has cables connected to the storage. Two lines of output are highlighted below followed by the entire host 8 scan results.

- **NEW: Host: scsi8 Channel: 00 Id: 00 Lun: 02**

```
$ sudo /usr/bin/rescan-scsi-bus.sh
Scanning SCSI subsystem for new devices
Scanning host 8 for  SCSI target IDs  0 1 2 3 4 5 6 7, all LUNs
 Scanning for device 8 0 0 0 ... 
OLD: Host: scsi8 Channel: 00 Id: 00 Lun: 00
      Vendor: SEAGATE  Model: 4006             Rev: I210
      Type:   Enclosure                        ANSI SCSI revision: 06
 Scanning for device 8 0 0 2 ... 
NEW: Host: scsi8 Channel: 00 Id: 00 Lun: 02
      Vendor: SEAGATE  Model: 4006             Rev: I210
      Type:   Direct-Access                    ANSI SCSI revision: 06
 Scanning for device 8 0 2 0 ... 
OLD: Host: scsi8 Channel: 00 Id: 02 Lun: 00
      Vendor: SEAGATE  Model: 4006             Rev: I210
      Type:   Enclosure                        ANSI SCSI revision: 06
1 new or changed device(s) found.          
	[8:0:0:2]
0 remapped or resized device(s) found.
0 device(s) removed.                 
```

### Device Discovery by-id

The next step is to review a list of devices created using the `ls -lR /dev/disk` command. This produces a recursive list of devices for each subfolder. In this example, many lines were removed if they did not relate to the new device created, so this is example text that was reduced in line count.

One explanation of the output listed below. There are several different ways `sdb` is listed as a device name.

| <div style="width:200px">Listing</div>             | Notes   |
| ---------------- | ------- |
| <small>wwn-0x**600c0ff000546067a079e36201000000** -> ../../sdb</small>        | <small>The WWN of the storage volume mapped to sdb</small> |

```
$ ls -lR /dev/disk

/dev/disk/by-id:
lrwxrwxrwx. 1 root root  9 Jul 29 10:02 scsi-0SEAGATE_4006_113061666534330000c0ff54606500000aebc87600c0ff54606700000aebc875 -> ../../sdb
lrwxrwxrwx. 1 root root  9 Jul 29 10:02 scsi-3600c0ff000546067a079e36201000000 -> ../../sdb
lrwxrwxrwx. 1 root root  9 Jul 29 10:02 scsi-SSEAGATE_4006_00c0ff5460670000a079e36201000000 -> ../../sdb
lrwxrwxrwx. 1 root root  9 Jul 29 10:02 wwn-0x600c0ff000546067a079e36201000000 -> ../../sdb

/dev/disk/by-path:
lrwxrwxrwx. 1 root root  9 Jul 29 10:02 pci-0000:09:00.0-sas-0x500c0ff0afe43400-lun-2 -> ../../sdb
```

### Device Discovery Using /dev and /sys/block

The same device is also listed under the `/dev/` folder and the `/sys/block` folder. We can also see that `sdb` has no associated devices, indicating it is not a multipath device.

```
$ ls -l /dev/sd*
brw-rw----. 1 root disk 8, 16 Jul 29 10:02 /dev/sdb

$ ls -l /sys/block/sd*
lrwxrwxrwx. 1 root root 0 Jul 29 10:02 /sys/block/sdb -> ../devices/pci0000:00/0000:00:03.0/0000:05:00.0/0000:06:09.0/0000:09:00.0/host8/port-8:0/end_device-8:0/target8:0:0/8:0:0:2/block/sdb

$ ls -l /sys/block/sdb/slaves
total 0

```

### Discovery Using lsscsi

The tool `lsscsi` produces useful information mapping operating system devices to the storage volume. For this guidebook, only host 8 lines are listed. A user can see how the device **sdb** is linked to the sas disk port **0x500c0ff0afe43400** and lun **2**. The tool also produces additional details if `lsscsi -t --list` is used.

```
$ lsscsi -t
[8:0:0:0]    enclosu sas:0x500c0ff0afe43400          -        
[8:0:0:2]    disk    sas:0x500c0ff0afe43400          /dev/sdb 
[8:0:2:0]    enclosu sas:0x500c0ff0afe43000          -        
```

[//]: <> (================================================================================================================================================================)

## <a name="section2">Storage Volume Mapping (Dual Path)</a>

At this point, we have created a storage volume and mapped it to a single SAS port. The next step is to map the same storage volume to a second SAS port, thus creating a dual path from the server to the same storage volume. With the multipath daemon installed and running, this step displays the system information after the second map and scsi bus rescan.

## SAS Device Manual Discovery

After the storage volume is mapped again on the external storage device, a SCSI rescan is performed on the server to discover and map that storage to operating system devices. This example is running the mulitpath daemon which creates an additional multipath device. Here is the list of devices created. The following is a table of new operating system devices discovered and created after the second SCSI bus rescan. The device `sdb` is listed a second time for reference although it already exists.

| Item             | Value   |
| ---------------- | ------- |
| <small>LUN 2 WWPN 1</small>     | <small>sdc</small>     |
| <small>LUN 2 WWPN 2</small>     | <small>sdb</small>     |
| <small>Multipath Device</small> | <small>dm-5</small>    |

### Rescan SCSI Bus

When running a rescan of the SCSI bus, the output indicates old and new devices discovered. In this example, only the scanning of host 8 is included to reduce the lines of output. Host 8 is the SAS host that has cables connected to the storage. Two lines of output are highlighted below followed by the entire host 8 scan results.

- **NEW: Host: scsi8 Channel: 00 Id: 02 Lun: 02**

```
$ sudo /usr/bin/rescan-scsi-bus.sh
Scanning SCSI subsystem for new devices
Scanning host 8 for  SCSI target IDs  0 1 2 3 4 5 6 7, all LUNs
 Scanning for device 8 0 0 0 ... 
OLD: Host: scsi8 Channel: 00 Id: 00 Lun: 00
      Vendor: SEAGATE  Model: 4006             Rev: I210
      Type:   Enclosure                        ANSI SCSI revision: 06
 Scanning for device 8 0 0 2 ... 
OLD: Host: scsi8 Channel: 00 Id: 00 Lun: 02
      Vendor: SEAGATE  Model: 4006             Rev: I210
      Type:   Direct-Access                    ANSI SCSI revision: 06
 Scanning for device 8 0 2 0 ... 
OLD: Host: scsi8 Channel: 00 Id: 02 Lun: 00
      Vendor: SEAGATE  Model: 4006             Rev: I210
      Type:   Enclosure                        ANSI SCSI revision: 06
 Scanning for device 8 0 2 2 ... 
NEW: Host: scsi8 Channel: 00 Id: 02 Lun: 02
      Vendor: SEAGATE  Model: 4006             Rev: I210
      Type:   Direct-Access                    ANSI SCSI revision: 06
1 new or changed device(s) found.          
	[8:0:2:2]
0 remapped or resized device(s) found.
0 device(s) removed.                 
```

### Device Discovery by-id

The next step is to review a list of devices created using the `ls -lR /dev/disk` command. This produces a recursive list of devices for each subfolder. In this example, many lines were removed if they were not related to the new devices created, so this is example text that was reduced in line count.

Some explanations of the output listed below. There are several different ways `dm-5` is listed as a `/dev/disk/by-id` device name.

| <div style="width:200px">Listing</div>             | Notes   |
| ---------------- | ------- |
| <small>wwn-0x**600c0ff000546067a079e36201000000** -> ../../dm-5</small>        | <small>The WWN of the storage volume mapped to dm-5</small> |

```
$ ls -lR /dev/disk

/dev/disk/by-id:
lrwxrwxrwx. 1 root root 10 Jul 29 10:26 dm-name-3600c0ff000546067a079e36201000000 -> ../../dm-5
lrwxrwxrwx. 1 root root 10 Jul 29 10:26 dm-uuid-mpath-3600c0ff000546067a079e36201000000 -> ../../dm-5
lrwxrwxrwx. 1 root root 10 Jul 29 10:26 scsi-3600c0ff000546067a079e36201000000 -> ../../dm-5
lrwxrwxrwx. 1 root root 10 Jul 29 10:26 wwn-0x600c0ff000546067a079e36201000000 -> ../../dm-5

/dev/disk/by-path:
lrwxrwxrwx. 1 root root  9 Jul 29 10:26 pci-0000:09:00.0-sas-0x500c0ff0afe43000-lun-2 -> ../../sdc
lrwxrwxrwx. 1 root root  9 Jul 29 10:26 pci-0000:09:00.0-sas-0x500c0ff0afe43400-lun-2 -> ../../sdb
```

### Device Discovery by-path

The `ls -lR /dev/disk` command can be used to discover the SCSI devices created after the storage volume is mapped to the server. These devices are referenced by their World Wide Port Name (WWPN) and Logical Unit Number (LUN).

| <div style="width:200px">Listing</div>             | Notes   |
| ---------------- | ------- |
| <small>pci-0000:09:00.0-sas-0x**500c0ff0afe43000**-lun-**2** -> ../../sdc</small> | <small>WWPN 1, and lun, mapped to sdc</small> |
| <small>pci-0000:09:00.0-sas-0x**500c0ff0afe43400**-lun-**2** -> ../../sdb</small> | <small>WWPN 2, and lun, mapped to sdb</small> |


### Multipath Discovery Using /sys/block

Once we know the multipath device `dm-5` we can use the `/sys/block` information to discover the linkage between `dm-5` and `sdc` and `sdb`. For each multipath device, the following path can be explored to discover the linked devices using `/sys/block/<dm-#>/slaves`.

```
$ ls -l /sys/block
lrwxrwxrwx. 1 root root 0 Jul 29 10:26 dm-5 -> ../devices/virtual/block/dm-5
lrwxrwxrwx. 1 root root 0 Jul 29 10:02 sdb -> ../devices/pci0000:00/0000:00:03.0/0000:05:00.0/0000:06:09.0/0000:09:00.0/host8/port-8:0/end_device-8:0/target8:0:0/8:0:0:2/block/sdb
lrwxrwxrwx. 1 root root 0 Jul 29 10:26 sdc -> ../devices/pci0000:00/0000:00:03.0/0000:05:00.0/0000:06:09.0/0000:09:00.0/host8/port-8:2/end_device-8:2/target8:0:2/8:0:2:2/block/sdc

$ ls -l /sys/block/dm-5/slaves
lrwxrwxrwx. 1 root root 0 Jul 29 10:26 sdb -> ../../../../pci0000:00/0000:00:03.0/0000:05:00.0/0000:06:09.0/0000:09:00.0/host8/port-8:0/end_device-8:0/target8:0:0/8:0:0:2/block/sdb
lrwxrwxrwx. 1 root root 0 Jul 29 10:26 sdc -> ../../../../pci0000:00/0000:00:03.0/0000:05:00.0/0000:06:09.0/0000:09:00.0/host8/port-8:2/end_device-8:2/target8:0:2/8:0:2:2/block/sdc
```

### Multipath Discovery Using /dev/mapper

Once we know the WWN of storage volume, we can prepend `3` to it and find the multipath device using the `/dev/mapper` information. In this case, we see `/dev/mapper/3<wwn>/` is linked to `dm-5`, our multipath device for this storage volume.

```
$ ls -l /dev/mapper
lrwxrwxrwx. 1 root root       7 Jul 29 10:26 3600c0ff000546067a079e36201000000 -> ../dm-5
```

### Discovery Using lsscsi

The tool `lsscsi` produces useful information mapping operating system devices to the storage volume. For this guidebook, only host 8 lines are listed. A user can see how the device **sdb** is linked to the sas disk port **0x500c0ff0afe43400** and lun **2**. And see how the device **sdc** is linked to the sas disk port **0x500c0ff0afe43000** and lun **2**. Use `lsscsi -t --list` to list additional details.

```
$ lsscsi -t
[8:0:0:0]    enclosu sas:0x500c0ff0afe43400          -        
[8:0:0:2]    disk    sas:0x500c0ff0afe43400          /dev/sdb 
[8:0:2:0]    enclosu sas:0x500c0ff0afe43000          -        
[8:0:2:2]    disk    sas:0x500c0ff0afe43000          /dev/sdc 
```


### Multipath Device Summary

For multipath devices, additional information is obtained using the `multipath` tool. This output produces information relating the WWN of the storage volume (**600c0ff000546067a079e36201000000**) to the multipath device (**dm-5**) and also lists each linked device (**sdc** and **sdb**) used to produce **dm-5**.

```
$ sudo multipath -ll
3600c0ff000546067a079e36201000000 dm-5 SEAGATE,4006
size=1.0G features='1 queue_if_no_path' hwhandler='1 alua' wp=rw
|-+- policy='service-time 0' prio=50 status=active
| `- 8:0:2:2 sdc 8:32 active ready running
`-+- policy='service-time 0' prio=10 status=enabled
  `- 8:0:0:2 sdb 8:16 active ready running
```


[//]: <> (================================================================================================================================================================)

## <a name="section3">Running the Example Program</a>

The example program allows you to attach and detach to a storage volume using the sas.go library functions `Attach()` and `Detach()`. 

### Example Help

```
$ ./example -h
====================================================================================================
example - Run the SAS Library Example Program

Examples:
./example -wwn 600c0ff000546067369fe36201000000
./example -wwn 600c0ff000546067369fe36201000000 -v=4
./example -wwn 600c0ff000546067369fe36201000000 -v=4 -detach

Low level library commands require sudo or root privilege. You must provide a wwn to be succeed.

Options:
  -add_dir_header
    	If true, adds the file directory to the header of the log messages
  -alsologtostderr
    	log to standard error as well as files (no effect when -logtostderr=true)
  -detach
    	automatically detach after a successful attach
  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory (no effect when -logtostderr=true)
  -log_file string
    	If non-empty, use this log file (no effect when -logtostderr=true)
  -log_file_max_size uint
    	Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
  -logtostderr
    	log to standard error instead of files (default true)
  -one_output
    	If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
  -skip_headers
    	If true, avoid header prefixes in the log messages
  -skip_log_headers
    	If true, avoid headers when opening log files (no effect when -logtostderr=true)
  -stderrthreshold value
    	logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=false) (default 2)
  -v value
    	number for the log level verbosity
  -vmodule value
    	comma-separated list of pattern=N settings for file-filtered logging
  -wwn string
    	Specify a WWN
```

### Running Example with a Single Mapped SAS Port

```
$ ./example -wwn 600c0ff000546067a079e36201000000
I0729 15:14:42.580104  226903 main.go:60] "[] sas test example" wwn="600c0ff000546067a079e36201000000" detach=false
I0729 15:14:42.580908  226903 main.go:72] "SAS Attach success" device="/dev/sdb" connector={VolumeName: TargetWWN:600c0ff000546067a079e36201000000 Multipath:false TargetDevice:/dev/sdb SCSIDevices:[] IoHandler:<nil>}
```

```
$ ./example -wwn 600c0ff000546067a079e36201000000 -v=3
I0729 15:15:13.931817  226915 main.go:60] "[] sas test example" wwn="600c0ff000546067a079e36201000000" detach=false
I0729 15:15:13.932071  226915 sas.go:63] "Attaching SAS storage volume"
I0729 15:15:13.932105  226915 sas.go:265] "find disk by id" wwnPath="wwn-0x600c0ff000546067a079e36201000000" devPath="/dev/disk/by-id/"
I0729 15:15:13.932522  226915 sas.go:272] "found device, evaluate symbolic links" devPath="/dev/disk/by-id/" name="wwn-0x600c0ff000546067a079e36201000000"
I0729 15:15:13.932730  226915 sas.go:291] "find linked devices on multipath" dm="/dev/sdb"
I0729 15:15:13.932769  226915 sas.go:301] "linked scsi devices path" linkPath="/sys/block/sdb/slaves"
I0729 15:15:13.932886  226915 sas.go:307] "linked scsi devices found" devices=[]
I0729 15:15:13.932916  226915 sas.go:213] "find disk by id returned" dm="/dev/sdb" devices=[]
I0729 15:15:13.932977  226915 main.go:72] "SAS Attach success" device="/dev/sdb" connector={VolumeName: TargetWWN:600c0ff000546067a079e36201000000 Multipath:false TargetDevice:/dev/sdb SCSIDevices:[] IoHandler:<nil>}
```

### Running Example with Dual Mapped SAS Ports

```
$ ./example -wwn 600c0ff000546067a079e36201000000
I0729 15:17:35.043454  227729 main.go:60] "[] sas test example" wwn="600c0ff000546067a079e36201000000" detach=false
I0729 15:17:35.044334  227729 main.go:72] "SAS Attach success" device="/dev/dm-5" connector={VolumeName: TargetWWN:600c0ff000546067a079e36201000000 Multipath:true TargetDevice:/dev/dm-5 SCSIDevices:[/dev/sdb /dev/sdc] IoHandler:<nil>}
```

```
$ ./example -wwn 600c0ff000546067a079e36201000000 -v=3
I0729 15:17:42.491035  227743 main.go:60] "[] sas test example" wwn="600c0ff000546067a079e36201000000" detach=false
I0729 15:17:42.491287  227743 sas.go:63] "Attaching SAS storage volume"
I0729 15:17:42.491321  227743 sas.go:265] "find disk by id" wwnPath="wwn-0x600c0ff000546067a079e36201000000" devPath="/dev/disk/by-id/"
I0729 15:17:42.491736  227743 sas.go:272] "found device, evaluate symbolic links" devPath="/dev/disk/by-id/" name="wwn-0x600c0ff000546067a079e36201000000"
I0729 15:17:42.491815  227743 sas.go:291] "find linked devices on multipath" dm="/dev/dm-5"
I0729 15:17:42.491849  227743 sas.go:301] "linked scsi devices path" linkPath="/sys/block/dm-5/slaves"
I0729 15:17:42.491993  227743 sas.go:307] "linked scsi devices found" devices=[/dev/sdb /dev/sdc]
I0729 15:17:42.492029  227743 sas.go:213] "find disk by id returned" dm="/dev/dm-5" devices=[/dev/sdb /dev/sdc]
I0729 15:17:42.492054  227743 sas.go:216] "add scsi device" device="/dev/sdb"
I0729 15:17:42.492077  227743 sas.go:216] "add scsi device" device="/dev/sdc"
I0729 15:17:42.492103  227743 sas.go:226] "multipath device was discovered" dm="/dev/dm-5" SCSIDevices=[/dev/sdb /dev/sdc]
I0729 15:17:42.492147  227743 main.go:72] "SAS Attach success" device="/dev/dm-5" connector={VolumeName: TargetWWN:600c0ff000546067a079e36201000000 Multipath:true TargetDevice:/dev/dm-5 SCSIDevices:[/dev/sdb /dev/sdc] IoHandler:<nil>}
```

```
$ sudo ./example -wwn 600c0ff000546067a079e36201000000 -v=3 -detach
I0729 15:17:54.470808  227761 main.go:60] "[] sas test example" wwn="600c0ff000546067a079e36201000000" detach=true
I0729 15:17:54.471010  227761 sas.go:63] "Attaching SAS storage volume"
I0729 15:17:54.471039  227761 sas.go:265] "find disk by id" wwnPath="wwn-0x600c0ff000546067a079e36201000000" devPath="/dev/disk/by-id/"
I0729 15:17:54.471427  227761 sas.go:272] "found device, evaluate symbolic links" devPath="/dev/disk/by-id/" name="wwn-0x600c0ff000546067a079e36201000000"
I0729 15:17:54.471504  227761 sas.go:291] "find linked devices on multipath" dm="/dev/dm-5"
I0729 15:17:54.471537  227761 sas.go:301] "linked scsi devices path" linkPath="/sys/block/dm-5/slaves"
I0729 15:17:54.471702  227761 sas.go:307] "linked scsi devices found" devices=[/dev/sdb /dev/sdc]
I0729 15:17:54.471739  227761 sas.go:213] "find disk by id returned" dm="/dev/dm-5" devices=[/dev/sdb /dev/sdc]
I0729 15:17:54.471766  227761 sas.go:216] "add scsi device" device="/dev/sdb"
I0729 15:17:54.471787  227761 sas.go:216] "add scsi device" device="/dev/sdc"
I0729 15:17:54.471814  227761 sas.go:226] "multipath device was discovered" dm="/dev/dm-5" SCSIDevices=[/dev/sdb /dev/sdc]
I0729 15:17:54.471856  227761 main.go:72] "SAS Attach success" device="/dev/dm-5" connector={VolumeName: TargetWWN:600c0ff000546067a079e36201000000 Multipath:true TargetDevice:/dev/dm-5 SCSIDevices:[/dev/sdb /dev/sdc] IoHandler:<nil>}
I0729 15:17:54.471886  227761 sas.go:82] "Detaching SAS volume" devicePath="/dev/dm-5"
I0729 15:17:54.471923  227761 sas.go:291] "find linked devices on multipath" dm="/dev/dm-5"
I0729 15:17:54.471951  227761 sas.go:301] "linked scsi devices path" linkPath="/sys/block/dm-5/slaves"
I0729 15:17:54.472050  227761 sas.go:307] "linked scsi devices found" devices=[/dev/sdb /dev/sdc]
I0729 15:17:54.472080  227761 sas.go:97] "sas: DetachDisk" devicePath="/dev/dm-5" dstPath="/dev/dm-5" devices=[/dev/sdb /dev/sdc]
I0729 15:17:54.472106  227761 sas.go:326] "remove device from scsi-subsystem" path="/sys/block/sdb/device/delete"
I0729 15:17:54.491171  227761 sas.go:326] "remove device from scsi-subsystem" path="/sys/block/sdc/device/delete"
E0729 15:17:54.512141  227761 main.go:76] "SAS Detach success"
```

[//]: <> (================================================================================================================================================================)

## <a name="section4">Additional Information</a>

Here are several development troubleshooting tips that may prove useful, but each should be use with extreme caution if used on servers that have live user data is use. Some or all of these commands may require `sudo` or root privileges in order to run.

### Rescan a Single SCSI Host

You can use the `echo` command to rescan a single SCSI host. In fact, this library performs a SCSI bus rescan if a device cannot be discovered on a first pass. The placeholder `<hostX>` must be replaced with the name of the host to scan. Use `ls -l /sys/class/scsi_host` to determine a list of all active SCSI hosts.

```
echo "- - -" > /sys/class/scsi_host/<hostX>/scan
```

### Removing Stale SCSI Devices

After removing a storage volume, unmapping it, and performing a SCSI bus rescan; you may find that the operating system still lists the device under one or more /dev/disk subfolders. You can use the following command to remove that stale device. The placeholder `<sdX>` must be replaced with the name of your device. For example, the devices created in this guide where `sdb` and `sdc`.

**Note:** Be careful, don’t remove devices you are using!

With root privilege, call echo directly, otherwise use a sudo shell.

```
echo 1 > /sys/block/<sdX>/device/delete
```

```
sudo sh -c 'echo 1 > /sys/block/<sdX>/device/delete'
```

### Discover SAS Host Address

To see host specific SAS details, use `cat` of the host SAS address file.

```
$ ls -l /sys/class/sas_host
lrwxrwxrwx. 1 root root 0 Jul 25 15:36 host8 -> ../../devices/pci0000:00/0000:00:03.0/0000:05:00.0/0000:06:09.0/0000:09:00.0/host8/sas_host/host8

$ cat /sys/class/scsi_host/host8/host_sas_address
0x500605b0091fbb20
```
