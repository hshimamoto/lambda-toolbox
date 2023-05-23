// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type Session struct {
	Outputs []string
	Bucket  *Bucket
	Verbose bool
}

func NewSession() *Session {
	bucketname := os.Getenv("BUCKET_NAME")
	s := &Session{}
	b, err := NewBucket(bucketname)
	if err != nil {
		s.Logf("NewBucket: %v", err)
		// ignore error at this point
	}
	s.Bucket = b
	verbose := os.Getenv("VERBOSE")
	if verbose == "yes" || verbose == "true" {
		s.Verbose = true
	}
	return s
}

type PostRequest struct {
	Command           string            `json:command`
	Function          string            `json function,omitempty`
	Zipfile           string            `json zipfile,omitempty`
	Destination       string            `json destination,omitempty`
	Sources           []string          `json sources,omitempty`
	ARN               *string           `json arn,omitempty`
	ARNs              []string          `json arns,omitempty`
	InstanceId        *string           `json instanceid,omitempty`
	InstanceIds       []string          `json instanceids,omitempty`
	VpcId             string            `json vpcid,omitempty`
	SubnetId          *string           `json subnetid,omitempty`
	AssociatePublicIp *bool             `json associatepublicip,omitempty`
	ImageId           *string           `json imageid,omitempty`
	InstanceType      string            `json instancetype,omitempty`
	KeyName           *string           `json keyname,omitempty`
	SecurityGroupIds  []string          `json securitygroupids,omitempty`
	AvailabilityZone  *string           `json az,omitempty`
	VolumeId          *string           `json volumeid,omitempty`
	Device            *string           `json device,omitempty`
	UserDataFile      *string           `json userdatafile,omitempty`
	Name              *string           `json name,omitempty`
	Owner             *string           `json owner,omitempty`
	Tags              map[string]string `json tags,omitempty`
	VolumeSize        *int32            `json volumesize,omitempty`
	ProfileArn        *string           `json profilearn,omitempty`
	ExecCommand       []string          `json execcommand,omitempty`
	Arch              *string           `json arch,omitempty`
	Distro            *string           `json distro,omitempty`
	Count             *int32            `json count,omitempty`
	Cluster           *string           `json cluster,omitempty`
	Group             *string           `json group,omitempty`
	TaskRole          *string           `json taskrole,omitempty`
	Family            *string           `json family,omitempty`
	ExecRole          *string           `json execrole,omitempty`
	Cpu               *string           `json cpu,omitempty`
	Memory            *string           `json memory,omitempty`
	Image             *string           `json image,omitempty`
	Nics              []string          `json nics,omitempty`
	Requests          []PostRequest     `json requests,omitempty`
	Force             *bool             `json force,omitempty`
	// parsed
	cmd  string
	args []string
}

func (s *Session) Logf(f string, args ...interface{}) {
	out := fmt.Sprintf(f, args...)
	if s.Verbose {
		fmt.Printf("%s\n", out)
	}
	s.Outputs = append(s.Outputs, out)
}

func (s *Session) LogLines(lines []string) {
	for _, line := range lines {
		s.Logf("%s", line)
	}
}

func (s *Session) getFile(filename string) ([]byte, error) {
	// try /tmp first
	body, err0 := os.ReadFile("/tmp/" + filename)
	if err0 == nil {
		return body, nil
	}
	body, err1 := s.Bucket.Get(filename)
	if err1 == nil {
		return body, nil
	}
	return nil, fmt.Errorf("%s is not found: (%v) (%v)", filename, err0, err1)
}

type EC2InstanceSpec struct {
	ImageId           string
	SecurityGroupIds  []string
	InstanceType      string
	KeyName           *string
	UserData          *string
	SubnetId          *string
	AssociatePublicIp *bool
	VolumeSize        int32
	ProfileArn        *string
	Tags              map[string]string
}

