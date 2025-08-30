// Package upgradetest provides Kubernetes manifest definitions for CSI driver upgrade tests.
package upgradetest

import "fmt"

// TestType represents the type of upgrade test
type TestType string

const (
	StaticTest  TestType = "static"
	DynamicTest TestType = "dynamic"
)

// GetStaticPVManifest returns PV for static provisioning test
func GetStaticPVManifest(namespace, bucketName string) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolume
metadata:
  name: upgrade-test-static-pv
spec:
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteMany
  mountOptions:
    - allow-delete
    - cache /tmp/cache
  csi:
    driver: s3.csi.scality.com
    volumeHandle: %s
    volumeAttributes:
      bucketName: %s
`, bucketName, bucketName)
}

// GetStaticTestManifest returns manifests for static provisioning test
func GetStaticTestManifest(namespace string) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: static-pvc
  namespace: %s
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 5Gi
  volumeName: upgrade-test-static-pv
---
apiVersion: v1
kind: Pod
metadata:
  name: static-test-pod
  namespace: %s
  labels:
    test: upgrade-static
spec:
  containers:
  - name: test
    image: ubuntu
    command: ["/bin/bash", "-c"]
    args:
    - |
      apt-get update && apt-get install -y procps
      echo "Static test pod started at $(date)" > /data/startup.log
      
      # Keep pod running
      while true; do
        sleep 10
      done
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: static-pvc
`, namespace, namespace)
}

// GetDynamicTestManifest returns manifests for dynamic provisioning test
func GetDynamicTestManifest(namespace string) string {
	return fmt.Sprintf(`
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-upgrade-test
provisioner: s3.csi.scality.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: s3-secret
  csi.storage.k8s.io/provisioner-secret-namespace: %s
  csi.storage.k8s.io/controller-publish-secret-name: s3-secret
  csi.storage.k8s.io/controller-publish-secret-namespace: %s
mountOptions:
  - allow-delete
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: dynamic-pvc
  namespace: %s
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-csi-upgrade-test
  resources:
    requests:
      storage: 5Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: dynamic-test-pod
  namespace: %s
  labels:
    test: upgrade-dynamic
spec:
  containers:
  - name: test
    image: ubuntu
    command: ["/bin/bash", "-c"]
    args:
    - |
      apt-get update && apt-get install -y procps
      echo "Dynamic test pod started at $(date)" > /data/startup.log
      
      # Keep pod running
      while true; do
        sleep 10
      done
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: dynamic-pvc
`, namespace, namespace, namespace, namespace)
}
