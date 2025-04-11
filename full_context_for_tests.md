# File Operations Test Plan for S3 CSI Driver

This document outlines the test plan for validating basic file operations with the S3 CSI Driver.

## Test Objectives

To verify that the S3 CSI Driver correctly supports all essential file operations when mounting S3 buckets as volumes in Kubernetes pods.

## Test Categories

### 1. Basic File Operations

- **File Creation**
  - Create files of various sizes (empty, small, medium, large)
  - Create files with special characters in names
  - Create files with very long names

- **File Reading**
  - Read entire files of different sizes
  - Perform partial reads (specific byte ranges)
  - Verify content integrity

- **File Updates**
  - Overwrite existing files
  - Append data to existing files
  - Modify specific portions of files

- **File Deletion**
  - Delete individual files
  - Delete multiple files in sequence
  - Attempt to delete non-existent files

### 2. Directory Operations

- **Directory Creation**
  - Create empty directories
  - Create nested directory structures
  - Create directories with special characters

- **Directory Listing**
  - List empty directories
  - List directories with few files
  - List directories with many files
  - List directory hierarchies

- **Directory Deletion**
  - Delete empty directories
  - Delete directories with content
  - Delete nested directory structures

### 3. Metadata and Permissions

- **File Metadata**
  - Check file sizes
  - Check file timestamps
  - Test extended attributes (if supported)

- **File Permissions**
  - Test read/write permissions
  - Test execution permissions (if applicable)
  - Test ownership settings

### 4. Concurrent Operations

- **Multiple Readers**
  - Test multiple pods reading the same file
  - Verify data consistency across readers

- **Multiple Writers**
  - Test multiple pods writing to different files in same volume
  - Test contention handling for same-file writes (if supported)

### 5. Edge Cases

- **Path Handling**
  - Test absolute vs relative paths
  - Test path traversal (../file)
  - Test maximum path length

- **Special Files**
  - Test zero-byte files
  - Test very large files (multi-GB if supported)
  - Test file names with various character sets

## Implementation Details

### Test Suite Structure

The file operations test suite will follow the structure of existing test suites in the codebase:

```go
type s3CSIFileOperationsTestSuite struct {
    tsInfo storageframework.TestSuiteInfo
}

func InitS3CSIFileOperationsTestSuite() storageframework.TestSuite {
    return &s3CSIFileOperationsTestSuite{
        tsInfo: storageframework.TestSuiteInfo{
            Name: "fileoperations",
            TestPatterns: []storageframework.TestPattern{
                storageframework.DefaultFsPreprovisionedPV,
            },
        },
    }
}

func (t *s3CSIFileOperationsTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
    return t.tsInfo
}

func (t *s3CSIFileOperationsTestSuite) SkipUnsupportedTests(_ storageframework.TestDriver, _ storageframework.TestPattern) {
}

func (t *s3CSIFileOperationsTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
    // Test implementations will go here
}
```

### Key Test Implementations

#### File Creation and Reading

Similar to the existing `checkWriteToPath` and `checkReadFromPath` functions, we'll implement specific tests for file operations:

```go
// Generate data with a specific size and seed
data := genBinDataFromSeed(dataSize, seed)
encoded := base64.StdEncoding.EncodeToString(data)

// Write to a file
e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo %s | base64 -d > %s", encoded, path))

// Verify content integrity with SHA256
sum := sha256.Sum256(data)
e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("sha256sum %s | grep -Fq %x", path, sum))
```

#### Directory Operations

For directory tests, we'll use standard shell commands to create and manipulate directories:

```go
// Create a nested directory structure
e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("mkdir -p %s/level1/level2/level3", basePath))

// Create files in various directories
e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo 'file1' > %s/level1/file1.txt", basePath))
e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo 'file2' > %s/level1/level2/file2.txt", basePath))

// Verify directory structure
checkListingPathWithEntries(f, pod, fmt.Sprintf("%s/level1", basePath), []string{"file1.txt", "level2"})
```

#### File Metadata and Permissions

Tests for file metadata and permissions will use standard Linux commands like `stat`:

```go
// Check file permissions
e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -c '%%a %%g %%u' %s | grep '644 %d %d'", 
    filePath, defaultNonRootGroup, defaultNonRootUser))

// Check file size
e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -c '%%s' %s | grep '%d'", 
    filePath, expectedSize))
```

### Utility Functions

We'll use a combination of existing utility functions and new ones:

#### Checking File Operations

```go
// Check if a file exists
func checkFileExists(f *framework.Framework, pod *v1.Pod, path string) {
    e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("test -f %s", path))
}

// Check if a directory exists
func checkDirExists(f *framework.Framework, pod *v1.Pod, path string) {
    e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("test -d %s", path))
}

// Create a file with specific content
func createFileWithContent(f *framework.Framework, pod *v1.Pod, path, content string) {
    e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo '%s' > %s", content, path))
}

// Append to an existing file
func appendToFile(f *framework.Framework, pod *v1.Pod, path, content string) {
    e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo '%s' >> %s", content, path))
}
```

### Concurrent Access Testing

For testing concurrent access, we'll implement a test similar to the existing multivolume test:

