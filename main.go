// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type Session struct {
	Outputs []string
	Bucket  *Bucket
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
	return s
}

type PostRequest struct {
	Command          string   `json:command`
	Function         string   `json function,omitempty`
	Zipfile          string   `json zipfile,omitempty`
	Destination      string   `json destination,omitempty`
	Sources          []string `json sources,omitempty`
	VpcId            string   `json vpcid,omitempty`
	ImageId          string   `json imageid,omitempty`
	InstanceType     string   `json instancetype,omitempty`
	KeyName          string   `json keyname,omitempty`
	SecurityGroupIds []string `json securitygroupids,omitempty`
	UserDataFile     string   `json userdatafile,omitempty`
	Name             string   `json name,omitempty`
	ExecCommand      string   `json execcommand,omitempty`
	Arch             string   `json arch,omitempty`
	Distro           string   `json distro,omitempty`
}

func (s *Session) Logf(f string, args ...interface{}) {
	out := fmt.Sprintf(f, args...)
	fmt.Sprintf("%s\n", out)
	s.Outputs = append(s.Outputs, out)
}

func (s *Session) doEC2Command(req PostRequest) {
	cli, err := NewEC2Client()
	if err != nil {
		s.Logf("NewEC2Client: %v", err)
		return
	}
	cmd := req.Command[4:]
	switch cmd {
	case "images":
		distro := req.Distro
		arch := req.Arch
		if distro == "" {
			distro = "amazon"
		}
		if arch == "" {
			arch = "x86_64"
		}
		image, err := cli.GetImage(distro, arch)
		if err != nil {
			s.Logf("GetImage: %v", err)
			return
		}
		s.Logf("%s", EC2ImageString(image))
	case "describe":
		cli.VpcId = req.VpcId
		if cli.VpcId != "" {
			s.Logf("VpcId: %s", cli.VpcId)
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
		var keyname *string = nil
		var userdata *string = nil
		if req.KeyName != "" {
			keyname = &req.KeyName
		}
		if req.UserDataFile != "" {
			obj, err := s.Bucket.Get(req.UserDataFile)
			if err != nil {
				s.Logf("UserDataFile: %v", err)
				return
			}
			data := base64.StdEncoding.EncodeToString(obj)
			userdata = &data
		}
		ebsoptimized := true
		spec := &types.RequestSpotLaunchSpecification{
			BlockDeviceMappings: EC2BlockDeviceMappings(40, "gp3"),
			EbsOptimized:        &ebsoptimized,
			ImageId:             &req.ImageId,
			InstanceType:        types.InstanceType(req.InstanceType),
			KeyName:             keyname,
			SecurityGroupIds:    req.SecurityGroupIds,
			UserData:            userdata,
		}
		sirs, err := cli.RequestSpotInstances(1, spec)
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
		kvs := map[string]string{
			"lambda-toolbox": "yes",
			"Name":           req.Name,
		}
		for _, sir := range sirs {
			if err := cli.CreateTags(*sir.InstanceId, kvs); err != nil {
				s.Logf("CreateTags: %v", err)
				// ignore error
			}
		}
	}
}

func (s *Session) doS3Command(req PostRequest) {
	cmd := req.Command[3:]
	switch cmd {
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
	}
}

func (s *Session) doLambdaCommand(req PostRequest) {
	cmd := req.Command[7:]
	switch cmd {
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

func (s *Session) doExecCommand(req PostRequest) {
	dir := req.Destination
	if dir == "" {
		dir = "/tmp"
	}
	cmd := req.Command[5:]
	switch cmd {
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
		for _, line := range lines {
			s.Logf("%s", line)
		}
	case "run":
		if req.ExecCommand == "" {
			s.Logf("no execcommand")
			return
		}
		lines, err := ExecRun(req.ExecCommand)
		if err != nil {
			s.Logf("Run: %v", err)
			return
		}
		for _, line := range lines {
			s.Logf("%s", line)
		}
	}
}

func (s *Session) handleJSONRequest(body []byte) {
	var req PostRequest
	err := json.Unmarshal(body, &req)
	if err != nil {
		s.Logf("Unmarshal: %v", err)
		return
	}
	if req.Command[0:4] == "ec2." {
		s.doEC2Command(req)
		return
	}
	if req.Command[0:3] == "s3." {
		s.doS3Command(req)
		return
	}
	if req.Command[0:7] == "lambda." {
		s.doLambdaCommand(req)
		return
	}
	if req.Command[0:5] == "exec." {
		s.doExecCommand(req)
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
	if name != "file" {
		s.Logf("unknown name = %s", name)
		return
	}
	// put it in tmp
	if filename == "" {
		s.Logf("no filename")
		return
	}
	if err := s.Bucket.Put("tmp/"+filename, a[1]); err != nil {
		s.Logf("S3Put: %v", err)
		return
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
	fmt.Println(req)
	s := NewSession()
	s.Logf("start handler")
	s.handle(req)
	s.Logf("end handler (%v)", time.Since(start))
	return strings.Join(s.Outputs, "\n") + "\n", nil
}

func main() {
	lambda.Start(Handler)
}
