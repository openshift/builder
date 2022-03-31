package main

import (
	"testing"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestIsNodeDefaultMapping(t *testing.T) {
	cases := []struct {
		description string
		mappings    []specs.LinuxIDMapping
		expected    bool
	}{
		{
			"node defaults",
			[]specs.LinuxIDMapping{
				{ContainerID: 0, HostID: 0, Size: 0xffffffff},
			},
			true,
		},
		{
			"typical isolated namespace",
			[]specs.LinuxIDMapping{
				{ContainerID: 0, HostID: 100100, Size: 65536},
			},
			false,
		},
		{
			"user with subuid",
			[]specs.LinuxIDMapping{
				{ContainerID: 0, HostID: 1001, Size: 1},
				{ContainerID: 1, HostID: 100100, Size: 65536},
			},
			false,
		},
		{
			"multiple ranges",
			[]specs.LinuxIDMapping{
				{ContainerID: 0, HostID: 1001, Size: 1024},
				{ContainerID: 1024, HostID: 100100, Size: 65536},
			},
			false,
		},
	}
	for i := range cases {
		t.Run(cases[i].description, func(t *testing.T) {
			assert.Equalf(t, cases[i].expected, isNodeDefaultMapping(cases[i].mappings), "isNodeDefaultMapping(%v)", cases[i].mappings)
		})
	}
}