func (s *Session) newEC2InstanceSpec(req PostRequest) (*EC2InstanceSpec, error) {
	if req.ImageId == nil {
		return nil, fmt.Errorf("no imageid")
	}
	if req.Name == nil {
		return nil, fmt.Errorf("no name")
	}
	var userdata *string = nil
	if req.UserDataFile != nil {
		obj, err := s.getFile(*req.UserDataFile)
		if err != nil {
			return nil, fmt.Errorf("UserDataFile: %v", err)
		}
		data := base64.StdEncoding.EncodeToString(obj)
		userdata = &data
	}
	tags := map[string]string{
		"lambda-toolbox": "yes",
		"Name":           *req.Name,
	}
	envtag := os.Getenv("TAGS")
	if envtag != "" {
		var etags map[string]string
		if json.Unmarshal([]byte(envtag), &etags) == nil {
			for k, v := range etags {
				tags[k] = v
			}
		}
	}
	for k, v := range req.Tags {
		tags[k] = v
	}
	var volumesize int32 = 8
	if req.VolumeSize != nil {
		volumesize = *req.VolumeSize
	}
	return &EC2InstanceSpec{
		ImageId:           *req.ImageId,
		SecurityGroupIds:  req.SecurityGroupIds,
		InstanceType:      req.InstanceType,
		KeyName:           req.KeyName,
		UserData:          userdata,
		SubnetId:          req.SubnetId,
		AssociatePublicIp: req.AssociatePublicIp,
		Tags:              tags,
		VolumeSize:        volumesize,
		ProfileArn:        req.ProfileArn,
	}, nil
}

func (s *Session) doEC2RunInstances(cli *EC2Client, req PostRequest) {
	ec2spec, err := s.newEC2InstanceSpec(req)
	if err != nil {
		s.Logf("newEC2InstanceSpec: %v", err)
		return
	}
	var count int32 = 1
	if req.Count != nil {
		count = *req.Count
	}
	instances, err := cli.RunInstances(count, ec2spec)
	if err != nil {
		s.Logf("RunInstances: %v", err)
		return
	}
	for _, i := range instances {
		cli.SetTags(i, ec2spec.Tags)
		s.Logf("%s", EC2InstanceString(i))
	}
}

func (s *Session) doEC2RequestSpotInstances(cli *EC2Client, req PostRequest) {
	ec2spec, err := s.newEC2InstanceSpec(req)
	if err != nil {
		s.Logf("newEC2InstanceSpec: %v", err)
		return
	}
	var count int32 = 1
	if req.Count != nil {
		count = *req.Count
	}
	sirs, err := cli.RequestSpotInstances(count, ec2spec)
	if err != nil {
		s.Logf("RequestSpotInstances: %v", err)
		return
	}
	ids := []string{}
	for _, sir := range sirs {
		s.Logf("id=%s", *sir.SpotInstanceRequestId)
		ids = append(ids, *sir.SpotInstanceRequestId)
	}
	first := true
	for {
		sirs, err = cli.DescribeSpotInstanceRequests(ids)
		if err != nil {
			s.Logf("DescribeSpotInstanceRequests: %v", err)
			if !first {
				return
			}
			first = false
			time.Sleep(time.Second)
			continue
		}
		fullfilled := true
		for _, sir := range sirs {
			if sir.InstanceId == nil {
				s.Logf("%s is not fullfilled", *sir.SpotInstanceRequestId)
				fullfilled = false
			}
		}
		if fullfilled {
			break
		}
		time.Sleep(time.Second)
	}
	// setup tag
	cli.InstanceIds = nil
	cli.VpcId = nil
	for _, sir := range sirs {
		cli.InstanceIds = append(cli.InstanceIds, *sir.InstanceId)
	}
	instances, err := cli.DescribeInstances()
	if err != nil {
		s.Logf("DescribeInstances: %v", err)
		// why?
		return
	}
	// mark spot instance
	ec2spec.Tags["SpotInstance"] = "yes"
	for _, i := range instances {
		cli.SetTags(i, ec2spec.Tags)
	}
}

