// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"os"
	"os/exec"
	"strings"
)

func ExecListFiles(dir string) ([]string, error) {
	cmd := exec.Command("ls", "-l", dir)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(string(output), "\n"), nil
}

func ExecConcat(dst string, srcs []string) error {
	// every file must be in /tmp
	f, err := os.Create("/tmp/" + dst)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, src := range srcs {
		obj, err := os.ReadFile("/tmp/" + src)
		if err != nil {
			return err
		}
		if _, err := f.Write(obj); err != nil {
			return err
		}
	}
	return nil
}

func ExecRun(cmdargs []string) ([]string, error) {
	cmd := exec.Command(cmdargs[0], cmdargs[1:]...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(string(output), "\n"), nil
}
