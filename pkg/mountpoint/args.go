package mountpoint

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	ArgForeground                      = "--foreground"
	ArgReadOnly                        = "--read-only"
	ArgAllowOther                      = "--allow-other"
	ArgAllowRoot                       = "--allow-root"
	ArgRegion                          = "--region"
	ArgCache                           = "--cache"
	ArgUserAgentPrefix                 = "--user-agent-prefix"
	ArgAWSMaxAttempts                  = "--aws-max-attempts"
	ArgGid                             = "--gid"
	ArgDirMode                         = "--dir-mode"
	ArgFileMode                        = "--file-mode"
	ArgForcePathStyle                  = "--force-path-style"
	ArgDebug                           = "--debug"
	ArgDebugCRT                        = "--debug-crt"
	ArgProfile                         = "--profile"            // stripped – Driver only supports static Keys, profile is for EKS/EC2 environments
	ArgEndpointURL                     = "--endpoint-url"       // stripped – cluster‑admin controls S3 endpoints
	ArgStorageClass                    = "--storage-class"      // stripped – driver forces bucket default (STANDARD)
	ArgExpressOneZoneCache             = "--cache-xz"           // stripped – Express One Zone shared cache
	ArgExpressOneZoneIncrementalUpload = "--incremental-upload" // stripped – Express One Zone incremental upload
	ArgFsTab                           = "-o"                   // stripped – driver does not support fs-tab
)

// An ArgKey represents the key of an argument.
type ArgKey = string

// An ArgValue represents the value of an argument.
type ArgValue = string

// A value to use in arguments without any value, i.e., an option.
const ArgNoValue = ""

// An arg represents an argument to be passed to Mountpoint.
type arg struct {
	key   ArgKey
	value ArgValue
}

// String returns string representation of the argument to pass Mountpoint.
func (a *arg) String() string {
	if a.value == ArgNoValue {
		return a.key
	}
	return fmt.Sprintf("%s=%s", a.key, a.value)
}

// An Args represents arguments to be passed to Mountpoint during mount.
type Args struct {
	args sets.Set[arg]
}

// ParseArgs parses given list of unnormalized and returns a normalized [Args].
func ParseArgs(passedArgs []string) Args {
	args := sets.New[arg]()

	for _, a := range passedArgs {
		var key, value string

		parts := strings.SplitN(strings.Trim(a, " "), "=", 2)
		if len(parts) == 2 {
			// Ex: `--key=value` or `key=value`
			key, value = parts[0], parts[1]
		} else {
			// Ex: `--key value` or `key value`
			// Ex: `--key` or `key`
			parts = strings.SplitN(strings.Trim(parts[0], " "), " ", 2)
			if len(parts) == 1 {
				// Ex: `--key` or `key`
				key = parts[0]
				value = ArgNoValue
			} else {
				// Ex: `--key value` or `key value`
				key, value = parts[0], strings.Trim(parts[1], " ")
			}
		}

		// prepend -- if it's not already there
		key = normalizeKey(key)

		// disallow options that don't make sense in CSI
		switch key {
		case "--foreground", "-f", "--help", "-h", "--version", "-v":
			continue
		}

		args.Insert(arg{key, value})
	}

	return Args{args}
}

// Set sets or replaces value of given key.
func (a *Args) Set(key ArgKey, value ArgValue) {
	key = normalizeKey(key)
	a.Remove(key)
	a.args.Insert(arg{key, value})
}

// SetIfAbsent sets value of given key only if that key does not exist.
func (a *Args) SetIfAbsent(key ArgKey, value ArgValue) {
	if !a.Has(key) {
		a.Set(key, value)
	}
}

// Value extracts value of given key, it returns extracted value and whether the key was found.
func (a *Args) Value(key ArgKey) (ArgValue, bool) {
	arg, exists := a.find(key)
	return arg.value, exists
}

// Has returns whether given key exists in [Args].
func (a *Args) Has(key ArgKey) bool {
	_, exists := a.find(key)
	return exists
}

// Remove removes given key, it returns the key's value and whether the key was found.
func (a *Args) Remove(key ArgKey) (ArgValue, bool) {
	arg, exists := a.find(key)
	if exists {
		a.args.Delete(arg)
	}
	return arg.value, exists
}

// SortedList returns ordered list of normalized arguments.
func (a *Args) SortedList() []string {
	args := make([]string, 0, a.args.Len())
	for _, arg := range a.args.UnsortedList() {
		args = append(args, arg.String())
	}
	slices.Sort(args)
	return args
}

// find tries to find given key from [Args], and returns whole entry, and whether the key was found.
func (a *Args) find(key ArgKey) (arg, bool) {
	key = normalizeKey(key)
	for _, arg := range a.args.UnsortedList() {
		if key == arg.key {
			return arg, true
		}
	}
	return arg{}, false
}

// normalizeKey normalized given key to have a "--" prefix.
func normalizeKey(key ArgKey) ArgKey {
	if !strings.HasPrefix(key, "-") {
		return "--" + key
	}
	return key
}
