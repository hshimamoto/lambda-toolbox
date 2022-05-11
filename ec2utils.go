// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func EC2GetTagsAndName(tags []types.Tag) (map[string]string, *string) {
	tagmap := map[string]string{}
	var name *string = nil
	for _, t := range tags {
		if *t.Key == "Name" {
			name = t.Value
			continue
		}
		tagmap[*t.Key] = *t.Value
	}
	return tagmap, name
}

func EC2InstanceName(i types.Instance) string {
	for _, t := range i.Tags {
		if *t.Key == "Name" {
			return *t.Value
		}
	}
	return ""
}

func EC2InstanceString(i types.Instance) string {
	name := ""
	pubip := ""
	if i.PublicIpAddress != nil {
		pubip = *i.PublicIpAddress
	}
	tags, namep := EC2GetTagsAndName(i.Tags)
	if namep != nil {
		name = *namep
	}
	keyval := []string{}
	for k, v := range tags {
		keyval = append(keyval, fmt.Sprintf("%s:%s", k, v))
	}
	sort.Slice(keyval, func(a, b int) bool {
		return keyval[a] < keyval[b]
	})
	return fmt.Sprintf("%s:%s:%s:%s:%s:%v",
		*i.InstanceId, name, i.InstanceType, i.State.Name, pubip, keyval)
}

func EC2VpcString(v types.Vpc) string {
	tags, namep := EC2GetTagsAndName(v.Tags)
	name := ""
	if namep != nil {
		name = *namep
	}
	keyval := []string{}
	for k, v := range tags {
		keyval = append(keyval, fmt.Sprintf("%s:%s", k, v))
	}
	sort.Slice(keyval, func(a, b int) bool {
		return keyval[a] < keyval[b]
	})
	return fmt.Sprintf("%s:%s:%v", *v.VpcId, name, keyval)
}

func EC2SecurityGroupString(sg types.SecurityGroup) string {
	tags, _ := EC2GetTagsAndName(sg.Tags)
	keyval := []string{}
	for k, v := range tags {
		keyval = append(keyval, fmt.Sprintf("%s:%s", k, v))
	}
	sort.Slice(keyval, func(a, b int) bool {
		return keyval[a] < keyval[b]
	})
	groupname := ""
	if sg.GroupName != nil {
		groupname = *sg.GroupName
	}
	return fmt.Sprintf("%s:%s:%s:%v",
		*sg.GroupId, groupname, *sg.VpcId, keyval)
}

func EC2NetworkInterfaceString(nic types.NetworkInterface) string {
	tags, _ := EC2GetTagsAndName(nic.TagSet)
	keyval := []string{}
	for k, v := range tags {
		keyval = append(keyval, fmt.Sprintf("%s:%s", k, v))
	}
	sort.Slice(keyval, func(a, b int) bool {
		return keyval[a] < keyval[b]
	})
	attach := ""
	if nic.Attachment != nil {
		if nic.Attachment.InstanceId != nil {
			attach = *nic.Attachment.InstanceId
		}
	}
	pubip := ""
	if nic.Association != nil {
		assoc := nic.Association
		if assoc.PublicIp != nil {
			pubip = *assoc.PublicIp
		} else if assoc.CarrierIp != nil {
			pubip = *assoc.CarrierIp
		}
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%v",
		*nic.NetworkInterfaceId,
		*nic.VpcId,
		*nic.SubnetId,
		attach,
		*nic.PrivateIpAddress, pubip, keyval)
}

func EC2VolumeString(vol types.Volume) string {
	tags, _ := EC2GetTagsAndName(vol.Tags)
	keyval := []string{}
	for k, v := range tags {
		keyval = append(keyval, fmt.Sprintf("%s:%s", k, v))
	}
	sort.Slice(keyval, func(a, b int) bool {
		return keyval[a] < keyval[b]
	})
	attach := ""
	for _, att := range vol.Attachments {
		attach = *att.InstanceId
	}
	var size int32 = 0
	if vol.Size != nil {
		size = *vol.Size
	}
	return fmt.Sprintf("%s:%s:%d:%s:%v",
		*vol.VolumeId, vol.VolumeType, size,
		attach, keyval)
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
