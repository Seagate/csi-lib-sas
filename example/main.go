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
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/Seagate/csi-lib-sas/sas"
	"k8s.io/klog/v2"
)

// Define usage and some common examples
const (
	usage = `====================================================================================================
example - Run the SAS Library Example Program

Examples:
./example -wwn 600c0ff000546067369fe36201000000
./example -wwn 600c0ff000546067369fe36201000000 -v=4
./example -wwn 600c0ff000546067369fe36201000000 -v=4 -detach

Low level library commands require sudo or root privilege. You must provide a wwn to be succeed.

Options:
`
)

func main() {
	// Enable contextual logging
	ctx := context.Background()
	klog.InitFlags(nil)
	klog.EnableContextualLogging(true)
	logger := klog.FromContext(ctx)

	wwn := flag.String("wwn", "", "Specify a WWN")
	detach := flag.Bool("detach", false, "automatically detach after a successful attach")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), usage)
		flag.PrintDefaults()
	}

	flag.Parse()

	logger.Info("[] sas test example", "wwn", *wwn, "detach", *detach)

	// Use command line arguments for test settings
	c := sas.Connector{
		TargetWWN: *wwn,
	}

	dp, err := sas.Attach(ctx, &c, &sas.OSioHandler{})

	if err != nil {
		logger.Error(err, "SAS Attach failure")
	} else {
		logger.Info("SAS Attach success", "device", dp, "connector", c)
		if *detach {
			err = sas.Detach(ctx, dp, &sas.OSioHandler{})
			if err == nil {
				logger.Error(err, "SAS Detach success")
			} else {
				logger.Error(err, "SAS Detach failure")
			}
		}
	}
}
