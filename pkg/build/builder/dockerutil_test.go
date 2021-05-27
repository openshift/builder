package builder

import (
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
)

type FakeDocker struct {
	pushImageFunc    func(opts docker.PushImageOptions, auth docker.AuthConfiguration) (string, error)
	pullImageFunc    func(opts docker.PullImageOptions, searchPaths []string) error
	buildImageFunc   func(opts docker.BuildImageOptions) error
	inspectImageFunc func(name string) (*docker.Image, error)
	removeImageFunc  func(name string) error

	buildImageCalled  bool
	pushImageCalled   bool
	removeImageCalled bool
	errPushImage      error

	callLog []methodCall
}

var _ DockerClient = &FakeDocker{}

type methodCall struct {
	methodName string
	args       []interface{}
}

func NewFakeDockerClient() *FakeDocker {
	return &FakeDocker{}
}

var fooBarRunTimes = 0

func (d *FakeDocker) BuildImage(opts docker.BuildImageOptions) error {
	if d.buildImageFunc != nil {
		return d.buildImageFunc(opts)
	}
	return nil
}
func (d *FakeDocker) PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) (string, error) {
	d.pushImageCalled = true
	if d.pushImageFunc != nil {
		return d.pushImageFunc(opts, auth)
	}
	return "", d.errPushImage
}
func (d *FakeDocker) RemoveImage(name string) error {
	if d.removeImageFunc != nil {
		return d.removeImageFunc(name)
	}
	return nil
}
func (d *FakeDocker) CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error) {
	return &docker.Container{}, nil
}
func (d *FakeDocker) PullImage(opts docker.PullImageOptions, searchPaths []string) error {
	if d.pullImageFunc != nil {
		return d.pullImageFunc(opts, searchPaths)
	}
	return nil
}
func (d *FakeDocker) RemoveContainer(opts docker.RemoveContainerOptions) error {
	return nil
}
func (d *FakeDocker) InspectImage(name string) (*docker.Image, error) {
	if d.inspectImageFunc != nil {
		return d.inspectImageFunc(name)
	}
	return &docker.Image{}, nil
}
func (d *FakeDocker) StartContainer(id string, hostConfig *docker.HostConfig) error {
	return nil
}
func (d *FakeDocker) WaitContainer(id string) (int, error) {
	return 0, nil
}
func (d *FakeDocker) AttachToContainerNonBlocking(opts docker.AttachToContainerOptions) (docker.CloseWaiter, error) {
	return nil, nil
}
func (d *FakeDocker) TagImage(name string, opts docker.TagImageOptions) error {
	d.callLog = append(d.callLog, methodCall{"TagImage", []interface{}{name, opts}})
	return nil
}

func TestTagImage(t *testing.T) {
	tests := []struct {
		old, new, newRepo, newTag string
	}{
		{"test/image", "new/image:tag", "new/image", "tag"},
		{"test/image:1.0", "new-name", "new-name", ""},
	}
	for _, tt := range tests {
		dockerClient := &FakeDocker{}
		tagImage(dockerClient, tt.old, tt.new)
		got := dockerClient.callLog
		tagOpts := docker.TagImageOptions{
			Repo:  tt.newRepo,
			Tag:   tt.newTag,
			Force: true,
		}
		want := []methodCall{
			{"TagImage", []interface{}{tt.old, tagOpts}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("dockerClient called with %#v, want %#v", got, want)
		}
	}
}

type testcase struct {
	name   string
	input  map[string]string
	expect string
	fail   bool
}

func TestCGroupParentExtraction(t *testing.T) {
	tcs := []testcase{
		{
			name: "systemd",
			input: map[string]string{
				"cpu":          "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"cpuacct":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"name=systemd": "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"net_prio":     "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"freezer":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"blkio":        "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"net_cls":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"memory":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"hugetlb":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"perf_event":   "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"cpuset":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"devices":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"pids":         "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
			},
			expect: "kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice",
		},
		{
			name: "systemd-besteffortpod",
			input: map[string]string{
				"memory": "/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod8d369e32_521b_11e7_8df4_507b9d27b5d9.slice/docker-fbf6fe5e4effd80b6a9b3318dd0e5f538b9c4ba8918c174768720c83b338a41f.scope",
			},
			expect: "kubepods-besteffort-pod8d369e32_521b_11e7_8df4_507b9d27b5d9.slice",
		},
		{
			name: "nonsystemd-burstablepod",
			input: map[string]string{
				"memory": "/kubepods/burstable/podc4ab0636-521a-11e7-8eea-0e5e65642be0/9ea9361dc31b0e18f699497a5a78a010eb7bae3f9a2d2b5d3027b37bdaa4b334",
			},
			expect: "/kubepods/burstable/podc4ab0636-521a-11e7-8eea-0e5e65642be0",
		},
		{
			name: "non-systemd",
			input: map[string]string{
				"cpu":          "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"cpuacct":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"name=systemd": "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"net_prio":     "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"freezer":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"blkio":        "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"net_cls":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"memory":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"hugetlb":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"perf_event":   "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"cpuset":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"devices":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"pids":         "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
			},
			expect: "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice",
		},
		{
			name: "no-memory-entry",
			input: map[string]string{
				"cpu":          "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"cpuacct":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"name=systemd": "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"net_prio":     "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"freezer":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"blkio":        "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"net_cls":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"hugetlb":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"perf_event":   "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"cpuset":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"devices":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"pids":         "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
			},
			expect: "",
			fail:   true,
		},
		{
			name: "unparseable",
			input: map[string]string{
				"memory": "kubepods.slice",
			},
			expect: "",
			fail:   true,
		},
	}

	for _, tc := range tcs {
		parent, err := extractParentFromCgroupMap(tc.input)
		if err != nil && !tc.fail {
			t.Errorf("[%s] unexpected exception: %v", tc.name, err)
		}
		if tc.fail && err == nil {
			t.Errorf("[%s] expected failure, did not get one and got cgroup parent=%s", tc.name, parent)
		}
		if parent != tc.expect {
			t.Errorf("[%s] expected cgroup parent= %s, got %s", tc.name, tc.expect, parent)
		}
	}
}

func TestReadMaxStringOrInt64(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		expectedVal int64
		expectedErr bool
	}{
		{
			name:        "bad-data",
			fileContent: "wqertwert",
			expectedErr: true,
		},
		{
			name:        "use-of-max",
			fileContent: "max",
			expectedVal: math.MaxInt64,
		},
		{
			name:        "normal-int",
			fileContent: "1234567891011",
			expectedVal: int64(1234567891011),
		},
		{
			name:        "file-missing",
			expectedErr: true,
		},
	}
	tmpDir, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("error creating tmp dir: %s", err.Error())
	}
	defer os.RemoveAll(tmpDir)
	for _, tc := range tests {
		t.Logf("running tc %s", tc.name)
		fileName := filepath.Join(tmpDir, tc.name)
		if len(tc.fileContent) > 0 {
			err = ioutil.WriteFile(fileName, []byte(tc.fileContent), 0644)
			if err != nil {
				t.Errorf("error writing data to file %s: %s", fileName, err.Error())
			}
		}
		val, err := readMaxStringOrInt64(fileName)
		if tc.expectedErr {
			if err == nil {
				t.Errorf("test %s expected error and did not get one", tc.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("test %s did not expect error and got: %s", tc.name, err.Error())
		}
		if tc.expectedVal != val {
			t.Errorf("test %s expected val %v and got %v", tc.name, tc.expectedVal, val)
		}
	}
}