func parseInstanceIds(req PostRequest) ([]string, error) {
	ids := req.InstanceIds
	if req.InstanceId != nil {
		ids = append(ids, *req.InstanceId)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no instance ids")
	}
	return ids, nil
}

func (s *Session) showInstancesState(instances []ec2types.InstanceStateChange) {
	for _, i := range instances {
		s.Logf("%s", EC2StateChangeString(i))
	}
}

func (s *Session) doEC2Command(req PostRequest) {
	cli, err := NewEC2Client()
	if err != nil {
		s.Logf("NewEC2Client: %v", err)
		return
	}
	switch req.cmd {
	case "vpcs":
		vpcs, err := cli.DescribeVpcs()
		if err != nil {
			s.Logf("DescribeVpcs: %v", err)
			return
		}
		for _, vpc := range vpcs {
			s.Logf("%s", EC2VpcString(vpc))
		}
	case "subnets":
		cli.VpcId = nil
		if req.VpcId != "" {
			s.Logf("VpcId: %s", req.VpcId)
			cli.VpcId = &req.VpcId
		}
		subnets, err := cli.DescribeSubnets()
		if err != nil {
			s.Logf("DescribeSubnets: %v", err)
			return
		}
		for _, subnet := range subnets {
			s.Logf("%s", EC2SubnetString(subnet))
		}
	case "sgs":
		cli.VpcId = nil
		if req.VpcId != "" {
			s.Logf("VpcId: %s", req.VpcId)
			cli.VpcId = &req.VpcId
		}
		sgs, err := cli.DescribeSecurityGroups()
		if err != nil {
			s.Logf("DescribeSecurityGroups: %v", err)
			return
		}
		for _, sg := range sgs {
			s.Logf("%s", EC2SecurityGroupString(sg))
		}
	case "nics":
		cli.VpcId = nil
		if req.VpcId != "" {
			s.Logf("VpcId: %s", req.VpcId)
			cli.VpcId = &req.VpcId
		}
		nics, err := cli.DescribeNetworkInterfaces(req.Nics)
		if err != nil {
			s.Logf("DescribeNetworkInterfaces: %v", err)
			return
		}
		for _, nic := range nics {
			s.Logf("%s", EC2NetworkInterfaceString(nic))
		}
	case "vols":
		vols, err := cli.DescribeVolumes()
		if err != nil {
			s.Logf("DescribeVolumes: %v", err)
			return
		}
		for _, vol := range vols {
			s.Logf("%s", EC2VolumeString(vol))
		}
	case "images":
		arch := "x86_64"
		if req.Arch != nil {
			arch = *req.Arch
		}
		var image ec2types.Image
		var err error
		if req.Name != nil && req.Owner != nil {
			image, err = cli.GetImage(*req.Name, *req.Owner, arch)
		} else {
			distro := "amazon"
			if req.Distro != nil {
				distro = *req.Distro
			}
			image, err = cli.GetDistroImage(distro, arch)
		}
		if err != nil {
			s.Logf("GetImage: %v", err)
			return
		}
		s.Logf("%s", EC2ImageString(image))
	case "describe", "instances":
		cli.VpcId = nil
		if req.VpcId != "" {
			s.Logf("VpcId: %s", req.VpcId)
			cli.VpcId = &req.VpcId
		}
		instances, err := cli.DescribeInstances()
		if err != nil {
			s.Logf("Describe: %v", err)
			return
		}
		for _, inst := range instances {
			s.Logf("%s", EC2InstanceString(inst))
		}
	case "spotrequest":
		s.doEC2RequestSpotInstances(cli, req)
	case "run":
		s.doEC2RunInstances(cli, req)
	case "start":
		ids, err := parseInstanceIds(req)
		if err != nil {
			s.Logf("start: %s", err)
			return
		}
		instances, err := cli.StartInstances(ids)
		if err != nil {
			s.Logf("StartInstances: %v", err)
			return
		}
		s.showInstancesState(instances)
	case "stop":
		ids, err := parseInstanceIds(req)
		if err != nil {
			s.Logf("stop: %s", err)
			return
		}
		instances, err := cli.StopInstances(ids, req.Force)
		if err != nil {
			s.Logf("StopInstances: %v", err)
			return
		}
		s.showInstancesState(instances)
	case "terminate":
		ids, err := parseInstanceIds(req)
		if err != nil {
			s.Logf("terminate: %s", err)
			return
		}
		instances, err := cli.TerminateInstances(ids)
		if err != nil {
			s.Logf("TerminateInstances: %v", err)
			return
		}
		s.showInstancesState(instances)
	case "rename":
		if req.InstanceId == nil {
			s.Logf("no instanceid")
			return
		}
		if req.Name == nil {
			s.Logf("no name")
			return
		}
		cli.InstanceIds = []string{*req.InstanceId}
		cli.VpcId = nil
		instances, err := cli.DescribeInstances()
		if err != nil {
			s.Logf("DescribeInstances: %v", err)
			return
		}
		if len(instances) != 1 {
			s.Logf("multiple instances")
			return
		}
		prevname := EC2InstanceName(instances[0])
		rename := map[string]string{
			"Name": *req.Name,
		}
		cli.SetTags(instances[0], rename)
		s.Logf("%s: rename %s to %s", *instances[0].InstanceId, prevname, *req.Name)
	case "createvolume":
		if req.AvailabilityZone == nil {
			s.Logf("no az")
			return
		}
		if req.VolumeSize == nil {
			s.Logf("no size")
			return
		}
		volumeid, err := cli.CreateVolume(*req.AvailabilityZone, *req.VolumeSize)
		if err != nil {
			s.Logf("CreateVolume: %v", err)
			return
		}
		s.Logf("Volume %s has been created", volumeid)
		if req.Name != nil {
			cli.CreateTags(volumeid, map[string]string{"Name": *req.Name})
		}
		return
	case "deletevolume":
		if req.VolumeId == nil {
			s.Logf("no volumeid")
			return
		}
		err := cli.DeleteVolume(*req.VolumeId)
		if err != nil {
			s.Logf("DeleteVolume: %v", err)
			return
		}
		return
	case "attachvolume":
		if req.VolumeId == nil {
			s.Logf("no volumeid")
			return
		}
		if req.InstanceId == nil {
			s.Logf("no instanceid")
			return
		}
		volumeId := *req.VolumeId
		instanceId := *req.InstanceId
		device := "/dev/sdf"
		if req.Device != nil {
			device = *req.Device
		}
		err := cli.AttachVolume(volumeId, instanceId, device)
		if err != nil {
			s.Logf("AttachVolume: %v", err)
			return
		}
	case "detachvolume":
		if req.VolumeId == nil {
			s.Logf("no volumeid")
			return
		}
		volumeId := *req.VolumeId
		err := cli.DetachVolume(volumeId)
		if err != nil {
			s.Logf("DetachVolume: %v", err)
			return
		}
	case "change":
		if len(req.args) == 0 {
			s.Logf("need change attributename")
			return
		}
		if req.args[0] != "type" {
			s.Logf("support only type")
			return
		}
		if req.InstanceId == nil {
			s.Logf("no instanceid")
			return
		}
		if req.InstanceType == "" {
			s.Logf("no instancetype")
			return
		}
		if err := cli.ModifyInstanceAttributeType(*req.InstanceId, req.InstanceType); err != nil {
			s.Logf("ModifyInstanceAttributeType: %v", err)
			return
		}
		s.Logf("instance type has been modified")
	}
}

