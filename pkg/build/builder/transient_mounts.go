package builder

import (
	"fmt"
	"sort"
	"strings"
)

// TransientMountOptions are options that change how a transient mount is setup
type TransientMountOptions struct {
	NoDev    bool
	NoExec   bool
	NoSuid   bool
	ReadOnly *bool
}

//TransientMount is a structure to hold the data needed to construct a transient mount
type TransientMount struct {
	Source      string
	Destination string
	Options     TransientMountOptions
}

// returns the string representation of a transient mount
// format: source:destiation:option1,option2,...
func (tm *TransientMount) String() string {
	var options []string

	if tm.Options.NoDev {
		options = append(options, "nodev")
	}
	if tm.Options.NoExec {
		options = append(options, "noexec")
	}
	if tm.Options.NoSuid {
		options = append(options, "nosuid")
	}
	if tm.Options.ReadOnly != nil {
		if *tm.Options.ReadOnly {
			options = append(options, "ro")
		} else {
			options = append(options, "rw")
		}
	}

	var mount = []string{tm.Source, tm.Destination}
	if len(options) != 0 {
		mount = append(mount, strings.Join(options, ","))
	}

	return strings.Join(mount, ":")

}

// TransientMounts is a map of transient mounts with the destination as the key
// so we can detect duplicates quickly as we append new ones
type TransientMounts map[string]TransientMount

// appends a transient mount to the map and returns an error if a duplicate
// destination is detected
func (t TransientMounts) append(mount TransientMount) error {
	// transient mounts can have multiple destinations for each source
	// but duplicate destinations are not supported
	if _, ok := t[mount.Destination]; ok {
		return fmt.Errorf("duplicate transient mount destination detected, %q already exists", mount.String())
	}

	t[mount.Destination] = mount

	return nil
}

// returns a slice of the string representation of the transient mounts in the map
func (t TransientMounts) asSlice() []string {
	var mounts []string
	for _, m := range t {
		mounts = append(mounts, m.String())
	}
	sort.Strings(mounts)
	return mounts
}

// generateTransientMounts generates all of the transient mounts and returns a slice
// of their string representations
func generateTransientMounts() ([]string, error) {
	mountsMap := make(TransientMounts)

	if err := appendRHSMMount(defaultMountStart, &mountsMap); err != nil {
		return []string{}, err
	}
	if err := appendETCPKIMount(defaultMountStart, &mountsMap); err != nil {
		return []string{}, err
	}
	if err := appendRHRepoMount(defaultMountStart, &mountsMap); err != nil {
		return []string{}, err
	}
	if err := appendCATrustMount(&mountsMap); err != nil {
		return []string{}, err
	}
	// this should always be last in case there is a collision
	if err := appendBuildVolumeMounts(&mountsMap); err != nil {
		return []string{}, err
	}

	log.V(5).Infof("transient mounts: %#v", mountsMap.asSlice())

	return mountsMap.asSlice(), nil
}
