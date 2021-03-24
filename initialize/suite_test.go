package initialize

import (
	"log" // restrictpkg:ignore to suppress mysql client logs.
	"testing"
	"time"

	"github.com/cybozu-go/moco-agent/test_utils"
	"github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	containerName = "moco-agent-init-test"
)

func TestInitialize(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(2 * time.Minute)
	RunSpecs(t, "Initialize Suite")
}

var _ = BeforeSuite(func(done Done) {
	mysql.SetLogger(mysql.Logger(log.New(GinkgoWriter, "[mysql] ", log.Ldate|log.Ltime|log.Lshortfile)))

	test_utils.StopAndRemoveMySQLD(containerName)
	test_utils.RemoveNetwork()

	Eventually(func() error {
		return test_utils.CreateNetwork()
	}, 10*time.Second).Should(Succeed())

	test_utils.CreateSocketDir()

	close(done)
}, 60)

var _ = AfterSuite(func() {
	test_utils.StopAndRemoveMySQLD(containerName)
	test_utils.RemoveNetwork()
	test_utils.RemoveSocketDir()
})

var _ = Describe("Test Initialize", func() {
	Context("TestMySQLUsers", testMySQLUsers)
})
