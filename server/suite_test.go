package server

import (
	"log" // restrictpkg:ignore to suppress mysql client logs.
	"os"
	"testing"
	"time"

	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/go-logr/stdr"
	"github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	testClusterName   = "moco-agent-test"
	donorHost         = "moco-agent-test-mysqld-donor"
	donorPort         = 3307
	donorServerID     = 1
	replicaHost       = "moco-agent-test-mysqld-replica"
	replicaPort       = 3308
	replicaServerID   = 2
	maxDelayThreshold = 5 * time.Second
)

var (
	testLogger = stdr.New(log.New(os.Stderr, "", log.LstdFlags))
)

func TestAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(2 * time.Minute)
	RunSpecs(t, "Agent Suite")
}

var _ = BeforeSuite(func(done Done) {
	mysql.SetLogger(log.New(GinkgoWriter, "[mysql] ", log.Ldate|log.Ltime|log.Lshortfile))
	metrics.Init(prometheus.DefaultRegisterer, "test", 2)

	os.RemoveAll(socketBaseDir)
	RemoveNetwork()
	CreateNetwork()
	close(done)
}, 60)

var _ = AfterSuite(func() {
	RemoveNetwork()
})
