package mounter

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"
)

func TestGetMPPodLock(t *testing.T) {
	// Clear the map before testing
	mpPodLocks = make(map[string]*MPPodLock)

	t.Run("New lock creation", func(t *testing.T) {
		podUID := "pod1"
		lock := getMPPodLock(podUID)

		assert.Equals(t, 1, lock.refCount)
		assert.Equals(t, 1, len(mpPodLocks))
	})

	t.Run("Existing lock retrieval", func(t *testing.T) {
		podUID := "pod2"
		firstLock := getMPPodLock(podUID)
		secondLock := getMPPodLock(podUID)

		if firstLock != secondLock {
			t.Fatal("Expected to get the same lock instance")
		}
		assert.Equals(t, 2, firstLock.refCount)
	})
}

func TestReleaseMPPodLock(t *testing.T) {
	// Clear the map before testing
	mpPodLocks = make(map[string]*MPPodLock)

	t.Run("Release existing lock", func(t *testing.T) {
		podUID := "pod3"
		getMPPodLock(podUID)
		getMPPodLock(podUID)

		releaseMPPodLock(podUID)

		lock, exists := mpPodLocks[podUID]
		assert.Equals(t, true, exists)
		assert.Equals(t, 1, lock.refCount)

		releaseMPPodLock(podUID)

		_, exists = mpPodLocks[podUID]
		assert.Equals(t, false, exists)
	})

	t.Run("Release non-existent lock", func(t *testing.T) {
		podUID := "non-existent-pod"
		releaseMPPodLock(podUID)
		// This test passes if no panic occurs
	})
}

func TestLockMountpointPod_SamePodSerialized(t *testing.T) {
	// Clear the map before testing
	mpPodLocks = make(map[string]*MPPodLock)

	podName := "test-pod-serial"
	var order []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			unlock := lockMountpointPod(podName)
			mu.Lock()
			order = append(order, idx)
			mu.Unlock()
			time.Sleep(10 * time.Millisecond) // Hold lock briefly to enforce serialization
			unlock()
		}(i)
	}
	wg.Wait()

	// All 3 goroutines should have completed
	assert.Equals(t, 3, len(order))

	// After all locks are released, the map entry should be cleaned up
	mpPodLocksMutex.Lock()
	_, exists := mpPodLocks[podName]
	mpPodLocksMutex.Unlock()
	assert.Equals(t, false, exists)
}

func TestLockMountpointPod_DifferentPodsParallel(t *testing.T) {
	// Clear the map before testing
	mpPodLocks = make(map[string]*MPPodLock)

	var wg sync.WaitGroup
	holdTime := 50 * time.Millisecond
	start := time.Now()

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			unlock := lockMountpointPod(fmt.Sprintf("pod-%d", idx))
			time.Sleep(holdTime)
			unlock()
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// If locks for different pods run in parallel, total time should be
	// approximately holdTime, not 3*holdTime. We use 2*holdTime as a
	// generous upper bound to avoid flakiness.
	if elapsed >= 2*holdTime {
		t.Errorf("Expected parallel execution (~%v) but took %v, suggesting serialization", holdTime, elapsed)
	}
}
