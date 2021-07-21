package builder

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	s2iapi "github.com/openshift/source-to-image/pkg/api"
)

// GetCGroupLimits returns a struct populated with cgroup limit values gathered
// from the local /sys/fs/cgroup filesystem.  Overflow values are set to
// math.MaxInt64.
func GetCGroupLimits() (*s2iapi.CGroupLimits, error) {
	// see https://git.kernel.org/pub/scm/linux/kernel/git/tj/cgroup.git/tree/Documentation/admin-guide/cgroup-v2.rst
	// for list of cgroupv2 files to try, but Nalin relayed that examination of the crun and runc code that 'memory.high'
	// is not used.
	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	cgroupV2 := cgroups.IsCgroup2UnifiedMode()
	if cgroupV2 {
		// start here in case we are unprivileged
		file = "/sys/fs/cgroup/memory.max"
		if _, err := os.Stat(file); err != nil && errors.Is(err, os.ErrNotExist) {
			cgroupFile, err := cgroups.ParseCgroupFile("/proc/self/cgroup")
			if err != nil {
				return nil, err
			}
			for k, v := range cgroupFile {
				// the empty key is "special case" in /proc/self/cgroup, and we we use its value to find our specific
				// pod among all the pods running on the given host, where since we are running as privileged,
				// /sys/fs/cgroup is bind mounted from the host.
				if len(strings.TrimSpace(k)) == 0 {
					file = filepath.Join("/sys/fs/cgroup", v, "memory.max")
					bytes, e := ioutil.ReadFile(file)
					if e != nil {
						// if we have a setting in /proc/self/cgroup but we have problems reading that file,
						// let's abort as there was an attempt to specify max memory but there is a problem
						// with the specification
						return nil, e
					}
					contents := ""
					if bytes != nil {
						contents = string(bytes)
					}
					log.V(5).Infof("using %s for cgroup2 memory.max with value %s", file, contents)
				}
			}
		}
	}
	byteLimit, err := readMaxStringOrInt64(file)

	if err != nil {
		_, e := os.Stat("/sys/fs/cgroup")
		notExist := false
		if e != nil && errors.Is(e, os.ErrNotExist) {
			notExist = true
		}
		// for systems without cgroups builds should succeed
		if notExist {
			return &s2iapi.CGroupLimits{}, nil
		}
		// otherwise for cgroupv1 error out
		if !cgroupV2 {
			return nil, fmt.Errorf("cannot determine cgroup limits: %w", err)
		}
		// if the cgroupv2 error is anything other than file does not exists, error out
		if !notExist && e != nil {
			return nil, fmt.Errorf("cannot determine cgroup limits: %w", err)
		}
		// otherwise return default for cgroupv2
		return &s2iapi.CGroupLimits{}, nil
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
	log.V(6).Infof("found cgroup values map: %v", cgMap)
	return extractParentFromCgroupMap(cgMap)
}