func (s *Session) doECSCommand(req PostRequest) {
	cli, err := NewECSClient()
	if err != nil {
		s.Logf("NewECSClient: %v", err)
		return
	}
	switch req.cmd {
	case "clusters":
		// DescribeClusters API requires cluster names or ARNs
		// first get ARNs with ListClusters
		s.Logf("list clusters")
		arns, err := cli.ListClusters()
		if err != nil {
			s.Logf("ListClusters: %v", err)
			return
		}
		for _, arn := range arns {
			s.Logf("%s", arn)
		}
		// then, call DescribeClusters with ARNs
		s.Logf("describe clusters")
		cls, err := cli.DescribeClusters(arns)
		if err != nil {
			s.Logf("DescribeClusters: %v", err)
			return
		}
		for _, c := range cls {
			s.Logf("%s", *c.ClusterName)
		}
	case "taskdefs":
		arns, err := cli.ListTaskDefinitions()
		if err != nil {
			s.Logf("ListTaskDefinitions: %v", err)
			return
		}
		for _, arn := range arns {
			s.Logf("%s", arn)
		}
	case "taskdef":
		family := req.Family
		if family == nil {
			// old compatibility
			if req.ARN == nil {
				s.Logf("need family or arn")
				return
			}
			s.Logf("please use family")
			family = req.ARN
		}
		taskdefp, err := cli.DescribeTaskDefinition(*family)
		if err != nil {
			s.Logf("DescribeTaskDefinition: %v", err)
			return
		}
		if taskdefp == nil {
			s.Logf("TaskDefinition nil\n")
			return
		}
		s.Logf("%s:%d", *taskdefp.Family, taskdefp.Revision)
		j, err := json.Marshal(taskdefp)
		if err != nil {
			s.Logf("Marshal: %v", err)
			return
		}
		s.Logf("taskdef: %s", j)
	case "regtaskdef":
		if req.Family == nil {
			s.Logf("need family")
			return
		}
		if req.ExecRole == nil {
			s.Logf("need execrole")
			return
		}
		if req.Cpu == nil {
			s.Logf("need cpu")
			return
		}
		if req.Memory == nil {
			s.Logf("need memory")
			return
		}
		cname := "ubuntu"
		if req.Name != nil {
			cname = *req.Name
		}
		cimage := "ubuntu:latest"
		if req.Image != nil {
			cimage = *req.Image
		}
		taskdef, err := cli.RegisterTaskDefinition(*req.Family, *req.Cpu, *req.Memory, *req.ExecRole, cname, cimage)
		if err != nil {
			s.Logf("RegisterTaskDefinition: %v", err)
			return
		}
		s.Logf("%+v", taskdef)
	case "deregtaskdef":
		if req.Family == nil {
			s.Logf("need family")
			return
		}
		taskdef, err := cli.DeregisterTaskDefinition(*req.Family)
		if err != nil {
			s.Logf("DeregisterTaskDefinition: %v", err)
			return
		}
		s.Logf("%+v", taskdef)
	case "tasks", "tasksraw":
		if req.Cluster == nil {
			s.Logf("need cluster")
			return
		}
		taskarns, err := cli.ListTasks(*req.Cluster)
		if err != nil {
			s.Logf("ListTasks: %v", err)
			return
		}
		if len(taskarns) == 0 {
			s.Logf("no tasks")
			return
		}
		tasks, err := cli.DescribeTasks(taskarns, *req.Cluster)
		if err != nil {
			s.Logf("DescribeTasks: %v", err)
			return
		}
		for _, t := range tasks {
			s.Logf("%s", *t.TaskArn)
			s.Logf(" def: %s", *t.TaskDefinitionArn)
			s.Logf(" status: %s", *t.LastStatus)
			s.Logf(" group: %s", *t.Group)
			for _, a := range t.Attachments {
				for _, kv := range a.Details {
					s.Logf("  %s: %s", *kv.Name, *kv.Value)
				}
			}
		}
		if req.cmd == "tasksraw" {
			raw, err := json.Marshal(tasks)
			if err != nil {
				s.Logf("Marshal: %v", err)
				return
			}
			s.Logf("raw: %s", raw)
		}
	case "runtask":
		var count int32 = 1
		if req.Count != nil {
			count = *req.Count
			if count >= 10 {
				s.Logf("count too large")
				return
			}
		}
		if req.ARN == nil {
			s.Logf("need arn")
			return
		}
		if req.Name == nil {
			s.Logf("need name")
			return
		}
		if req.Cluster == nil {
			s.Logf("need cluster")
			return
		}
		if req.SubnetId == nil {
			s.Logf("need subnetid")
			return
		}
		if req.SecurityGroupIds == nil {
			s.Logf("need securitygroupids")
			return
		}
		if req.ExecCommand == nil {
			s.Logf("need execommand")
			return
		}
		taskdefp, err := cli.DescribeTaskDefinition(*req.ARN)
		if err != nil {
			s.Logf("DescribeTaskDefinition: %v", err)
			return
		}
		if taskdefp == nil {
			s.Logf("TaskDefinition nil\n")
			return
		}
		pubip := true
		if req.AssociatePublicIp != nil {
			pubip = *req.AssociatePublicIp
		}
		spot := len(req.args) > 0 && req.args[0] == "spot"
		tasks, err := cli.RunTask(taskdefp, spot, count, req.Group, req.TaskRole, req.Cpu, req.Memory, *req.Name, *req.Cluster, *req.SubnetId, pubip, req.SecurityGroupIds, req.ExecCommand)
		if err != nil {
			s.Logf("RunTask: %v", err)
			return
		}
		if req.Tags != nil {
			s.Logf("TagResource: %v", req.Tags)
			for _, task := range tasks {
				arn := *task.TaskArn
				err := cli.TagResource(arn, req.Tags)
				if err != nil {
					s.Logf("task:%s %v", arn, err)
				}
			}
		}
		for _, task := range tasks {
			s.Logf("starting %s", *task.TaskArn)
		}
	case "stoptask":
		if req.Cluster == nil {
			s.Logf("need cluster")
			return
		}
		arns := req.ARNs
		if len(arns) == 0 {
			if req.ARN == nil {
				s.Logf("need arn")
				return
			}
			arns = []string{*req.ARN}
		}
		for _, arn := range arns {
			task, err := cli.StopTask(arn, *req.Cluster)
			if err != nil {
				s.Logf("StopTask: %v", err)
				continue
			}
			s.Logf("stopping %s", *task.TaskArn)
		}
	case "exec":
		if req.Cluster == nil {
			s.Logf("need cluster")
			return
		}
		if req.ExecCommand == nil {
			s.Logf("need execommand")
			return
		}
		cmd := strings.Join(req.ExecCommand, " ")
		arns := req.ARNs
		if len(arns) == 0 {
			if req.ARN == nil {
				s.Logf("need arn")
				return
			}
			arns = []string{*req.ARN}
		}
		for _, arn := range arns {
			s.Logf("exec %s on %s", cmd, arn)
			err := cli.ExecuteCommand(arn, *req.Cluster, cmd)
			if err != nil {
				s.Logf("ExecuteCommand: %v", err)
			}
		}
	case "tag":
		if req.Tags == nil {
			s.Logf("need tags")
			return
		}
		arns := req.ARNs
		if len(arns) == 0 {
			if req.ARN == nil {
				s.Logf("need arn")
				return
			}
			arns = []string{*req.ARN}
		}
		for _, arn := range arns {
			s.Logf("tags %v on %s", req.Tags, arn)
			err := cli.TagResource(arn, req.Tags)
			if err != nil {
				s.Logf("TagResource: %v", err)
			}
		}
	}
}

