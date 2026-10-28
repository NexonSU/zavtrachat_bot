//go:build !unix

package main

import "os/exec"

func configureMediaProcess(cmd *exec.Cmd) {}
