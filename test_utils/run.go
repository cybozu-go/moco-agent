package test_utils

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/cybozu-go/well"
)

func run(cmd *well.LogCmd) error {
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf

	err := cmd.Run()
	stdout := strings.TrimRight(outBuf.String(), "\n")
	if len(stdout) != 0 {
		fmt.Println("[test_utils/stdout] " + stdout)
	}
	stderr := strings.TrimRight(errBuf.String(), "\n")
	if len(stderr) != 0 {
		fmt.Println("[test_utils/stderr] " + stderr)
	}
	return err
}
