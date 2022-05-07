// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
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

func ExecRun(exe string) ([]string, error) {
	cmd := exec.Command(exe)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(string(output), "\n"), nil
}
