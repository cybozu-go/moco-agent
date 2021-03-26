package initialize

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Plugin struct {
	name   string
	soName string
}

var Plugins = []Plugin{
	{
		name:   "rpl_semi_sync_master",
		soName: "semisync_master.so",
	},
	{
		name:   "rpl_semi_sync_slave",
		soName: "semisync_slave.so",
	},
	{
		name:   "clone",
		soName: "mysql_clone.so",
	},
}

func EnsurePluginsForMOCO(ctx context.Context, db *sqlx.DB) error {
	for _, p := range Plugins {
		err := ensurePlugin(ctx, db, p)
		if err != nil {
			return err
		}
	}

	return nil
}

func ensurePlugin(ctx context.Context, db *sqlx.DB, plugin Plugin) error {
	var installed bool
	err := db.GetContext(ctx, &installed, "SELECT COUNT(*) FROM information_schema.plugins WHERE PLUGIN_NAME=? and PLUGIN_STATUS='ACTIVE'", plugin.name)
	if err != nil {
		return err
	}

	if !installed {
		queryStr := fmt.Sprintf(`INSTALL PLUGIN %s SONAME '%s'`, plugin.name, plugin.soName)
		_, err = db.ExecContext(ctx, queryStr)
		if err != nil {
			return err
		}
	}

	return nil
}
