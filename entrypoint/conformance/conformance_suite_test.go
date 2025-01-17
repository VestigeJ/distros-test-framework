package sonobuoyconformance

import (
	"flag"
	"os"
	"testing"

	"github.com/rancher/distros-test-framework/config"
	"github.com/rancher/distros-test-framework/pkg/customflag"
	"github.com/rancher/distros-test-framework/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	kubeconfig string
	cluster    *shared.Cluster
)

func TestMain(m *testing.M) {
	flag.StringVar(&customflag.ServiceFlag.External.SonobuoyVersion, "sonobuoyVersion", "0.57.2", "Sonobuoy binary version")
	flag.Var(&customflag.ServiceFlag.Destroy, "destroy", "Destroy cluster after test")
	flag.Parse()

	_, err := config.AddEnv()
	if err != nil {
		shared.LogLevel("error", "error adding env vars: %w\n", err)
		os.Exit(1)
	}

	verifyClusterNodes(cluster)
	kubeconfig = os.Getenv("KUBE_CONFIG")
	if kubeconfig == "" {
		// gets a cluster from terraform.
		cluster = shared.ClusterConfig()
	} else {
		// gets a cluster from kubeconfig.
		cluster = shared.KubeConfigCluster(kubeconfig)
	}

	os.Exit(m.Run())
}

func TestConformance(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Run Conformance Suite")
}

var _ = AfterSuite(func() {
	if customflag.ServiceFlag.Destroy {
		status, err := shared.DestroyCluster()
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})

func verifyClusterNodes(cluster *shared.Cluster) {
	shared.LogLevel("info", "verying cluster configuration matches minimum requirements for conformance tests")
	if cluster.NumAgents < 1 && cluster.NumServers < 1 {
		shared.LogLevel("error", "%s", "cluster must at least consist of 1 server and 1 agent")
		os.Exit(1)
	}
}