func (s *Session) doS3Command(req PostRequest) {
	switch req.cmd {
	case "concat":
		if req.Destination == "" || len(req.Sources) == 0 {
			s.Logf("need destination and sources")
			return
		}
		if err := s.Bucket.ConcatObjects(req.Destination, req.Sources); err != nil {
			s.Logf("ConcatObjects: %v", err)
			return
		}
		s.Logf("concat ok")
	case "store":
		if req.Destination == "" || len(req.Sources) == 0 {
			s.Logf("need destination and sources")
			return
		}
		if err := s.Bucket.StoreObject(req.Destination, req.Sources); err != nil {
			s.Logf("StoreObject: %v", err)
			return
		}
		s.Logf("stored")
	}
}

func (s *Session) doLambdaCommand(req PostRequest) {
	switch req.cmd {
	case "update":
		if req.Function == "" || req.Zipfile == "" {
			s.Logf("need function and zipfile")
			return
		}
		bucketname := os.Getenv("BUCKET_NAME")
		if bucketname == "" {
			s.Logf("no bucket")
			return
		}
		if err := LambdaUpdateFunctionCode(req.Function, bucketname, req.Zipfile); err != nil {
			s.Logf("LambdaUpdateFunctionCode: %v", err)
			return
		}
		s.Logf("update ok")
	}
}

