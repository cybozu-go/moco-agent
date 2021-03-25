package initialize

import (
	"context"
	"os"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/test_utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func testMySQLUsers() {
	It("should ensure a user", func() {
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

		db, err := GetMySQLConnLocalSocket("root", "", socketDir+"/mysqld.sock", 20)
		Expect(err).ShouldNot(HaveOccurred())

		ctx := context.Background()
		_, err = db.ExecContext(ctx, "SET GLOBAL partial_revokes='ON'")
		Expect(err).ShouldNot(HaveOccurred())

		By("creating user with revoke privileges")
		user := userSetting{
			name:       "moco-init-test-user-1",
			password:   "password",
			privileges: []string{"ALL"},
			revokePrivileges: map[string][]string{
				"mysql.*": {"INSERT", "CREATE"},
			},
		}
		err = ensureMySQLUser(ctx, db, user)
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming revoked privileges")
		var attr string
		err = db.Get(&attr, "SELECT User_attributes FROM mysql.user WHERE user='moco-init-test-user-1'")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(attr).Should(Equal(`{"Restrictions": [{"Database": "mysql", "Privileges": ["INSERT", "CREATE"]}]}`))

		var grantOp string
		err = db.Get(&grantOp, "SELECT Select_priv FROM mysql.user WHERE user='moco-init-test-user-1'")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(grantOp).Should(Equal("Y"))

		By("recalling with the same user name")
		user = userSetting{
			name:       "moco-init-test-user-1",
			password:   "password",
			privileges: []string{"ALL"},
			revokePrivileges: map[string][]string{
				"mysql.user": {"CREATE"},
			},
		}
		err = ensureMySQLUser(ctx, db, user)
		Expect(err).ShouldNot(HaveOccurred())

		By("creating user with grant option and mysql_native_password plugin")
		user = userSetting{
			name:                    "moco-init-test-user-2",
			password:                "password",
			privileges:              []string{"ALL"},
			withGrantOption:         true,
			useNativePasswordPlugin: true,
		}
		err = ensureMySQLUser(ctx, db, user)
		Expect(err).ShouldNot(HaveOccurred())

		var plugin []byte
		err = db.Get(&plugin, "SELECT plugin FROM mysql.user WHERE user='moco-init-test-user-2'")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(plugin)).Should(Equal("mysql_native_password"))

		err = db.Get(&grantOp, "SELECT Grant_priv FROM mysql.user WHERE user='moco-init-test-user-2'")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(grantOp).Should(Equal("Y"))

		db.Close()
	})

	It("should create MOCO-embedded users", func() {
		By("creating MOCO embedded users")
		db, err := GetMySQLConnLocalSocket("root", "", socketDir+"/mysqld.sock", 20)
		Expect(err).ShouldNot(HaveOccurred())
		defer db.Close()
		err = EnsureMOCOUsers(context.Background(), db)
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming user existens")
		var count int
		err = db.Get(&count, "SELECT COUNT(*) FROM mysql.user WHERE host='%' and user in (?,?,?,?,?,?)",
			mocoagent.AdminUser,
			mocoagent.AgentUser,
			mocoagent.ReplicationUser,
			mocoagent.CloneDonorUser,
			mocoagent.ReadOnlyUser,
			mocoagent.WritableUser)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(count).Should(Equal(6))

		By("dropping local root user is dropped")
		err = DropLocalRootUser(context.Background(), db)
		Expect(err).ShouldNot(HaveOccurred())
		err = db.Get(&count, "SELECT COUNT(*) FROM mysql.user WHERE host='localhost' and user='root'")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(count).Should(Equal(0))

		By("ensuring MOCO embedded users")
		err = EnsureMOCOUsers(context.Background(), db)
		Expect(err).ShouldNot(HaveOccurred())
	})
}
