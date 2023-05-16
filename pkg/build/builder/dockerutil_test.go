package builder

import (
	"io/ioutil"
	"math"
	"os"
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
		val, err := readMaxStringOrInt64(tc.fileContent)
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
