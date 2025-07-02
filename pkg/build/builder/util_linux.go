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
// 92233720368547.
func GetCGroupLimits() (*s2iapi.CGroupLimits, error) {
	// we're going to read the contents of various files we care about,
	// then parse them from the contents, so that we don't need to read
	// a given file more than once
	fileContents := make(map[string]string)
	// see https://git.kernel.org/pub/scm/linux/kernel/git/tj/cgroup.git/tree/Documentation/admin-guide/cgroup-v2.rst
	// for list of cgroupv2 files to try, but @nalind relayed that examination of the crun and runc code that 'memory.high'
	// is not used.
	memoryLimitFile := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	parseMemoryLimitFile := readMaxStringOrInt64
	cpuQuotaFile := "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
	parseCpuQuotaFile := readMaxStringOrInt64
	scaleCpuQuota := func(quota int64) int64 { return quota }
	cpuPeriodFile := "/sys/fs/cgroup/cpu/cpu.cfs_period_us"
	parseCpuPeriodFile := readMaxStringOrInt64
	scaleCpuPeriod := func(period int64) int64 { return period }
	cpuSharesFile := "/sys/fs/cgroup/cpu/cpu.shares"
	parseCpuSharesFile := readMaxStringOrInt64
	scaleCpuShares := func(shares int64) int64 { return shares }
	cgroupV2 := cgroups.IsCgroup2UnifiedMode()
	cgroupVersion := 1
	if cgroupV2 {
		// start here in case we are unprivileged
		cgroupVersion = 2
		memoryLimitFile = "/sys/fs/cgroup/memory.max"
		parseMemoryLimitFile = readMaxStringOrInt64
		cpuQuotaFile = "/sys/fs/cgroup/cpu.max"
		parseCpuQuotaFile = readMaxStringsOrInt64s(0)
		scaleCpuQuota = func(quota int64) int64 { return quota * 10 }
		cpuPeriodFile = "/sys/fs/cgroup/cpu/cpu.max"
		parseCpuPeriodFile = readMaxStringsOrInt64s(1)
		scaleCpuPeriod = func(period int64) int64 { return period * 10 }
		cpuSharesFile = "/sys/fs/cgroup/cpu/cpu.weight"
		parseCpuSharesFile = readMaxStringOrInt64
		scaleCpuShares = func(shares int64) int64 {
			// scale from [1..10000] (v2 range) to [2..0x40000] (v1 range) to convert weight to shares
			// borrowed from k8s.io/kubernetes/pkg/kubelet/cm.CpuWeightToCpuShares
			approximation := (((shares - 1) * 262142) / 9999) + 2
			switch {
			case approximation > 262144:
				return 262144
			case approximation < 2:
				return 2
			default:
				return approximation
			}
		}
		// check if we're privileged, in which case our cgroup will be a subdirectory somewhere below /sys/fs/cgroup
		cgroupFile, err := cgroups.ParseCgroupFile("/proc/self/cgroup")
		if err != nil {
			return nil, err
		}
		for k, v := range cgroupFile {
			// the empty key is a special case in /proc/self/cgroup, and we we use its value to find our specific
			// pod among all the pods running on the given host, where since we are running as privileged,
			// /sys/fs/cgroup is bind mounted from the host.
			if len(strings.TrimSpace(k)) == 0 {
				// if there's a cgroup there, assume that's where we can find settings that apply to us
				if _, err := os.Stat(filepath.Join("/sys/fs/cgroup", v, "pids.current")); err == nil {
					memoryLimitFile = filepath.Join("/sys/fs/cgroup", v, "memory.max")
					cpuQuotaFile = filepath.Join("/sys/fs/cgroup", v, "cpu.max")
					cpuPeriodFile = filepath.Join("/sys/fs/cgroup", v, "cpu.max")
					cpuSharesFile = filepath.Join("/sys/fs/cgroup", v, "cpu.weight")
				}
			}
		}
	}

	// read the contents of the three or four files
	var memoryByteLimit, cpuQuota, cpuPeriod, cpuShares int64
	var err error
	for _, filename := range []string{memoryLimitFile, cpuQuotaFile, cpuPeriodFile, cpuSharesFile} {
		if _, ok := fileContents[filename]; !ok {
			var b []byte
			b, err = ioutil.ReadFile(filename)
			if err != nil {
				goto returnError
			}
			fileContents[filename] = strings.TrimSpace(string(b))
		}
	}
	memoryByteLimit, err = parseMemoryLimitFile(fileContents[memoryLimitFile])
	if err != nil {
		goto returnError
	}
	log.V(5).Infof("using %s for cgroupv%d memory.max with value %s", memoryLimitFile, cgroupVersion, string(fileContents[memoryLimitFile]))
	cpuQuota, err = parseCpuQuotaFile(fileContents[cpuQuotaFile])
	if err != nil {
		goto returnError
	}
	cpuQuota = scaleCpuQuota(cpuQuota)
	log.V(5).Infof("using %s for cgroupv%d cpu.quota_us value %s", cpuQuotaFile, cgroupVersion, string(fileContents[cpuQuotaFile]))
	cpuPeriod, err = parseCpuPeriodFile(fileContents[cpuPeriodFile])
	if err != nil {
		goto returnError
	}
	cpuPeriod = scaleCpuPeriod(cpuPeriod)
	log.V(5).Infof("using %s for cgroupv%d cpu.cfs_period_us value %s", cpuPeriodFile, cgroupVersion, string(fileContents[cpuPeriodFile]))
	cpuShares, err = parseCpuSharesFile(fileContents[cpuSharesFile])
	if err != nil {
		goto returnError
	}
	cpuShares = scaleCpuShares(cpuShares)
	log.V(5).Infof("using %s for cgroupv%d cpu.shares value %s", cpuSharesFile, cgroupVersion, string(fileContents[cpuSharesFile]))

	// math.MaxInt64 seems to give cgroups trouble, this value is
	// still 92 terabytes, so it ought to be sufficiently large for
	// our purposes.
	if memoryByteLimit > 92233720368547 {
		memoryByteLimit = 92233720368547
	}

	if cpuPeriod > 1000000 {
		cpuPeriod = 1000000
	}

	return &s2iapi.CGroupLimits{
		// Though we are capped on memory and cpu at the cgroup parent level,
		// some build containers care what their memory limit is so they can
		// adapt, thus we need to set the memory limit at the container level
		// too, so that information is available to them.
		MemoryLimitBytes: memoryByteLimit,
		// Set memoryswap==memorylimit, this ensures no swapping occurs.
		// see: https://docs.docker.com/engine/reference/run/#runtime-constraints-on-cpu-and-memory
		MemorySwap: memoryByteLimit,
		CPUShares:  cpuShares,
		CPUQuota:   cpuQuota,
		CPUPeriod:  cpuPeriod,
	}, nil

returnError:
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
