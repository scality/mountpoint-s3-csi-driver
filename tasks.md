# Remove AWS_PROFILE Support for Authentication

**Jira Ticket**: [SCS-XXX]

## Goals
- Remove AWS_PROFILE environment variable support for authentication
- Remove support for the `--profile` mount option for mountpoint-s3
- Remove all AWS profile-related functionality
- Remove AWS_CONFIG_FILE and AWS_SHARED_CREDENTIALS_FILE support for credential resolution
- Add appropriate tests to verify the removal works correctly

## Functional Requirements
1. Remove all AWS_PROFILE references from the codebase
2. Remove the AWS profile provider functionality
3. Remove AWS_CONFIG_FILE and AWS_SHARED_CREDENTIALS_FILE support
4. Block the `--profile` mount option
5. Update tests to verify AWS_PROFILE is not supported
6. Add e2e tests to verify mount options are properly handled

## Task Dashboard

| Phase | Task | Description | Status | Depends On | Commit Message |
|-------|------|-------------|--------|------------|----------------|
| 1     | 1    | Remove AWS_PROFILE environment variable | ✅ Done |            | s3csi-34: Remove AWS_PROFILE support from envprovider<br><br>Remove EnvProfile constant and update affected tests. This is part of the effort to remove AWS profile support from the driver. |
| 1     | 1.1  | Remove EnvProfile constant from pkg/driver/node/envprovider/provider.go | ✅ Done | 1          | |
| 1     | 1.2  | Update tests in pkg/driver/node/envprovider/provider_test.go | ✅ Done | 1.1        | |
| 1     | 2    | Remove AWS Profile provider functionality | ✅ Done |            | s3csi-34: Remove profile provider from credentialprovider<br><br>Remove AWS profile provider section from provider_driver.go and update related tests to eliminate AWS_PROFILE credential method. |
| 1     | 2.1  | Remove profile provider section from pkg/driver/node/credentialprovider/provider_driver.go | ✅ Done | 1          | |
| 1     | 2.2  | Update tests in pkg/driver/node/credentialprovider/provider_test.go | ✅ Done | 2.1        | |
| 2     | 3    | Remove AWS profile package | ✅ Done | 2          | s3csi-34: Remove awsprofile package and references<br><br>Delete the awsprofile directory and clean up imports across the codebase to remove all traces of AWS profile functionality. |
| 2     | 3.1  | Remove entire pkg/driver/node/credentialprovider/awsprofile directory and its tests | ✅ Done | 2.2        | |
| 2     | 3.2  | Clean up imports and references to the awsprofile package | ✅ Done | 3.1        | |
| 3     | 4    | Remove AWS Config File references | ✅ Done | 3         | s3csi-34: Remove AWS config file support<br><br>Remove AWS_CONFIG_FILE and AWS_SHARED_CREDENTIALS_FILE constants and update related code to remove shared credentials files support. |
| 3     | 4.1  | Remove EnvConfigFile and EnvSharedCredentialsFile constants from pkg/driver/node/envprovider/provider.go | ✅ Done | 3        | |
| 3     | 4.2  | Update all code references to these constants | ✅ Done | 4.1      | |
| 3     | 5    | Block --profile flag in mount options | ✅ Done | 4          | s3csi-34: Block --profile flag in mount options<br><br>Add ArgProfile constant and modify mount_args_policy to strip the --profile flag from mount arguments. |
| 3     | 5.1  | Add ArgProfile constant to pkg/mountpoint/args.go | ✅ Done | 4.2        | |
| 3     | 5.2  | Update pkg/driver/node/mounter/mount_args_policy.go to strip --profile flag | ✅ Done | 5.1        | |
| 4     | 6    | Add e2e tests for profile removal | ⬜ To Do | 5          | s3csi-34: Add e2e tests for profile flag stripping<br><br>Add tests to verify --profile mount option is properly stripped and AWS_PROFILE environment variable is not used. |
| 4     | 6.1  | Add test in tests/e2e/customsuites/mountoptions.go to verify --profile is stripped | ⬜ To Do | 5.2        | |
| 4     | 6.2  | Add test to verify AWS_PROFILE env var is no longer used | ⬜ To Do | 6.1        | |
| 5     | 7    | Final validation | ⬜ To Do | 6          | s3csi-34: Perform final validation of AWS_PROFILE removal<br><br>Execute all tests and verify that all authentication methods work properly without AWS_PROFILE support. |
| 5     | 7.1  | Run all tests to ensure changes work correctly | 🟡 In Progress | 6.2        | Unit tests pass, e2e tests pending |
| 5     | 7.2  | Manual verification of authentication methods without AWS_PROFILE | ⬜ To Do | 7.1        | |

