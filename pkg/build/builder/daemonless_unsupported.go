//go:build !linux
// +build !linux

package builder

import (
	"context"

	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"

	buildapiv1 "github.com/openshift/api/build/v1"
)

type Isolation struct{}
type DummyStore struct{}

type DaemonlessClient struct {
	Isolation     Isolation
	Store         storage.Store
	SystemContext types.SystemContext
}

func (d *DaemonlessClient) BuildImage(opts docker.BuildImageOptions) error {
	return errors.New("building images not supported on this platform")
}
func (d *DaemonlessClient) PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
	return errors.New("pushing images not supported on this platform")
}
func (d *DaemonlessClient) RemoveImage(name string) error {
	return errors.New("removing images not supported on this platform")
}
func (d *DaemonlessClient) PullImage(opts docker.PullImageOptions, searchPaths []string) error {
	return errors.New("pulling images not supported on this platform")
}
func (d *DaemonlessClient) RemoveContainer(opts docker.RemoveContainerOptions) error {
	return errors.New("removing containers not supported on this platform")
}
func (d *DaemonlessClient) InspectImage(name string) (*docker.Image, error) {
	return nil, errors.New("inspecting images not supported on this platform")
}
func (d *DaemonlessClient) TagImage(name string, opts docker.TagImageOptions) error {
	return errors.New("tagging images not supported on this platform")
}
func daemonlessRun(ctx context.Context, store storage.Store, isolation Isolation, createOpts docker.CreateContainerOptions, attachOpts docker.AttachToContainerOptions) error {
	return errors.New("running containers not supported on this platform")
}
func buildDaemonlessImage(sc types.SystemContext, store storage.Store, isolation Isolation, dir string, optimization buildapiv1.ImageOptimizationPolicy, opts *docker.BuildImageOptions) error {
	return errors.New("running building images not supported on this platform")
}

// GetDaemonlessClient returns an error.
func GetDaemonlessClient(systemContext types.SystemContext, store storage.Store, isolationSpec, blobCacheDirectory string, imageOptimizationPolicy buildapiv1.ImageOptimizationPolicy) (client DockerClient, err error) {
	return nil, errors.New("building images without an engine not supported on this platform")
}
