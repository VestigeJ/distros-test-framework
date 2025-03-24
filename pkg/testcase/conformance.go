package testcase

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/avast/retry-go"

	"github.com/rancher/distros-test-framework/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestSonobuoyMixedOS runs sonobuoy tests for mixed os cluster (linux + windows) node.
func TestSonobuoyMixedOS(deleteWorkload bool, version string) {
	err := shared.InstallSonobuoy("install", version)
	Expect(err).NotTo(HaveOccurred())

	cmd := "sonobuoy run --kubeconfig=" + shared.KubeConfigFile +
		" --plugin my-sonobuoy-plugins/mixed-workload-e2e/mixed-workload-e2e.yaml" +
		" --aggregator-node-selector kubernetes.io/os:linux --wait"
	res, err := shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed output: "+res)

	cmd = "sonobuoy retrieve --kubeconfig=" + shared.KubeConfigFile
	testResultTar, err := shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)

	cmd = "sonobuoy results  " + testResultTar
	res, err = shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
	Expect(res).Should(ContainSubstring("Plugin: mixed-workload-e2e\nStatus: passed\n"))

	if deleteWorkload {
		cmd = "sonobuoy delete --all --wait --kubeconfig=" + shared.KubeConfigFile
		_, err = shared.RunCommandHost(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		err = shared.InstallSonobuoy("delete", version)
		if err != nil {
			GinkgoT().Errorf("error: %v", err)
			return
		}
	}
}

func TestConformance(version string) {
	err := shared.InstallSonobuoy("install", version)
	Expect(err).NotTo(HaveOccurred())

	launchSonobuoyTests()

	statusErr := checkStatus()
	Expect(statusErr).NotTo(HaveOccurred())

	testResultTar := retrieveResultsTar()
	shared.LogLevel("info", "%s", "testResultTar: "+testResultTar)

	results := getResults(testResultTar)
	shared.LogLevel("info", "sonobuoy results: %s", results)

	resultsErr := checkResults(results)
	Expect(resultsErr).NotTo(HaveOccurred())

	// cleanupTests()
}

func launchSonobuoyTests() {
	shared.LogLevel("info", "checking namespace existence")

	cmds := "kubectl get namespace sonobuoy --kubeconfig=" + shared.KubeConfigFile
	res, _ := shared.RunCommandHost(cmds)
	if strings.Contains(res, "Active") {
		shared.LogLevel("info", "%s", "sonobuoy namespace is active, waiting for it to complete")
		return
	}

	if strings.Contains(res, "Error from server (NotFound): namespaces \"sonobuoy\" not found") {
		cmd := "sonobuoy run --kubeconfig=" + shared.KubeConfigFile +
			" --mode=certified-conformance --kubernetes-version=" + shared.ExtractKubeImageVersion()
		_, err := shared.RunCommandHost(cmd)
		Expect(err).NotTo(HaveOccurred())
	}
}

func checkStatus() error {
	shared.LogLevel("info", "checking status of running tests")

	return retry.Do(
		func() error {
			res, err := shared.RunCommandHost("sonobuoy status --kubeconfig=" + shared.KubeConfigFile)
			if err != nil {
				shared.LogLevel("error", "Error checking sonobuoy status: %v", err)
				return fmt.Errorf("sonobuoy status failed: %v", err)
			}

			shared.LogLevel("info", "Sonobuoy Status at %v:\n%s",
				time.Now().Format(time.Kitchen), res)

			if !strings.Contains(res, "Sonobuoy has completed") {
				return fmt.Errorf("sonobuoy has not completed on time, sonobuoy status:\n%s", res)
			}

			return nil
		},
		retry.Attempts(26),
		retry.Delay(10*time.Minute),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, _ error) {
			shared.LogLevel("debug", "Attempt %d: Sonobuoy status check not finished yet, retrying...", n+1)
		}),
	)
}

func retrieveResultsTar() string {
	shared.LogLevel("info", "retrieving sonobuoy results tar")

	cmd := "sonobuoy retrieve --kubeconfig=" + shared.KubeConfigFile
	res, err := shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed cmd: %s\nerror: %v", cmd, err)

	return res
}

