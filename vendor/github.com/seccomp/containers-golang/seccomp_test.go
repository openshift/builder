// +build seccomp

// SPDX-License-Identifier: Apache-2.0

// Copyright 2013-2018 Docker, Inc.

package seccomp // import "github.com/seccomp/containers-golang"

import (
	"io/ioutil"
	"testing"

	"github.com/opencontainers/runtime-tools/generate"
)

func TestLoadProfile(t *testing.T) {
	f, err := ioutil.ReadFile("fixtures/example.json")
	if err != nil {
		t.Fatal(err)
	}
	g, err := generate.New("linux")
	if err != nil {
		t.Fatal(err)
	}
	rs := g.Spec()
	if _, err := LoadProfile(string(f), rs); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaultProfile(t *testing.T) {
	f, err := ioutil.ReadFile("seccomp.json")
	if err != nil {
		t.Fatal(err)
	}
	g, err := generate.New("linux")
	if err != nil {
		t.Fatal(err)
	}
	rs := g.Spec()
	if _, err := LoadProfile(string(f), rs); err != nil {
		t.Fatal(err)
	}
}
