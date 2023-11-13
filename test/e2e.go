package test

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/NibiruChain/nibiru-operator/test/chainnode"
	"github.com/NibiruChain/nibiru-operator/test/chainnodeset"
	"github.com/NibiruChain/nibiru-operator/test/framework"
)

var (
	tf              *framework.TestFramework
	ctx, testCancel = context.WithCancel(context.Background())

	eventuallyTimeout time.Duration
	certsDir          string
	issuerName        string
	workerCount       int
	nodeUtilsImage    string
)

func RunE2ETests(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(eventuallyTimeout)
	RunSpecs(t, "nibiru-operator integration tests suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	logger := log.FromContext(ctx)

	By("bootstrapping test environment")
	var err error
	tf, err = framework.New(ctx,
		framework.WithCertsDir(certsDir),
		framework.WithIssuerName(issuerName),
		framework.WithNodeUtilsImage(nodeUtilsImage),
		framework.WithWorkerCount(workerCount),
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(tf).ToNot(BeNil())

	chainnode.RegisterTestFramework(tf)
	chainnodeset.RegisterTestFramework(tf)

	By("running manager")
	go func() {
		err := tf.RunManager()
		if err != nil {
			logger.Error(err, "error running manager")
		}
		Expect(err).ToNot(HaveOccurred())
	}()
	d := &net.Dialer{Timeout: time.Second}
	Eventually(func() error {
		conn, err := tls.DialWithDialer(d, "tcp", "127.0.0.1:9443", &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	testCancel()
	err := tf.TearDown()
	Expect(err).NotTo(HaveOccurred())
})