func (s *Session) doSTSCommand(req PostRequest) {
	cli, err := NewSTSClient()
	if err != nil {
		s.Logf("NewSTSClient: %v", err)
		return
	}
	switch req.cmd {
	case "switch":
		if req.ARN == nil {
			s.Logf("need arn")
			return
		}
		cred, err := cli.AssumeRole(*req.ARN)
		if err != nil {
			s.Logf("AssumeRole: %v", err)
			return
		}
		s.Logf("%s %s %s", *cred.AccessKeyId, *cred.SecretAccessKey, *cred.SessionToken)
	}
}

func (s *Session) doExecCommand(req PostRequest) {
	dir := req.Destination
	if dir == "" {
		dir = "/tmp"
	}
	switch req.cmd {
	case "unzip":
		if req.Zipfile == "" {
			s.Logf("no zipfile")
			return
		}
		obj, err := s.Bucket.Get(req.Zipfile)
		if err != nil {
			s.Logf("S3Get: %v", err)
			return
		}
		if err := Unzip(obj, dir); err != nil {
			s.Logf("Unzip: %v", err)
			return
		}
		s.Logf("Unzip: ok")
	case "files":
		lines, err := ExecListFiles(dir)
		if err != nil {
			s.Logf("ListFiles: %v", err)
			return
		}
		s.LogLines(lines)
	case "concat":
		if req.Destination == "" || len(req.Sources) == 0 {
			s.Logf("need destination and sources")
			return
		}
		if err := ExecConcat(req.Destination, req.Sources); err != nil {
			s.Logf("ExecConcat: %v", err)
			return
		}
		s.Logf("concat ok")
	case "run":
		if req.ExecCommand == nil {
			s.Logf("no execcommand")
			return
		}
		lines, err := ExecRun(req.ExecCommand)
		if err != nil {
			s.Logf("Run: %v", err)
			return
		}
		s.LogLines(lines)
	}
}

