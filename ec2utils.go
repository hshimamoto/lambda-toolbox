// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func EC2InstanceString(i types.Instance) string {
	name := ""
	pubip := ""
	if i.PublicIpAddress != nil {
		pubip = *i.PublicIpAddress
	}
	tags := map[string]string{}
	keys := []string{}
	for _, t := range i.Tags {
		if *t.Key == "Name" {
			name = *t.Value
			continue
		}
		tags[*t.Key] = *t.Value
		keys = append(keys, *t.Key)
	}
	sort.Slice(keys, func(a, b int) bool {
		return keys[a] < keys[b]
	})
	vals := []string{}
	for _, k := range keys {
		vals = append(vals, fmt.Sprintf("%s:%s", k, tags[k]))
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:[%s]",
		*i.InstanceId, name, i.InstanceType, i.State.Name, pubip,
		strings.Join(vals, ","))
}

func EC2ImageString(i types.Image) string {
	return fmt.Sprintf("%s:%s:%s",
		*i.ImageId, *i.Name, *i.Description)
}

func EC2StateChangeString(i types.InstanceStateChange) string {
	return fmt.Sprintf("%s:%s to %s",
		*i.InstanceId, i.PreviousState.Name, i.CurrentState.Name)
}

func EC2BlockDeviceMappings(volsz int32, voltype string) []types.BlockDeviceMapping {
	devname := "/dev/sda1"
	return []types.BlockDeviceMapping{
		types.BlockDeviceMapping{
			DeviceName: &devname,
			Ebs: &types.EbsBlockDevice{
				VolumeSize: &volsz,
				VolumeType: types.VolumeType(voltype),
			},
		},
	}
}

func (cli *EC2Client) GetImage(distro, arch string) (types.Image, error) {
	name := ""
	owner := ""
	switch distro {
	case "amazon":
		name = "amzn2-ami-kernel-*-hvm-*-gp2"
		owner = "amazon"
	case "ubuntu20", "ubuntu":
		name = "ubuntu/*-20.04-*"
		owner = "099720109477"
	case "ubuntu22":
		name = "ubuntu/*-22.04-*"
		owner = "099720109477"
	}
	if name == "" || owner == "" {
		return types.Image{}, fmt.Errorf("no name or owner")
	}
	images, err := cli.DescribeImages(owner, arch, name)
	if err != nil {
		return types.Image{}, err
	}
	if len(images) == 0 {
		return types.Image{}, fmt.Errorf("no images")
	}
	sort.Slice(images, func(a, b int) bool {
		return *images[a].CreationDate > *images[b].CreationDate
	})
	return images[0], nil
}

func (cli *EC2Client) SetTags(i types.Instance, tags map[string]string) []error {
	var errs []error = nil
	// Instance itself
	if err := cli.CreateTags(*i.InstanceId, tags); err != nil {
		errs = append(errs, err)
	}
	// Block Devices
	for _, b := range i.BlockDeviceMappings {
		ebs := b.Ebs
		if ebs == nil || ebs.VolumeId == nil {
			continue
		}
		if err := cli.CreateTags(*ebs.VolumeId, tags); err != nil {
			errs = append(errs, err)
		}
	}
	// Network Interfaces
	for _, n := range i.NetworkInterfaces {
		if n.NetworkInterfaceId == nil {
			continue
		}
		if err := cli.CreateTags(*n.NetworkInterfaceId, tags); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
