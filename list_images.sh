#!/bin/bash

values_file=${1:-./charts/piraeus/values.yaml}
[ ! -f "$values_file" ] && echo Cannot find file $values_file &&  exit 1

# check if yq is installed
if ! yq -V > /dev/null; then
    echo Must Install yq to handle yaml files
    exit 1
fi

# print etcd and ha-controller image
for i in etcd piraeus-ha-controller; do
    cat "$values_file" | yq e ".$i.image | (.repository, .tag)" | sed 'N;s/\n/:/'
done | sed 's/:$/:latest/'

# print scheduler image
sched_img=$( cat "$values_file" | yq e .stork.schedulerImage )
sched_tag=$( cat "$values_file" | yq e .stork.schedulerTag )
if [ -z "$sched_tag" ]; then
    sched_tag=$(kubectl version -o yaml 2>/dev/null | yq .serverVersion.gitVersion)
fi
[ -z "$sched_tag" ] || echo "${sched_img}:${sched_tag}"

# print other images
for i in .stork.storkImage \
         .csi.pluginImage \
         .csi.csiAttacherImage \
         .csi.csiLivenessProbeImage \
         .csi.csiNodeDriverRegistrarImage \
         .csi.csiProvisionerImage \
         .csi.csiSnapshotterImage \
         .csi.csiResizerImage \
         .operator.image \
         .operator.controller.controllerImage \
         .operator.satelliteSet.satelliteImage \
         .operator.satelliteSet.monitoringImage \
         .operator.satelliteSet.kernelModuleInjectionImage \
         ; do
    cat "$values_file" | yq e $i
done | sort -u | sed 's/:$/:latest/'