func getResults(testResultTar string) string {
	cmd := "sonobuoy results " + testResultTar
	res, err := shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed cmd: %s\nwith output: %s\nerror: %v", cmd, res, err)

	return res
}

//nolint:funlen // just for now, to test on jenkins.
func checkResults(results string) error {
	pluginsPass := strings.Contains(results, "Plugin: systemd-logs\nStatus: passed") &&
		strings.Contains(results, "Plugin: e2e\nStatus: passed")
	if pluginsPass {
		shared.LogLevel("info", "all plugins passed")
		return nil
	}

	failures := extractFailedTests(results)
	if len(failures) == 0 {
		if !strings.Contains(results, "Status: failed") {
			shared.LogLevel("info", "no explicit failures detected")

			return nil
		}
		shared.LogLevel("warn", "status failed but no specific test failures found, proceeding with rerun")
	} else {
		shared.LogLevel("warn", "found %d test failures", len(failures))
	}

	serverFlags := os.Getenv("server_flags")
	if strings.Contains(serverFlags, "cilium") && len(failures) > 0 {
		shared.LogLevel("info", "checking cilium for expected failures")

		nonNetworkFailures := false
		for _, failure := range failures {
			if !strings.Contains(failure, "[sig-network]") {
				nonNetworkFailures = true
				shared.LogLevel("warn", "Found non-network failure: %s", failure)
			}
		}

		if !nonNetworkFailures {
			shared.LogLevel("info", "Cilium CNI detected, all failures are in sig-network namespace, skipping rerun")

			return nil
		}

		shared.LogLevel("warn", "found non-network failures with Cilium CNI, proceeding with rerun")
	}

	newTar := retrieveResultsTar()
	if newTar == "" {
		return errors.New("failed to retrieve results tarball")
	}
	shared.LogLevel("info", "new results tarball: %s", newTar)

	cleanupTests()

	rerunErr := rerunFailedTests(newTar)
	if rerunErr != nil {
		return fmt.Errorf("rerun failed: %w", rerunErr)
	}

	statusErr := checkStatus()
	Expect(statusErr).NotTo(HaveOccurred())

	shared.LogLevel("info", "getting new results after rerun")
	newResults := getResults(newTar)

	newFailures := extractFailedTests(newResults)
	if len(newFailures) > 0 {
		return fmt.Errorf("tests still failing after rerun: %v", newFailures)
	}

	Expect(newResults).ShouldNot(ContainSubstring("Status: failed"), "failed tests: %s", newResults)
	Expect(newResults).ShouldNot(ContainSubstring("Failed tests:"), "failed tests: %s", newResults)

	pluginsPass = strings.Contains(newResults, "Plugin: systemd-logs\nStatus: passed") &&
		strings.Contains(newResults, "Plugin: e2e\nStatus: passed")
	Expect(pluginsPass).Should(BeTrue())

	return nil
}

func extractFailedTests(results string) []string {
	var failures []string

	failedIndex := strings.Index(results, "Failed tests:")
	if failedIndex == -1 {
		return failures
	}

	lines := strings.Split(results[failedIndex:], "\n")
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			break
		}
		failures = append(failures, line)
	}

	return failures
}

func rerunFailedTests(testResultTar string) error {
	cmd := "sonobuoy run --rerun-failed=" + testResultTar + "  --kubeconfig=" + shared.KubeConfigFile +
		" --kubernetes-version=" + shared.ExtractKubeImageVersion()

	shared.LogLevel("info ", "rerunning failed tests with cmd: %s", cmd)

	res, err := exec.Command("bash", "-c", cmd).Output()
	// res, err := shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed cmd: %s\nerror: %v", cmd, err.Error())

	// todo: remove
	shared.LogLevel("info", "rerun sonobuoy tests: RES !!!! %s", res)

	return nil
}

func cleanupTests() {
	shared.LogLevel("info", "cleaning up cluster conformance tests and deleting sonobuoy namespace")

	cmd := "sonobuoy delete --all --wait --kubeconfig=" + shared.KubeConfigFile
	res, err := shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
	Expect(res).Should(ContainSubstring("deleted"))
}
