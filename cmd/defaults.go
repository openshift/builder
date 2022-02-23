package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"

	"k8s.io/klog/v2"
)

// Return "chroot" if we know we're not actually root, "oci" otherwise.
func builderDefaultIsolation() (string, error) {
	if inOurUserNamespace() {
		// We probably don't have enough privileges to use a proper
		// runtime.
		// Lean on the container that we're in being itself
		// unprivileged (i.e., having control groups including the
		// device cgroup configured for us, being provided with a
		// smaller set of devices in /dev, and likely running without a
		// few capabilities that we don't need), and reduce the degree
		// of isolation that we try to use to what we know we're
		// actually allowed to do in an unprivileged container.
		return "chroot", nil
	}
	// Use the proper runtime.
	return "oci", nil
}

// Check that /dev/fuse is usable and we have the fuse-overlayfs helper.
func builderCanUseOverlayFUSE() error {
	device, err := os.Open("/dev/fuse")
	if err != nil {
		return fmt.Errorf("error opening device: %v", err)
	}
	defer device.Close()
	if _, err := os.Stat("/usr/bin/fuse-overlayfs"); err != nil {
		return err
	}
	return nil
}

// Try various storage setups until we find one that works with the privileges
// that we currently have.
func builderDefaultStorage() (string, string, error) {
	for _, candidate := range []struct {
		driver, options string
		also            func() error
	}{
		{"overlay", `["mountopt=metacopy=on"]`, nil},
		{"overlay", ``, nil},
		{"overlay", `["mount_program=/usr/bin/fuse-overlayfs"]`, builderCanUseOverlayFUSE},
		{"vfs", "", nil},
	} {
		var options []string
		// Is there an additional test?
		if candidate.also != nil {
			if why := candidate.also(); why != nil {
				klog.V(2).Info(why.Error())
				continue
			}
		}
		// Are there options for this case?
		if candidate.options != "" {
			err := json.Unmarshal([]byte(candidate.options), &options)
			if err != nil {
				klog.Errorf("internal error parsing options %q: %v", candidate.options, err)
				continue
			}
		}
		// Precreate some things.
		if _, err := os.Stat(fmt.Sprintf("/var/lib/shared/%s-layers/layers.lock", candidate.driver)); err == nil {
			if _, err := os.Stat(fmt.Sprintf("/var/lib/shared/%s-images/images.lock", candidate.driver)); err == nil {
				if _, err := os.Stat(fmt.Sprintf("/var/lib/shared/%s-containers/containers.lock", candidate.driver)); err == nil {
					options = append(options, fmt.Sprintf("%s.imagestore=/var/lib/shared", candidate.driver))
				}
			}
		}
		// Clear out the directory we're about to use.
		os.RemoveAll("/var/lib/containers/storage/tmp")
		os.RemoveAll("/run/containers/storage/tmp")
		// Try to initialize storage.
		store, err := storage.GetStore(storage.StoreOptions{
			GraphRoot:          "/var/lib/containers/storage/tmp",
			RunRoot:            "/run/containers/storage/tmp",
			GraphDriverName:    candidate.driver,
			GraphDriverOptions: options,
		})
		if err != nil {
			klog.V(2).Infof("Unable to initialize storage %q with options %v: %v\n", candidate.driver, options, err)
			continue
		}
		// Shut down the storage that we were able to initialize.
		_, err = store.Shutdown(true)
		// Re-encode the options before returning them.
		reencodedOptions, err := json.Marshal(options)
		if err != nil {
			klog.Errorf("Error re-encoding options %v: %v\n", options, err)
			continue
		}
		klog.Infof("Defaulting to storage driver %q with options %v.\n", candidate.driver, options)
		return candidate.driver, string(reencodedOptions), nil
	}
	return "", "", nil
}
