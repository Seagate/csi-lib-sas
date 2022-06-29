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
	"strings"

	"github.com/Seagate/csi-lib-sas/sas"
	"k8s.io/klog/v2"
)

func main() {
	// Enable contextual logging
	ctx := context.Background()
	klog.InitFlags(nil)
	klog.EnableContextualLogging(true)
	logger := klog.FromContext(ctx)

	wwns := flag.String("wwns", "", "Specify a comma separated list of WWNs")
	lun := flag.String("lun", "1", "Specify a LUN, defaults to 1")
	flag.Parse()

	lcwwns := strings.ToLower(*wwns)
	logger.Info("[] sas test example", "lun", *lun, "wwns", lcwwns)

	c := sas.Connector{}

	// Use command line arguments for test settings
	c.TargetWWNs = strings.Split(lcwwns, ",")
	c.Lun = *lun

	dp, err := sas.Attach(ctx, c, &sas.OSioHandler{})

	if err != nil {
		logger.Error(err, "SAS Attach failure")
	} else {
		logger.Info("SAS Attach success", "devicePath", dp)
		sas.Detach(ctx, dp, &sas.OSioHandler{})
	}
}
