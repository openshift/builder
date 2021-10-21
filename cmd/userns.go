package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/storage/pkg/unshare"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/syndtr/gocapability/capability"
	"k8s.io/klog/v2"
)

const usernsMarkerVariable = "BUILDER_USERNS_CONFIGURED"

func parseIDMappings(uidmap, mustMapUIDs, gidmap, mustMapGIDs string) ([]specs.LinuxIDMapping, []specs.LinuxIDMapping) {
	// helper for parsing a string of the form "container:host:size[,container:host:size...]"
	parseMapping := func(what, mapSpec string) []specs.LinuxIDMapping {
		var mapping []specs.LinuxIDMapping
		for _, entry := range strings.Split(mapSpec, ",") {
			if entry == "" {
				continue
			}
			triple := strings.Split(entry, ":")
			if len(triple) != 3 {
				klog.Errorf("Invalid format for %s entry %q\n", what, entry)
				return nil
			}
			containerID, err := strconv.ParseUint(triple[0], 10, 32)
			if err != nil {
				klog.Errorf("Invalid format for %s entry %q container ID %q: %v\n", what, entry, triple[0], err)
				return nil
			}
			hostID, err := strconv.ParseUint(triple[1], 10, 32)
			if err != nil {
				klog.Errorf("Invalid format for %s entry %q host ID %q: %v\n", what, entry, triple[1], err)
				return nil
			}
			size, err := strconv.ParseUint(triple[2], 10, 32)
			if err != nil {
				klog.Errorf("Invalid format for %s entry %q size %q: %v\n", what, entry, triple[2], err)
				return nil
			}
			mapping = append(mapping, specs.LinuxIDMapping{
				ContainerID: uint32(containerID),
				HostID:      uint32(hostID),
				Size:        uint32(size),
			})
		}
		return mapping
	}
	parseMustMap := func(what, list string) ([]uint32, error) {
		var results []uint32
		for _, spec := range strings.Split(list, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			u, err := strconv.ParseUint(spec, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("parsing %s value %q: %v", what, spec, err)
			}
			results = append(results, uint32(u))
		}
		return results, nil
	}

	// Return what's already in place, or whatever was specified.
	UIDs, GIDs, err := unshare.GetHostIDMappings("")
	if err != nil {
		klog.Fatalf("Error reading current ID mappings: %v\n", err)
	}
	if os.Geteuid() != 0 {
		uid := fmt.Sprintf("%d", os.Geteuid())
		UIDs, GIDs, err = unshare.GetSubIDMappings(uid, uid)
		if err != nil {
			klog.Fatalf("Error reading ID mappings for %s: %v\n", err)
		}
	}
	if uidMappings := parseMapping("uidmap", uidmap); len(uidMappings) != 0 {
		UIDs = uidMappings
	}
	if gidMappings := parseMapping("gidmap", gidmap); len(gidMappings) != 0 {
		GIDs = gidMappings
	}
	if len(UIDs) > 0 {
		list, err := parseMustMap("must-map-uid", mustMapUIDs)
		if err != nil {
			klog.Fatalf("%v", err)
		}
		if UIDs, err = mustMap(UIDs, list...); err != nil {
			klog.Fatalf("Error updating UID map to ensure that must-map UIDs are mapped %s: %v\n", err)
		}
	}
	if len(GIDs) > 0 {
		list, err := parseMustMap("must-map-gid", mustMapGIDs)
		if err != nil {
			klog.Fatalf("%v", err)
		}
		if GIDs, err = mustMap(GIDs, list...); err != nil {
			klog.Fatalf("Error updating GID map to ensure that must-map GIDs are mapped %s: %v\n", err)
		}
	}
	return UIDs, GIDs
}

func mustMap(input []specs.LinuxIDMapping, requirements ...uint32) ([]specs.LinuxIDMapping, error) {
	sort.Slice(requirements, func(i, j int) bool {
		return requirements[i] < requirements[j]
	})
	output := append([]specs.LinuxIDMapping{}, input...)
	sort.Slice(output, func(i, j int) bool {
		return output[i].ContainerID < output[j].ContainerID
	})
	for i := range requirements {
		requirement := requirements[len(requirements)-i-1]
		present := false
		for j := range output {
			if output[j].ContainerID <= requirement && requirement < output[j].ContainerID+output[j].Size {
				present = true
				break
			}
		}
		if present {
			continue
		}
		use := -1
		for j := range output {
			candidate := len(output) - j - 1
			if output[candidate].Size > 1 {
				use = candidate
				break
			}
		}
		if use == -1 {
			return nil, fmt.Errorf("unable to select a range with an ID that could be used for %d", requirement)
		}
		output[use].Size--
		freedID := output[use].HostID + output[use].Size
		output = append(append(append([]specs.LinuxIDMapping{}, output[:use+1]...), specs.LinuxIDMapping{
			HostID:      freedID,
			ContainerID: requirement,
			Size:        1,
		}), output[use+1:]...)
	}
	return output, nil
}

func inUserNamespace() bool {
	return os.Getenv(usernsMarkerVariable) != ""
}

func maybeReexecUsingUserNamespace(uidmap, mustMapUIDs string, useNewuidmap bool, gidmap, mustMapGIDs string, useNewgidmap bool) {
	// If we've already done all of this, there's no need to do it again.
	if inUserNamespace() {
		return
	}

	// If there's nothing to do, just return.
	if uidmap == "" && gidmap == "" && os.Geteuid() == 0 {
		if caps, err := capability.NewPid(0); err == nil && caps.Get(capability.EFFECTIVE, capability.CAP_SYS_ADMIN) {
			return
		}
	}

	// Parse our --uidmap and --gidmap flags into ID mappings and re-exec ourselves.
	cmd := unshare.Command(append([]string{fmt.Sprintf("%s-in-a-user-namespace", os.Args[0])}, os.Args[1:]...)...)

	// Set up a new user namespace with the ID mappings.
	cmd.UnshareFlags = syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS
	cmd.UidMappings, cmd.GidMappings = parseIDMappings(uidmap, mustMapUIDs, gidmap, mustMapGIDs)
	cmd.UseNewuidmap, cmd.UseNewgidmap = useNewuidmap, useNewgidmap
	cmd.GidMappingsEnableSetgroups = true

	// Set markers so that we know we've done all of this already, and set
	// HOME so that the child doesn't try to read configuration from
	// /root/.config, which it can't if it's another user that's being told
	// it's root because it's running in a user namespace, which would
	// trigger a permissions error.  HOME also needs to be writable.
	cmd.Env = append(os.Environ(), usernsMarkerVariable+"=done", "HOME=/var/lib/containers")

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	unshare.ExecRunnable(cmd, nil)
	klog.Fatalf("Internal error: should not have gotten back from ExecRunnable().\n")
}