func (s *Session) handlePostRequest(req PostRequest) {
	if req.Command != "" {
		// parse
		a := strings.Split(req.Command, ".")
		if len(a) == 1 {
			s.Logf("command parse error: %s", req.Command)
			return
		}
		key := a[0]
		req.cmd = a[1]
		req.args = a[2:]
		domap := map[string]func(PostRequest){
			"ec2":    s.doEC2Command,
			"ecs":    s.doECSCommand,
			"s3":     s.doS3Command,
			"lambda": s.doLambdaCommand,
			"sts":    s.doSTSCommand,
			"exec":   s.doExecCommand,
		}
		if f, ok := domap[key]; ok {
			f(req)
		}
		return
	}
	for _, r := range req.Requests {
		s.handlePostRequest(r)
	}
}

func (s *Session) handleJSONRequest(body []byte) {
	var req PostRequest
	err := json.Unmarshal(body, &req)
	if err != nil {
		s.Logf("Unmarshal: %v", err)
		return
	}
	s.handlePostRequest(req)
}

func (s *Session) handleMultipartRequestSubpartS3(key string, obj []byte) {
	if err := s.Bucket.Put(key, obj); err != nil {
		s.Logf("S3Put: %v", err)
		return
	}
}

func (s *Session) handleMultipartRequestSubpartTMP(filename string, obj []byte) {
	if err := os.WriteFile("/tmp/"+filename, obj, 0644); err != nil {
		s.Logf("WriteFile: %v", err)
		return
	}
}

