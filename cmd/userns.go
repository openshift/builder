package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/storage/pkg/unshare"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/syndtr/gocapability/capability"

	"k8s.io/klog/v2"
)

const usernsMarkerVariable = "_CONTAINERS_USERNS_CONFIGURED"

func parseIDMappings(uidmap, gidmap string) ([]specs.LinuxIDMapping, []specs.LinuxIDMapping) {
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

	// Return what's already in place, or whatever was specified.
	UIDs, GIDs, err := unshare.GetHostIDMappings("")
	if err != nil {
		klog.Fatalf("Error reading current ID mappings: %v\n", err)
	}
	if os.Geteuid() != 0 {
		uid := fmt.Sprintf("%d", os.Geteuid())
		UIDs, GIDs, err = unshare.GetSubIDMappings(uid, uid)
		if err != nil {
			klog.Fatalf("Error reading ID mappings for %s: %v\n", uid, err)
		}
	} else {
		for i := range UIDs {
			UIDs[i].HostID = UIDs[i].ContainerID
		}
		for i := range GIDs {
			GIDs[i].HostID = GIDs[i].ContainerID
		}
	}
	if uidMappings := parseMapping("uidmap", uidmap); len(uidMappings) != 0 {
		UIDs = uidMappings
	}
	if gidMappings := parseMapping("gidmap", gidmap); len(gidMappings) != 0 {
		GIDs = gidMappings
	}
	return UIDs, GIDs
}

// inOurUserNamespace returns true if we've already set up a user namespace of
// our own
func inOurUserNamespace() bool {
	return os.Getenv(usernsMarkerVariable) != ""
}

// isNodeDefaultMapping returns true if the current ID map is the default node
// ID map: one range starting at 0, with length 2^32-1, mapped to itself
func isNodeDefaultMapping(m []specs.LinuxIDMapping) bool {
	return len(m) == 1 && m[0].ContainerID == 0 && m[0].HostID == 0 && m[0].Size == 0xffffffff
}

func maybeReexecUsingUserNamespace(uidmap string, useNewuidmap bool, gidmap string, useNewgidmap bool) {
	// If we've already done all of this, there's no need to do it again.
	if inOurUserNamespace() {
		return
	}

	// If there's nothing to do, just return.
	if uidmap == "" && gidmap == "" && os.Geteuid() == 0 {
		if caps, err := capability.NewPid2(0); err == nil {
			if err := caps.Load(); err == nil && caps.Get(capability.EFFECTIVE, capability.CAP_SYS_ADMIN) {
				return
			}
		}
	}

	// Log the ID maps we were started with.
	UIDs, GIDs, err := unshare.GetHostIDMappings("")
	if err != nil {
		klog.Fatalf("Error reading current ID mappings: %v\n", err)
	}
	if !isNodeDefaultMapping(UIDs) || !isNodeDefaultMapping(GIDs) {
		uidMap, gidMap := "[", "["
		for i, entry := range UIDs {
			if i > 0 {
				uidMap += ", "
			}
			uidMap += fmt.Sprintf("(%d:%d:%d)", entry.ContainerID, entry.HostID, entry.Size)
		}
		for i, entry := range GIDs {
			if i > 0 {
				gidMap += ", "
			}
			gidMap += fmt.Sprintf("(%d:%d:%d)", entry.ContainerID, entry.HostID, entry.Size)
		}
		uidMap += "]"
		gidMap += "]"
		klog.V(2).Infof("Started in kernel user namespace with UID map %s and GID map %s.", uidMap, gidMap)
	}

	// Parse our --uidmap and --gidmap flags into ID mappings and re-exec ourselves.
	cmd := unshare.Command(append([]string{fmt.Sprintf("%s-in-a-user-namespace", os.Args[0])}, os.Args[1:]...)...)

	// Set up a new user namespace with the ID mappings.
	cmd.UnshareFlags = syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS
	cmd.UidMappings, cmd.GidMappings = parseIDMappings(uidmap, gidmap)
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
