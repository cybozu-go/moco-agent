package test_utils

import (
	"context"
	"fmt"

	"github.com/cybozu-go/well"
)

func StartMinIO(name string, port int) error {
	ctx := context.Background()

	cmd := well.CommandContext(ctx,
		"docker", "run", "-d", "--restart=always",
		"--network="+networkName,
		"--name", name,
		"-p", fmt.Sprintf("%d:%d", port, port),
		"minio/minio", "server", "/data",
	)
	return run(cmd)
}

func StopMinIO(name string) error {
	ctx := context.Background()
	cmd := well.CommandContext(ctx, "docker", "stop", name)
	run(cmd)

	cmd = well.CommandContext(ctx, "docker", "rm", name)
	return run(cmd)
}
