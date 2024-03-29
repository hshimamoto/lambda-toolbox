// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EC2Client struct {
	InstanceIds []string
	VpcId       *string
	client      *ec2.Client
}

func NewEC2Client() (*EC2Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	client := &EC2Client{
		client: ec2.NewFromConfig(cfg),
	}
	return client, nil
}

func (cli *EC2Client) CreateTags(id string, kvs map[string]string) error {
	tag := func(key, val string) types.Tag {
		tagKey := key
		tagValue := val
		return types.Tag{
			Key:   &tagKey,
			Value: &tagValue,
		}
	}
	tags := []types.Tag{}
	for k, v := range kvs {
		tags = append(tags, tag(k, v))
	}
	input := &ec2.CreateTagsInput{
		Resources: []string{id},
		Tags:      tags,
	}
	_, err := cli.client.CreateTags(context.TODO(), input)
	return err
}

func (cli *EC2Client) DescribeInstances() ([]types.Instance, error) {
	// create filter
	input := &ec2.DescribeInstancesInput{
		InstanceIds: cli.InstanceIds,
	}
	if cli.VpcId != nil {
		fname := "vpc-id"
		filter := types.Filter{
			Name:   &fname,
			Values: []string{*cli.VpcId},
		}
		input.Filters = append(input.Filters, filter)
	}
	output, err := cli.client.DescribeInstances(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	instances := []types.Instance{}
	for _, r := range output.Reservations {
		instances = append(instances, r.Instances...)
	}
	return instances, nil
}

func (cli *EC2Client) DescribeVpcs() ([]types.Vpc, error) {
	input := &ec2.DescribeVpcsInput{}
	output, err := cli.client.DescribeVpcs(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Vpcs, nil
}

func (cli *EC2Client) DescribeSubnets() ([]types.Subnet, error) {
	input := &ec2.DescribeSubnetsInput{}
	if cli.VpcId != nil {
		fname := "vpc-id"
		filter := types.Filter{
			Name:   &fname,
			Values: []string{*cli.VpcId},
		}
		input.Filters = append(input.Filters, filter)
	}
	output, err := cli.client.DescribeSubnets(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Subnets, nil
}

func (cli *EC2Client) DescribeSecurityGroups() ([]types.SecurityGroup, error) {
	input := &ec2.DescribeSecurityGroupsInput{}
	if cli.VpcId != nil {
		fname := "vpc-id"
		filter := types.Filter{
			Name:   &fname,
			Values: []string{*cli.VpcId},
		}
		input.Filters = append(input.Filters, filter)
	}
	output, err := cli.client.DescribeSecurityGroups(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.SecurityGroups, nil
}

func (cli *EC2Client) DescribeNetworkInterfaces(nics []string) ([]types.NetworkInterface, error) {
	input := &ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: nics,
	}
	if cli.VpcId != nil {
		fname := "vpc-id"
		filter := types.Filter{
			Name:   &fname,
			Values: []string{*cli.VpcId},
		}
		input.Filters = append(input.Filters, filter)
	}
	output, err := cli.client.DescribeNetworkInterfaces(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.NetworkInterfaces, nil
}

func (cli *EC2Client) DescribeVolumes() ([]types.Volume, error) {
	input := &ec2.DescribeVolumesInput{}
	output, err := cli.client.DescribeVolumes(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Volumes, nil
}

func (cli *EC2Client) CreateVolume(az string, sz int32) (string, error) {
	input := &ec2.CreateVolumeInput{
		AvailabilityZone: &az,
		Size:             &sz,
		VolumeType:       "gp3",
	}
	output, err := cli.client.CreateVolume(context.TODO(), input)
	if err != nil {
		return "", err
	}
	return *output.VolumeId, nil
}

func (cli *EC2Client) DeleteVolume(volumeid string) error {
	input := &ec2.DeleteVolumeInput{
		VolumeId: &volumeid,
	}
	_, err := cli.client.DeleteVolume(context.TODO(), input)
	if err != nil {
		return err
	}
	return nil
}

func (cli *EC2Client) AttachVolume(volumeId, instanceId, device string) error {
	input := &ec2.AttachVolumeInput{
		Device:     &device,
		InstanceId: &instanceId,
		VolumeId:   &volumeId,
	}
	_, err := cli.client.AttachVolume(context.TODO(), input)
	if err != nil {
		return err
	}
	return nil
}

func (cli *EC2Client) DetachVolume(volumeId string) error {
	input := &ec2.DetachVolumeInput{
		VolumeId: &volumeId,
	}
	_, err := cli.client.DetachVolume(context.TODO(), input)
	if err != nil {
		return err
	}
	return nil
}

func (cli *EC2Client) RequestSpotInstances(count int32, ec2spec *EC2InstanceSpec) ([]types.SpotInstanceRequest, error) {
	netspecs, securitygroupids := getNetworkInterfaceSpecification(ec2spec)
	ebsoptimized := true
	spec := &types.RequestSpotLaunchSpecification{
		BlockDeviceMappings: EC2BlockDeviceMappings(ec2spec.VolumeSize, "gp3"),
		EbsOptimized:        &ebsoptimized,
		ImageId:             &ec2spec.ImageId,
		InstanceType:        types.InstanceType(ec2spec.InstanceType),
		KeyName:             ec2spec.KeyName,
		SecurityGroupIds:    securitygroupids,
		NetworkInterfaces:   netspecs,
		UserData:            ec2spec.UserData,
	}
	if ec2spec.ProfileArn != nil {
		spec.IamInstanceProfile = &types.IamInstanceProfileSpecification{
			Arn: ec2spec.ProfileArn,
		}
	}
	tag := func(key, val string) types.Tag {
		tagKey := key
		tagValue := val
		return types.Tag{
			Key:   &tagKey,
			Value: &tagValue,
		}
	}
	tags := []types.Tag{
		tag("lambda-toolbox", "yes"),
	}
	input := &ec2.RequestSpotInstancesInput{
		InstanceCount:       &count,
		LaunchSpecification: spec,
		TagSpecifications: []types.TagSpecification{
			types.TagSpecification{
				ResourceType: types.ResourceTypeSpotInstancesRequest,
				Tags:         tags,
			},
		},
	}
	output, err := cli.client.RequestSpotInstances(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.SpotInstanceRequests, nil
}

func (cli *EC2Client) DescribeSpotInstanceRequests(ids []string) ([]types.SpotInstanceRequest, error) {
	input := &ec2.DescribeSpotInstanceRequestsInput{
		SpotInstanceRequestIds: ids,
	}
	output, err := cli.client.DescribeSpotInstanceRequests(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.SpotInstanceRequests, nil
}

func (cli *EC2Client) DescribeImages(owner, arch, name string) ([]types.Image, error) {
	filter := func(key, val string) types.Filter {
		return types.Filter{Name: &key, Values: []string{val}}
	}
	input := &ec2.DescribeImagesInput{
		Owners: []string{owner},
		Filters: []types.Filter{
			filter("architecture", arch),
			filter("name", name),
		},
	}
	output, err := cli.client.DescribeImages(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Images, nil
}

func (cli *EC2Client) StartInstances(ids []string) ([]types.InstanceStateChange, error) {
	input := &ec2.StartInstancesInput{
		InstanceIds: ids,
	}
	output, err := cli.client.StartInstances(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.StartingInstances, nil
}

func (cli *EC2Client) StopInstances(ids []string, force *bool) ([]types.InstanceStateChange, error) {
	input := &ec2.StopInstancesInput{
		InstanceIds: ids,
		Force:       force,
	}
	output, err := cli.client.StopInstances(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.StoppingInstances, nil
}

func (cli *EC2Client) TerminateInstances(ids []string) ([]types.InstanceStateChange, error) {
	input := &ec2.TerminateInstancesInput{
		InstanceIds: ids,
	}
	output, err := cli.client.TerminateInstances(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.TerminatingInstances, nil
}

func (cli *EC2Client) RunInstances(count int32, ec2spec *EC2InstanceSpec) ([]types.Instance, error) {
	netspecs, securitygroupids := getNetworkInterfaceSpecification(ec2spec)
	ebsoptimized := true
	input := &ec2.RunInstancesInput{
		MaxCount:            &count,
		MinCount:            &count,
		BlockDeviceMappings: EC2BlockDeviceMappings(ec2spec.VolumeSize, "gp3"),
		EbsOptimized:        &ebsoptimized,
		ImageId:             &ec2spec.ImageId,
		InstanceType:        types.InstanceType(ec2spec.InstanceType),
		KeyName:             ec2spec.KeyName,
		SecurityGroupIds:    securitygroupids,
		NetworkInterfaces:   netspecs,
		UserData:            ec2spec.UserData,
	}
	if ec2spec.ProfileArn != nil {
		input.IamInstanceProfile = &types.IamInstanceProfileSpecification{
			Arn: ec2spec.ProfileArn,
		}
	}
	output, err := cli.client.RunInstances(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Instances, nil
}

func (cli *EC2Client) ModifyInstanceAttributeType(instanceid, instancetype string) error {
	input := &ec2.ModifyInstanceAttributeInput{
		InstanceId: &instanceid,
		InstanceType: &types.AttributeValue{
			Value: &instancetype,
		},
	}
	_, err := cli.client.ModifyInstanceAttribute(context.TODO(), input)
	return err
}

func getNetworkInterfaceSpecification(ec2spec *EC2InstanceSpec) ([]types.InstanceNetworkInterfaceSpecification, []string) {
	if ec2spec.SubnetId == nil {
		return nil, ec2spec.SecurityGroupIds
	}
	var index int32 = 0
	return []types.InstanceNetworkInterfaceSpecification{
		types.InstanceNetworkInterfaceSpecification{
			AssociatePublicIpAddress: ec2spec.AssociatePublicIp,
			DeviceIndex:              &index,
			SubnetId:                 ec2spec.SubnetId,
			Groups:                   ec2spec.SecurityGroupIds,
		},
	}, nil
}
