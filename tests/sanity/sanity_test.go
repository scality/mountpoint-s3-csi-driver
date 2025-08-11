/*
Copyright 2022 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sanity

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/kubernetes/fake"

	sanity "github.com/kubernetes-csi/csi-test/v4/pkg/sanity"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/mounter"
)

const (
	mountPath = "/tmp/csi/mount"
	stagePath = "/tmp/csi/stage"
	socket    = "/tmp/csi.sock"
	endpoint  = "unix://" + socket
)

var s3Driver *driver.Driver

func TestSanity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sanity Tests Suite")
}

// Waits for driver's grpc server to become up.
// Testing framework connects to UDS in a hacky way (which was required before grpc implemented support for UDS),
// which leads to a "Connection timed out" error if `grpc.Dial` was called before UDS created / server started listening:
// https://github.com/kubernetes-csi/csi-test/blob/master/utils/grpcutil.go#L37
func waitDriverIsUp(endpoint string) {
	By("connecting to CSI driver")
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.NewClient(endpoint, dialOptions...)
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()
	client := csi.NewIdentityClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resp, err := client.Probe(ctx, &csi.ProbeRequest{}, grpc.WaitForReady(true))
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.Ready.GetValue()).To(BeTrue())
}

var _ = BeforeSuite(func() {
	// Set up environment for local testing
	os.Setenv("AWS_ENDPOINT_URL", "http://localhost:8000")
	os.Setenv("AWS_ACCESS_KEY_ID", "accessKey1")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "verySecretKey1")
	os.Setenv("AWS_REGION", "us-east-1")

	fakeClient := fake.NewSimpleClientset()
	nodeServer := node.NewS3NodeServer("fake_id", &mounter.FakeMounter{})
	s3Driver = driver.NewDriverForTests(endpoint, "fake_id", nodeServer, fakeClient)

	go func() {
		Expect(s3Driver.Run()).NotTo(HaveOccurred())
	}()
	waitDriverIsUp(endpoint)
})

var _ = AfterSuite(func() {
	s3Driver.Stop()
	Expect(os.RemoveAll(socket)).NotTo(HaveOccurred())
})

var _ = Describe("Scality CSI Driver for S3", func() {
	BeforeEach(func() {
		_ = os.RemoveAll("/tmp/csi")
		_ = os.MkdirAll("/tmp/csi", os.ModePerm)
	})

	config := &sanity.TestConfig{
		Address:        endpoint,
		TargetPath:     mountPath,
		StagingPath:    stagePath,
		TestVolumeSize: 2000 * driver.GiB,
		IDGen:          &sanity.DefaultIDGenerator{},
		DialOptions: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
		TestVolumeParameters: map[string]string{
			"bucketName": "test-bucket-" + sanity.UniqueString("sanity"),
		},
		TestVolumeAccessType: "mount",
	}
	sanity.GinkgoTest(config)
})
