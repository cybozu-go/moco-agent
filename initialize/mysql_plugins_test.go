package initialize

import (
	"context"
	"os"
	"time"

	"github.com/cybozu-go/moco-agent/test_utils"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func testMySQLPlugins() {
	var db *sqlx.DB
	var ctx = context.Background()

	It("should ensure plugins", func() {
		err := test_utils.StartMySQLDForTestInit(containerName, socketDir)
		Expect(err).ShouldNot(HaveOccurred())

		var count int
		for {
			time.Sleep(time.Second)
			_, err := os.Stat(socketDir + "/mysqld.sock")
			if err != nil {
				count = 0
				continue
			}
			if count++; count > 10 {
				break
			}
		}

		db, err = GetMySQLConnLocalSocket("root", "", socketDir+"/mysqld.sock", 20)
		Expect(err).ShouldNot(HaveOccurred())

		plugin := Plugin{
			name:   "group_replication",
			soName: "group_replication.so",
		}
		err = ensurePlugin(ctx, db, plugin)
		Expect(err).ShouldNot(HaveOccurred())
		var installed bool
		err = db.GetContext(ctx, &installed, "SELECT COUNT(*) FROM information_schema.plugins WHERE PLUGIN_NAME=? and PLUGIN_STATUS='ACTIVE'", plugin.name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(installed).Should(BeTrue(), "plugin %s should be installed", plugin.name)

		err = ensurePlugin(ctx, db, plugin)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("should be installed plugins required by MOCO", func() {
		err := EnsurePluginsForMOCO(ctx, db)
		Expect(err).ShouldNot(HaveOccurred())

		expected := []string{"rpl_semi_sync_master", "rpl_semi_sync_slave", "clone"}
		for _, n := range expected {
			var installed bool
			err = db.GetContext(ctx, &installed, "SELECT COUNT(*) FROM information_schema.plugins WHERE PLUGIN_NAME=? and PLUGIN_STATUS='ACTIVE'", n)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(installed).Should(BeTrue(), "plugin %s should be installed", n)
		}

		err = test_utils.StopAndRemoveMySQLD(containerName)
		Expect(err).ShouldNot(HaveOccurred())
	})
}
