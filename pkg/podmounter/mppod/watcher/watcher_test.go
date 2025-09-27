package watcher_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod/watcher"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"
)

func TestGettingAlreadyScheduledAndReadyPod(t *testing.T) {
	client := fake.NewClientset()

	mpPod := createMountpointPod(t, client, testMountpointPodName)
	mpPod.run()

	mpPodWatcher := createAndStartWatcher(t, client)

	pod, err := mpPodWatcher.Wait(context.Background(), mpPod.pod.Name)
	assert.NoError(t, err)
	assert.Equals(t, mpPod.pod, pod)
}

func TestNodeFiltering(t *testing.T) {
	t.Run("Watcher with specific nodeID filters pods correctly", func(t *testing.T) {
		client := fake.NewClientset()
		nodeID := "test-node-1"

		// Create pod on the target node
		mpPodOnNode := createMountpointPod(t, client, "pod-on-node")
		mpPodOnNode.pod.Spec.NodeName = nodeID
		mpPodOnNode.pod, _ = client.CoreV1().Pods(testMountpointPodNamespace).Update(context.Background(), mpPodOnNode.pod, metav1.UpdateOptions{})
		mpPodOnNode.run()

		// Create pod on a different node
		mpPodOtherNode := createMountpointPod(t, client, "pod-other-node")
		mpPodOtherNode.pod.Spec.NodeName = "other-node"
		mpPodOtherNode.pod, _ = client.CoreV1().Pods(testMountpointPodNamespace).Update(context.Background(), mpPodOtherNode.pod, metav1.UpdateOptions{})
		mpPodOtherNode.run()

		// Create watcher with specific nodeID
		mpPodWatcher := watcher.New(client, testMountpointPodNamespace, nodeID, 10*time.Second)
		stopCh := make(chan struct{})
		t.Cleanup(func() {
			close(stopCh)
		})
		err := mpPodWatcher.Start(stopCh)
		assert.NoError(t, err)

		// Should find pod on the target node
		pod, err := mpPodWatcher.Wait(context.Background(), "pod-on-node")
		assert.NoError(t, err)
		assert.Equals(t, mpPodOnNode.pod.Name, pod.Name)
		assert.Equals(t, nodeID, pod.Spec.NodeName)

		// Should not find pod on other node
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		pod, err = mpPodWatcher.Wait(ctx, "pod-other-node")
		assert.Equals(t, watcher.ErrPodNotFound, err)
		if pod != nil {
			t.Fatalf("Pod should be nil if not found, but got %#v", pod)
		}
	})

	t.Run("Watcher requires nodeID", func(t *testing.T) {
		client := fake.NewClientset()

		// Attempting to create watcher with empty nodeID should panic
		defer func() {
			if r := recover(); r != nil {
				// Expected panic
				expectedMsg := "watcher: nodeID is required and cannot be empty"
				if msg, ok := r.(string); ok && msg == expectedMsg {
					// Test passed
					return
				}
				t.Fatalf("Expected panic with message %q but got %v", expectedMsg, r)
			} else {
				t.Fatal("Expected panic when creating watcher with empty nodeID")
			}
		}()

		// This should panic
		_ = watcher.New(client, testMountpointPodNamespace, "", 10*time.Second)
	})
}

