package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// moco-agent uses scratch for its base image.
// MOCO requires the use of the cp command to copy the moco-init binary
// contained in the moco-agent image to empty-dir.
// The scratch image does not include cp, so we implement the equivalent of the cp command.
// This is the same method used by the kubernetes/kubernetes project to copy etcd binaries to the distroless image.
// refs: https://github.com/kubernetes/kubernetes/blob/v1.25.1/cluster/images/etcd/cp/cp.go
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 3 {
		return errors.New("usage: cp SOURCE DEST")
	}

	sf, err := os.Open(os.Args[1])
	if err != nil {
		return fmt.Errorf("unable to open source file %q: %w", os.Args[1], err)
	}

	defer sf.Close()

	fi, err := sf.Stat()
	if err != nil {
		return fmt.Errorf("unable to stat source file %q: %w", os.Args[1], err)
	}

	if fi.IsDir() {
		return fmt.Errorf("copying directories is not supported: %q", os.Args[1])
	}

	dir := filepath.Dir(os.Args[2])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("unable to create directory %q: %w", dir, err)
	}

	df, err := os.Create(os.Args[2])
	if err != nil {
		return fmt.Errorf("unable to create destination file %q: %w", os.Args[1], err)
	}

	defer df.Close()

	if _, err = io.Copy(df, sf); err != nil {
		return fmt.Errorf("unable to copy %q to %q: %w", os.Args[1], os.Args[2], err)
	}

	if err := df.Sync(); err != nil {
		return fmt.Errorf("unable to flash destination file: %w", err)
	}

	if err := os.Chmod(os.Args[2], fi.Mode()); err != nil {
		return fmt.Errorf("unable to chmod destination file: %w", err)
	}

	return nil
}
