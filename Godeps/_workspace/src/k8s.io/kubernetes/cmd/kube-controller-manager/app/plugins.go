/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package app

import (
// This file exists to force the desired plugin implementations to be linked.
// This should probably be part of some configuration fed into the build for a
// given binary target.

//Cloud providers
	"k8s.io/kubernetes/pkg/cloudprovider"
	_ "k8s.io/kubernetes/pkg/cloudprovider/providers"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/aws"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/openstack"

// Volume plugins
	"k8s.io/kubernetes/pkg/util/io"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/aws_ebs"
	"k8s.io/kubernetes/pkg/volume/cinder"
	"k8s.io/kubernetes/pkg/volume/gce_pd"
	"k8s.io/kubernetes/pkg/volume/host_path"
	"k8s.io/kubernetes/pkg/volume/nfs"

	"github.com/golang/glog"
)

// ProbeRecyclableVolumePlugins collects all persistent volume plugins into an easy to use list.
func ProbeRecyclableVolumePlugins(flags VolumeConfigFlags) []volume.VolumePlugin {
	allPlugins := []volume.VolumePlugin{}

	// The list of plugins to probe is decided by this binary, not
	// by dynamic linking or other "magic".  Plugins will be analyzed and
	// initialized later.

	// Each plugin can make use of VolumeConfig.  The single arg to this func contains *all* enumerated
	// CLI flags meant to configure volume plugins.  From that single config, create an instance of volume.VolumeConfig
	// for a specific plugin and pass that instance to the plugin's ProbeVolumePlugins(config) func.

	// HostPath recycling is for testing and development purposes only!
	hostPathConfig := volume.VolumeConfig{
		RecyclerMinimumTimeout:   flags.PersistentVolumeRecyclerMinimumTimeoutHostPath,
		RecyclerTimeoutIncrement: flags.PersistentVolumeRecyclerIncrementTimeoutHostPath,
		RecyclerPodTemplate:      volume.NewPersistentVolumeRecyclerPodTemplate(),
	}
	if err := attemptToLoadRecycler(flags.PersistentVolumeRecyclerPodTemplateFilePathHostPath, &hostPathConfig); err != nil {
		glog.Fatalf("Could not create hostpath recycler pod from file %s: %+v", err)
	}
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins(hostPathConfig)...)

	nfsConfig := volume.VolumeConfig{
		RecyclerMinimumTimeout:   flags.PersistentVolumeRecyclerMinimumTimeoutNFS,
		RecyclerTimeoutIncrement: flags.PersistentVolumeRecyclerIncrementTimeoutNFS,
		RecyclerPodTemplate:      volume.NewPersistentVolumeRecyclerPodTemplate(),
	}
	if err := attemptToLoadRecycler(flags.PersistentVolumeRecyclerPodTemplateFilePathNFS, &nfsConfig); err != nil {
		glog.Fatalf("Could not create NFS recycler pod from file %s: %+v", err)
	}
	allPlugins = append(allPlugins, nfs.ProbeVolumePlugins(nfsConfig)...)

	allPlugins = append(allPlugins, aws_ebs.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, gce_pd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, cinder.ProbeVolumePlugins()...)

	return allPlugins
}

// NewVolumeProvisionersForCloud maps a cloud provider to a specific volume plugin.
func NewVolumeProvisionersForCloud(cloud cloudprovider.Interface) map[string]volume.ProvisionableVolumePlugin {
	var provisioner volume.VolumePlugin
	switch {
	case cloud != nil && cloud.ProviderName() == aws_cloud.ProviderName:
		provisioner = aws_ebs.ProbeVolumePlugins()[0]
	case cloud != nil && cloud.ProviderName() == gce_cloud.ProviderName:
		provisioner = gce_pd.ProbeVolumePlugins()[0]
	case cloud != nil && cloud.ProviderName() == openstack.ProviderName:
		provisioner = cinder.ProbeVolumePlugins()[0]
	}

	plugins := map[string]volume.ProvisionableVolumePlugin{
		"experimental-hostpath-testing-only": host_path.ProbeVolumePlugins(volume.VolumeConfig{})[0].(volume.ProvisionableVolumePlugin),
	}

	if provisioner != nil {
		plugins["gold"] = provisioner.(volume.ProvisionableVolumePlugin)
		plugins["silver"] = provisioner.(volume.ProvisionableVolumePlugin)
		plugins["bronze"] = provisioner.(volume.ProvisionableVolumePlugin)
	}

	return plugins
}

// attemptToLoadRecycler tries decoding a pod from a filepath for use as a recycler for a volume.
// If successful, this method will set the recycler on the config.
// If unsucessful, an error is returned.
func attemptToLoadRecycler(path string, config *volume.VolumeConfig) error {
	if path != "" {
		recyclerPod, err := io.LoadPodFromFile(path)
		if err != nil {
			return err
		}
		config.RecyclerPodTemplate = recyclerPod
	}
	return nil
}