```go
testConcurrentAccess := func(ctx context.Context, pvc *v1.PersistentVolumeClaim, numPods int) {
    var pods []*v1.Pod
    node := l.config.ClientNodeSelection
    
    // Create pods
    for i := 0; i < numPods; i++ {
        pod, err := e2epod.CreatePod(ctx, f.ClientSet, f.Namespace.Name, nil, 
                                     []*v1.PersistentVolumeClaim{pvc}, 
                                     admissionapi.LevelBaseline, "")
        framework.ExpectNoError(err)
        pods = append(pods, pod)
    }
    
    // Each pod creates a unique file
    for i, pod := range pods {
        filePath := fmt.Sprintf("/mnt/volume1/file-%d.txt", i)
        content := fmt.Sprintf("Content from pod %d", i)
        createFileWithContent(f, pod, filePath, content)
    }
    
    // Each pod verifies all files
    for _, pod := range pods {
        for i := 0; i < numPods; i++ {
            filePath := fmt.Sprintf("/mnt/volume1/file-%d.txt", i)
            content := fmt.Sprintf("Content from pod %d", i)
            verifyFileContent(f, pod, filePath, content)
        }
    }
}
```

### Performance Considerations

For performance testing, we'll leverage the existing FIO framework:

```go
// Example FIO config for large file read test
func largeFileReadTest(f *framework.Framework, pod *v1.Pod, filePath string) {
    fioCfg := `
[global]
name=large_file_read
bs=1M
runtime=30s
time_based
group_reporting
filename=%s

[sequential_read]
size=1G
rw=read
ioengine=sync
fallocate=none
`
    configPath := "/tmp/large_read.fio"
    e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo '%s' > %s", 
                                     fmt.Sprintf(fioCfg, filePath), configPath))
    e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("fio %s", configPath))
}
```

## S3-Specific Considerations

When implementing the file operations test suite, several S3-specific considerations must be taken into account:

### Object Storage vs. File System

- **Directories are virtual**: S3 is an object store without native directory concepts, so directory operations need special handling
- **Atomic operations**: S3 operations are primarily atomic at the object level, not at the file/partial update level
- **Eventually consistent**: S3 offers eventual consistency which may affect test cases that check for immediate visibility of changes
- **Handling metadata**: S3 objects have their own metadata model which doesn't directly map to file system attributes

### S3 Limitations and Performance Characteristics

1. **List operations**: S3 list operations can be slow for directories with many objects
2. **Small file overhead**: There's significant overhead for small file operations on S3
3. **Prefixes and delimiters**: S3 uses prefixes and delimiters for "directory-like" listing
4. **Sequential vs. random access**: Sequential access patterns perform better than random access
5. **Throughput considerations**: The test suite should measure throughput for different types of operations

### Mountpoint-Specific Considerations

Since the CSI driver uses Mountpoint for S3 as its underlying mounting technology, we should account for:

1. **Caching behavior**: Mountpoint implements various caching mechanisms that may affect test results
2. **Read-after-write consistency**: Test for expected behavior in read-after-write scenarios
3. **Maximum file size**: Test varying file sizes to evaluate performance characteristics
4. **Operations that may be unsupported**: Some standard filesystem operations may be unavailable or behave differently

## Integration with Existing Framework

### Using Common Test Utilities

The file operations test suite will utilize existing test utilities:

1. **Volume framework**: Leverage Kubernetes E2E storage framework
2. **Pod creation helpers**: Use existing pod creation and management functions
3. **Volume resource management**: Use the framework's volume resource lifecycle management
4. **Test assertions**: Use existing assertion utilities for consistent error reporting

### Extension Points

1. **Mount options testing**: Extend existing mount options tests with file operation validation
2. **Multi-volume interactions**: Test file operations across multiple volumes
3. **Cache behavior validation**: Extend cache tests with specific file operation scenarios

### Key Functions to Reuse

```go
// From testsuites/util.go
custom_testsuites.CreateVolumeResourceWithMountOptions() // For creating volumes with specific options
custom_testsuites.CreatePod()                           // For creating pods with volumes
custom_testsuites.PodModifierNonRoot()                  // For testing non-root user scenarios
custom_testsuites.CheckWriteToPath()                    // For writing data to files
custom_testsuites.CheckReadFromPath()                   // For reading data from files
```

## Test Implementation Plan

1. Create a new test suite file `testsuites/fileoperations.go`
2. Implement the core test suite structure
3. Add common utility functions for file operations
4. Implement tests for each category:
   - Basic file operations
   - Directory operations
   - Metadata/permissions tests
   - Concurrent access tests
   - Edge cases
5. Add the test suite to `e2e_test.go`:

```go
var CSITestSuites = []func() framework.TestSuite{
    testsuites.InitVolumesTestSuite,
    custom_testsuites.InitS3CSIMultiVolumeTestSuite,
    custom_testsuites.InitS3MountOptionsTestSuite,
    custom_testsuites.InitS3CSICredentialsTestSuite,
    custom_testsuites.InitS3CSICacheTestSuite,
    custom_testsuites.InitS3CSIFileOperationsTestSuite, // Add new test suite
}
```

## Success Criteria

- All basic file operations work correctly
- File content integrity is maintained
- Directory operations function as expected
- Proper error handling for invalid operations
- Performance meets acceptable thresholds

## Test Environment Requirements

- Kubernetes cluster with S3 CSI driver installed
- Access to S3 endpoint
- Sufficient permissions for all operations
- Multiple worker nodes for concurrent testing 