func (s *Session) handleMultipartRequestSubpart(body []byte) {
	// header and content
	a := bytes.SplitN(body, []byte("\r\n\r\n"), 2)
	if len(a) != 2 {
		// no content
		s.Logf("no content")
		return
	}
	cdisp := ""
	ctype := ""
	for _, header := range strings.Split(string(a[0]), "\r\n") {
		// use a again
		a := strings.SplitN(header, ": ", 2)
		if len(a) != 2 {
			continue
		}
		key := strings.ToLower(a[0])
		switch key {
		case "content-disposition":
			cdisp = a[1]
		case "content-type":
			ctype = a[1]
		}
	}
	if cdisp == "" {
		s.Logf("no Disposition")
		return
	}
	s.Logf("type: %s, disp: %s", ctype, cdisp)
	a_disp := strings.Split(cdisp, "; ")
	if a_disp[0] != "form-data" {
		s.Logf("unknown Disposition")
		return
	}
	name := ""
	filename := ""
	for _, p := range a_disp[1:] {
		// use a again
		a := strings.Split(p, "=")
		if len(a) != 2 {
			continue
		}
		key := strings.ToLower(a[0])
		val := a[1]
		if val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		switch key {
		case "name":
			name = val
		case "filename":
			filename = val
		}
	}
	s.Logf("name = %s, filename = %s", name, filename)
	// put it in tmp
	if filename == "" {
		s.Logf("no filename")
		return
	}
	switch name {
	case "file", "s3":
		s.handleMultipartRequestSubpartS3("tmp/"+filename, a[1])
	case "tmp":
		s.handleMultipartRequestSubpartTMP(filename, a[1])
	default:
		s.Logf("unknown name = %s", name)
	}
}

func (s *Session) handleMultipartRequest(boundary string, body []byte) {
	for n, part := range bytes.Split(body, []byte(boundary)) {
		s.Logf("part %d", n)
		s.handleMultipartRequestSubpart(part)
	}
}

func (s *Session) handle(req events.LambdaFunctionURLRequest) {
	switch req.RequestContext.HTTP.Method {
	case "GET":
		s.Logf("Lambda works")
		return
	case "POST": // do nothing
	default:
		s.Logf("Unknown request")
		return
	}
	// IP check
	sourceip := req.RequestContext.HTTP.SourceIP
	allowed := os.Getenv("ALLOWED_IPS")
	deny := true
	for _, ip := range strings.Split(allowed, ",") {
		if sourceip == strings.TrimSpace(ip) {
			deny = false
			break
		}
	}
	allowed_hosts := os.Getenv("ALLOWED_HOSTS")
	for _, host := range strings.Split(allowed_hosts, ",") {
		addr, err := net.ResolveIPAddr("ip4", host)
		if err != nil {
			continue
		}
		if sourceip == addr.String() {
			deny = false
			break
		}
	}
	if deny {
		s.Logf("SourceIP: %s is NOT allowed", sourceip)
		return
	}
	rawbody := []byte(req.Body)
	if req.IsBase64Encoded {
		rawbody, _ = base64.StdEncoding.DecodeString(req.Body)
	}
	ctype, ok := req.Headers["content-type"]
	if !ok {
		s.Logf("No Content-Type")
		return
	}
	if ctype == "application/json" {
		s.handleJSONRequest(rawbody)
		return
	}
	if strings.Index(ctype, "multipart/form-data; boundary=") == 0 {
		boundary := strings.Split(ctype, "boundary=")[1]
		s.handleMultipartRequest("\r\n--"+boundary, rawbody)
		return
	}
	s.Logf("Unknown Content-Type: %s", ctype)
}

// Invoke from Lambda URL
func Handler(req events.LambdaFunctionURLRequest) (string, error) {
	start := time.Now()
	s := NewSession()
	s.Logf("start handler")
	s.handle(req)
	s.Logf("end handler (%v)", time.Since(start))
	return strings.Join(s.Outputs, "\n") + "\n", nil
}

func main() {
	lambda.Start(Handler)
}
