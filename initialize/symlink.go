package initialize

import (
	"context"
)

func CreateSymlink(ctx context.Context, target string, source string) error {
	_, err := doExec(ctx, nil, "ln", "-sf", target, source)

	return err
}