func TestGettingScheduledButNotYetReadyPod(t *testing.T) {
	client := fake.NewClientset()

	mpPod := createMountpointPod(t, client, testMountpointPodName)

	mpPodWatcher := createAndStartWatcher(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	pod, err := mpPodWatcher.Wait(ctx, testMountpointPodName)
	assert.Equals(t, watcher.ErrPodNotReady, err)
	if pod != nil {
		t.Fatalf("Pod should be nil if `watcher.ErrPodNotReady` error returned, but got %#v", pod)
	}

	mpPod.run()

	pod, err = mpPodWatcher.Wait(context.Background(), testMountpointPodName)
	assert.NoError(t, err)
	assert.Equals(t, mpPod.pod, pod)
}

func TestGettingNotYetScheduledPod(t *testing.T) {
	client := fake.NewClientset()

	mpPodWatcher := createAndStartWatcher(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	pod, err := mpPodWatcher.Wait(ctx, testMountpointPodName)
	assert.Equals(t, watcher.ErrPodNotFound, err)
	if pod != nil {
		t.Fatalf("Pod should be nil if `watcher.ErrPodNotFound` error returned, but got %#v", pod)
	}

	mpPod := createMountpointPod(t, client, testMountpointPodName)

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	pod, err = mpPodWatcher.Wait(ctx, testMountpointPodName)
	assert.Equals(t, watcher.ErrPodNotReady, err)
	if pod != nil {
		t.Fatalf("Pod should be nil if `watcher.ErrPodNotReady` error returned, but got %#v", pod)
	}

	mpPod.run()

	pod, err = mpPodWatcher.Wait(context.Background(), testMountpointPodName)
	assert.NoError(t, err)
	assert.Equals(t, mpPod.pod, pod)
}

func TestGet(t *testing.T) {
	t.Run("get existing pod", func(t *testing.T) {
		client := fake.NewClientset()

		mpPod := createMountpointPod(t, client, testMountpointPodName)
		mpPodWatcher := createAndStartWatcher(t, client)

		pod, err := mpPodWatcher.Get(testMountpointPodName)
		assert.NoError(t, err)
		assert.Equals(t, mpPod.pod.Name, pod.Name)
		assert.Equals(t, mpPod.pod.UID, pod.UID)
	})

	t.Run("get non-existent pod", func(t *testing.T) {
		client := fake.NewClientset()

		mpPodWatcher := createAndStartWatcher(t, client)

		pod, err := mpPodWatcher.Get("non-existent-pod")
		if err == nil {
			t.Fatalf("Expected error for non-existent pod, but got nil")
		}
		if pod != nil {
			t.Fatalf("Expected nil pod for non-existent pod, but got %#v", pod)
		}
	})
}

func TestGettingPodsConcurrently(t *testing.T) {
	client := fake.NewClientset()

	mpPodWatcher := createAndStartWatcher(t, client)

	foundPods := make(chan *corev1.Pod)
	for range 5 {
		go func() {
			pod, err := mpPodWatcher.Wait(context.Background(), testMountpointPodName)
			assert.NoError(t, err)
			foundPods <- pod
		}()
	}

	mpPod := createMountpointPod(t, client, testMountpointPodName)
	mpPod.run()

	for range 5 {
		foundPod := <-foundPods
		assert.Equals(t, mpPod.pod, foundPod)
	}
}

func createAndStartWatcher(t *testing.T, client kubernetes.Interface) *watcher.Watcher {
	mpPodWatcher := watcher.New(client, testMountpointPodNamespace, testNodeID, 10*time.Second)

	stopCh := make(chan struct{})
	t.Cleanup(func() {
		close(stopCh)
	})

	err := mpPodWatcher.Start(stopCh)
	assert.NoError(t, err)

	return mpPodWatcher
}

const (
	testMountpointPodName      = "mp-pod"
	testMountpointPodNamespace = "mount-s3-test"
	testNodeID                 = "test-node"
)

type mountpointPod struct {
	t      *testing.T
	client kubernetes.Interface
	pod    *corev1.Pod
}

func createMountpointPod(t *testing.T, client kubernetes.Interface, name string) *mountpointPod {
	t.Helper()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID(uuid.New().String()),
			Name: name,
		},
		Spec: corev1.PodSpec{
			NodeName: testNodeID, // Schedule on the test node
		},
	}
	pod, err := client.CoreV1().Pods(testMountpointPodNamespace).Create(context.Background(), pod, metav1.CreateOptions{})
	assert.NoError(t, err)

	return &mountpointPod{t, client, pod}
}

func (mp *mountpointPod) run() {
	mp.t.Helper()
	mp.pod.Status.Phase = corev1.PodRunning
	var err error
	mp.pod, err = mp.client.CoreV1().Pods(testMountpointPodNamespace).UpdateStatus(context.Background(), mp.pod, metav1.UpdateOptions{})
	assert.NoError(mp.t, err)
}
