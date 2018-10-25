package builder

import (
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	s2iapi "github.com/openshift/source-to-image/pkg/api"
)

// GetCGroupLimits returns a struct populated with cgroup limit values gathered
// from the local /sys/fs/cgroup filesystem.  Overflow values are set to
// math.MaxInt64.
func GetCGroupLimits() (*s2iapi.CGroupLimits, error) {
	byteLimit, err := readInt64("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		// for systems without cgroups builds should succeed
		if _, err := os.Stat("/sys/fs/cgroup"); os.IsNotExist(err) {
			return &s2iapi.CGroupLimits{}, nil
		}
		return nil, fmt.Errorf("cannot determine cgroup limits: %v", err)
	}
	// math.MaxInt64 seems to give cgroups trouble, this value is
	// still 92 terabytes, so it ought to be sufficiently large for
	// our purposes.
	if byteLimit > 92233720368547 {
		byteLimit = 92233720368547
	}

	parent, err := getCgroupParent()
	if err != nil {
		return nil, fmt.Errorf("read cgroup parent: %v", err)
	}

	return &s2iapi.CGroupLimits{
		// Though we are capped on memory and cpu at the cgroup parent level,
		// some build containers care what their memory limit is so they can
		// adapt, thus we need to set the memory limit at the container level
		// too, so that information is available to them.
		MemoryLimitBytes: byteLimit,
		// Set memoryswap==memorylimit, this ensures no swapping occurs.
		// see: https://docs.docker.com/engine/reference/run/#runtime-constraints-on-cpu-and-memory
		MemorySwap: byteLimit,
		Parent:     parent,
	}, nil
}

// getCgroupParent determines the parent cgroup for a container from
// within that container.
func getCgroupParent() (string, error) {
	cgMap, err := cgroups.ParseCgroupFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	glog.V(6).Infof("found cgroup values map: %v", cgMap)
	return extractParentFromCgroupMap(cgMap)
}
