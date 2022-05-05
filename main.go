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
	SecurityGroupIds []string `json securitygroupids,omitempty`
	Name             string   `json name,omitempty`
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
		spec := &types.RequestSpotLaunchSpecification{
			ImageId:          &req.ImageId,
			InstanceType:     EC2InstanceType(req.InstanceType),
			SecurityGroupIds: req.SecurityGroupIds,
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
		for {
			sirs, err = cli.DescribeSpotInstanceRequests(ids)
			if err != nil {
				s.Logf("DescribeSpotInstanceRequests: %v", err)
				return
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
		s.handleMultipartRequest("--"+boundary, rawbody)
		return
	}
	s.Logf("Unknown Content-Type: %s", ctype)
}

// Invoke from Lambda URL
func Handler(req events.LambdaFunctionURLRequest) (string, error) {
	fmt.Println(req)
	s := NewSession()
	s.Logf("start handler")
	s.handle(req)
	return strings.Join(s.Outputs, "\n") + "\n", nil
}

func main() {
	lambda.Start(Handler)
}