## Technical Details

### Phase 1: Remove AWS_PROFILE environment variable

#### Task 1.1: Remove EnvProfile constant
- Location: `pkg/driver/node/envprovider/provider.go`
- Current implementation:
```go
const (
    // ... other env vars
    EnvProfile = "AWS_PROFILE"
    // ... other env vars
)
```
- Expected changes:
  - Remove the `EnvProfile` constant
  - No backward compatibility is needed

#### Task 1.2: Update tests for EnvProfile removal
- Location: `pkg/driver/node/envprovider/provider_test.go`
- Current tests reference `AWS_PROFILE` in environment variable maps
- Expected changes:
  - Remove `AWS_PROFILE` from test environment maps (line 263)
  - Update assertions that include AWS_PROFILE (line 265)

### Phase 2: Remove AWS Profile provider functionality

#### Task 2.1: Remove profile provider section
- Location: `pkg/driver/node/credentialprovider/provider_driver.go`
- Current implementation:
  - Profile provider code in `provideFromDriver` (lines 37-45)
  - `driverLevelLongTermCredentialsProfilePrefix` function (lines 119-121)
  - `provideLongTermCredentialsFromDriver` creates and uses AWS profiles (lines 89-116)
- Expected changes:
  - Remove the profile provider section in `provideFromDriver` (lines 37-45)
  - Remove the `driverLevelLongTermCredentialsProfilePrefix` function (lines 119-121)
  - Update `provideLongTermCredentialsFromDriver` to directly set access keys without creating a profile
  - Remove the `EnvProfile` reference in the returned Environment (line 111)

#### Task 2.2: Update provider_test.go
- Location: `pkg/driver/node/credentialprovider/provider_test.go`
- Current tests contain assertions for AWS profile creation and usage
- Expected changes:
  - Remove the "only profile provider" test case (lines 106-128)
  - Update test cases that assert AWS_PROFILE is set in the environment (lines 146-148, 729-731)
  - Update or remove the assertLongTermCredentials function that checks AWS profile creation (lines 826-840)

### Phase 3: Remove AWS profile package

#### Task 3.1: Remove the awsprofile directory and its tests
- Location: `pkg/driver/node/credentialprovider/awsprofile/`
- Current implementation:
  - `aws_profile.go` contains the AWS profile creation and management code
  - `aws_profile_test.go` contains tests for profile creation and management
  - `awsprofiletest/` directory contains test utilities and helpers
- Expected changes:
  - Remove the entire directory and all files
  - This will remove both implementation and test code

#### Task 3.2: Clean up imports and references
- Locations:
  - `pkg/driver/node/credentialprovider/provider_driver.go` (line 9)
  - `pkg/driver/node/credentialprovider/provider_test.go` (line 17)
  - Any other files that import the package
- Expected changes:
  - Remove imports of the awsprofile package
  - Update/remove any code that references the package

### Phase 4: Remove AWS Config File references

#### Task 4.1: Remove EnvConfigFile and EnvSharedCredentialsFile constants
- Location: `pkg/driver/node/envprovider/provider.go`
- Current implementation:
```go
const (
    // ... other env vars
    EnvConfigFile            = "AWS_CONFIG_FILE"
    EnvSharedCredentialsFile = "AWS_SHARED_CREDENTIALS_FILE"
    // ... other env vars
)
```
- Expected changes:
  - Remove the constants from the list
  - Remove references to these constants in Default() function if present

#### Task 4.2: Update code references
- Locations:
  - Throughout the codebase where AWS_CONFIG_FILE and AWS_SHARED_CREDENTIALS_FILE are used
- Expected changes:
  - Update code that uses these constants for credential resolution
  - Update tests that reference these environment variables

### Phase 5: Block --profile flag in mount options

#### Task 5.1: Add ArgProfile constant
- Location: `pkg/mountpoint/args.go`
- Current implementation:
  - No `ArgProfile` constant exists
  - Other constants like `ArgEndpointURL` that are stripped
- Expected changes:
  - Add a new constant:
    ```go
    ArgProfile = "--profile" // stripped - AWS profile selection not supported
    ```

#### Task 5.2: Update mount_args_policy.go
- Location: `pkg/driver/node/mounter/mount_args_policy.go`
- Current implementation:
  - Has code to strip various args like `ArgEndpointURL` 
