package main

import (
	"testing"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestMustMap(t *testing.T) {
	testCases := []struct {
		description string
		input       []specs.LinuxIDMapping
		required    []uint32
		output      []specs.LinuxIDMapping
	}{
		{
			description: "no adjustments necessary",
			input: []specs.LinuxIDMapping{
				{
					HostID:      1000,
					ContainerID: 0,
					Size:        1,
				},
				{
					HostID:      100000,
					ContainerID: 1,
					Size:        65536,
				},
			},
			required: []uint32{65534, 65535},
			output: []specs.LinuxIDMapping{
				{
					HostID:      1000,
					ContainerID: 0,
					Size:        1,
				},
				{
					HostID:      100000,
					ContainerID: 1,
					Size:        65536,
				},
			},
		},
		{
			description: "shave four",
			input: []specs.LinuxIDMapping{
				{
					HostID:      1000,
					ContainerID: 0,
					Size:        1,
				},
				{
					HostID:      100000,
					ContainerID: 1,
					Size:        65536,
				},
			},
			required: []uint32{65535, 165534, 165533, 165535},
			output: []specs.LinuxIDMapping{
				{
					HostID:      1000,
					ContainerID: 0,
					Size:        1,
				},
				{
					HostID:      100000,
					ContainerID: 1,
					Size:        65532,
				},
				{
					HostID:      165532,
					ContainerID: 65535,
					Size:        1,
				},
				{
					HostID:      165533,
					ContainerID: 165533,
					Size:        1,
				},
				{
					HostID:      165534,
					ContainerID: 165534,
					Size:        1,
				},
				{
					HostID:      165535,
					ContainerID: 165535,
					Size:        1,
				},
			},
		},
	}
	for i := range testCases {
		t.Run(testCases[i].description, func(t *testing.T) {
			output, err := mustMap(testCases[i].input, testCases[i].required...)
			assert.Nil(t, err)
			assert.Equal(t, testCases[i].output, output)
		})
	}
}