- Expected changes:
  - Add code to strip `ArgProfile` similar to other args:
    ```go
    if _, ok := args.Remove(mountpoint.ArgProfile); ok {
        klog.Warningf("--profile ignored: AWS profile credentials are not supported by the CSI driver")
    }
    ```

### Phase 6: Add e2e tests for profile removal

#### Task 6.1: Add test for --profile stripping
- Location: `tests/e2e/customsuites/mountoptions.go`
- Current implementation:
  - Has tests for other stripped options like `--endpoint-url`
- Expected changes:
  - Add a new test case:
    ```go
    ginkgo.It("strips --profile volume level mount flag", func(ctx context.Context) {
        validateStrippedOption(ctx, "--profile=my-aws-profile", "profile")
    })
    ```
  - Add `--profile=my-aws-profile"` to the list of unsupported flags in the multi-flag test (around line 250)

#### Task 6.2: Add test for AWS_PROFILE env var
- Location: `tests/e2e/customsuites/credentials.go` (or create a new test file)
- Expected changes:
  - Add a test that verifies AWS_PROFILE is not used for authentication
  - Test should create a setup with AWS_PROFILE set but invalid
  - Verify authentication still works with valid credentials in other forms

### Phase 7: Final validation

#### Task 7.1: Run all tests
- Run all unit tests: `make test`
- Run all e2e tests: `make e2e-test`
- Verify there are no failures related to the changes

#### Task 7.2: Manual verification
- Test with different authentication methods:
  - IAM roles
  - Access keys directly
  - Web identity token files
- Verify the CSI driver still works correctly

## Risks and Considerations
1. Removing AWS_PROFILE support might affect users who rely on it
2. Authentication failures might occur if proper alternatives aren't in place
3. Test coverage needs to be comprehensive to catch any regressions
4. Several test files reference the AWS_PROFILE functionality and will need significant updates

## Success Criteria
1. All unit and e2e tests pass
2. The CSI driver works correctly without AWS_PROFILE support
3. All authentication methods except AWS_PROFILE continue to work
4. The --profile mount option is properly stripped
5. Documentation is updated to reflect the changes

---
## Plan Context (Jira: SCS-XXX)
In this code phase I want to remove AWS_PROFILE support for authentication that means --profile for mountpoint-s3 and AWS_PROFILE ENV.
We also want to remove like volume level, mount options, dash dash profile.

I want to make sure we add tests for the same, make a plan for this.

Here is documentation for the same
Mountpoint uses the same credentials configuration options as the AWS CLI, and will automatically discover credentials from multiple sources. If you are able to run AWS CLI commands like aws s3 ls against your bucket, you should generally also be able to use Mountpoint against that bucket.

Note

Mountpoint does not currently support authenticating with IAM Identity Center (SSO or Legacy SSO). This issue is tracked in #433.

We recommend you use short-term AWS credentials whenever possible. Mountpoint supports several options for short-term AWS credentials:

When running Mountpoint on an Amazon EC2 instance, you can associate an IAM role with your instance using an instance profile, and Mountpoint will automatically assume that IAM role and manage refreshing the credentials.
When running Mountpoint in an Amazon ECS task, you can similarly associate an IAM role with the task for Mountpoint to automatically assume and manage refreshing the credentials.
You can configure Mountpoint to automatically assume a specific IAM role using the role_arn field of the ~/.aws/config file. This configuration can be useful for cross-account access, where the target IAM role is in a different AWS account. You will need to specify how to obtain credentials that have permission to assume the role with either the source_profile or credential_source fields. For example, if you want Mountpoint to assume the IAM role arn:aws:iam::123456789012:role/marketingadminrole, you can associate an instance profile with your EC2 instance that has permission to assume that role, and then configure a profile in your ~/.aws/config file:
[profile marketingadmin]
role_arn = arn:aws:iam::123456789012:role/marketingadminrole
credential_source = Ec2InstanceMetadata
With this configuration, running Mountpoint with the --profile marketingadmin command-line argument will automatically assume the specified IAM role and manage refreshing the credentials.
Otherwise, you can acquire temporary AWS credentials for an IAM role from the AWS Console or with the aws sts assume-role AWS CLI command, and store them in the ~/.aws/credentials file.
If you need to use long-term AWS credentials, you can store them in the configuration and credentials files in ~/.aws, or specify them with environment variables (AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY).

To manage multiple AWS credentials, you can use the --profile command-line argument or AWS_PROFILE environment variable to select a profile from the configuration and credentials files.

For public buckets that do not require AWS credentials, you can use the --no-sign-request command-line flag to disable AWS credentials. 