package builder

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	idocker "github.com/containers/image/v5/docker"
	"github.com/docker/distribution/registry/api/errcode"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"

	"github.com/openshift/builder/pkg/build/builder/cmd/dockercfg"
)

var (
	// DefaultPushOrPullRetryCount is the number of retries of pushing or pulling the built Docker image
	// into a configured repository
	DefaultPushOrPullRetryCount = 2
	// DefaultPushOrPullRetryDelay is the time to wait before triggering a push or pull retry
	DefaultPushOrPullRetryDelay = 5 * time.Second
)

// DockerClient is an interface to the Docker client that contains
// the methods used by the common builder
type DockerClient interface {
	BuildImage(opts docker.BuildImageOptions) error
	PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) (string, error)
	RemoveImage(name string) error
	CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error)
	PullImage(opts docker.PullImageOptions, authSearchPaths []string) error
	RemoveContainer(opts docker.RemoveContainerOptions) error
	InspectImage(name string) (*docker.Image, error)
	TagImage(name string, opts docker.TagImageOptions) error
}

func unwrapUnauthorizedError(err error) error {
	cause := errors.Cause(err)
	if _, ok := cause.(idocker.ErrUnauthorizedForCredentials); ok {
		// strip off wrappers that mainly add the image name as their added context,
		// which just duplicates information that we're already logging
		return cause
	}
	return err
}

func retryImageAction(actionName string, action func() error) error {
	var err error

	for retries := 0; retries <= DefaultPushOrPullRetryCount; retries++ {
		err = action()
		if err == nil {
			return nil
		}
		log.V(0).Infof("Warning: %s failed, retrying in %s ...", actionName, DefaultPushOrPullRetryDelay)
		time.Sleep(DefaultPushOrPullRetryDelay)
	}

	if errs, ok := errors.Cause(err).(errcode.Errors); ok {
		// if this error is a group of errors, process them all in turn
		var unwrap bool
		for i := range errs {
			if registryError, ok := errs[i].(errcode.Error); ok {
				if registryError.Code == errcode.ErrorCodeUnauthorized {
					// remove any Wrapf() wrapping, since we're
					// already going to be providing context when we
					// print the error
					unwrap = true
				}
			}
		}
		if unwrap {
			err = errors.Cause(err)
		}
	}

	err = unwrapUnauthorizedError(err)

	return fmt.Errorf("After retrying %d times, %s image still failed due to error: %v", DefaultPushOrPullRetryCount, actionName, err)
}

func removeImage(client DockerClient, name string) error {
	return client.RemoveImage(name)
}

// tagImage uses the dockerClient to tag a Docker image with name. It is a
// helper to facilitate the usage of dockerClient.TagImage, because the former
// requires the name to be split into more explicit parts.
func tagImage(dockerClient DockerClient, image, name string) error {
	repo, tag := docker.ParseRepositoryTag(name)
	return dockerClient.TagImage(image, docker.TagImageOptions{
		Repo: repo,
		Tag:  tag,
		// We need to set Force to true to update the tag even if it
		// already exists. This is the same behavior as `docker build -t
		// tag .`.
		Force: true,
	})
}

// readInt64 reads a file containing a 64 bit integer value
// and returns the value as an int64.  If the file contains
// a value larger than an int64, it returns MaxInt64,
// if the value is smaller than an int64, it returns MinInt64.
func readInt64(filePath string) (int64, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return -1, err
	}
	s := strings.TrimSpace(string(data))
	val, err := strconv.ParseInt(s, 10, 64)
	// overflow errors are ok, we'll get return a math.MaxInt64 value which is more
	// than enough anyway.  For underflow we'll return MinInt64 and the error.
	if err != nil && err.(*strconv.NumError).Err == strconv.ErrRange {
		if s[0] == '-' {
			return math.MinInt64, err
		}
		return math.MaxInt64, nil
	} else if err != nil {
		return -1, err
	}
	return val, nil
}

// extractParentFromCgroupMap finds the cgroup parent in the cgroup map
func extractParentFromCgroupMap(cgMap map[string]string) (string, error) {
	memory, ok := cgMap["memory"]
	if !ok {
		return "", fmt.Errorf("could not find memory cgroup subsystem in map %v", cgMap)
	}
	log.V(6).Infof("cgroup memory subsystem value: %s", memory)

	parts := strings.Split(memory, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("unprocessable cgroup memory value: %s", memory)
	}

	var cgroupParent string
	if strings.HasSuffix(memory, ".scope") {
		// systemd system, take the second to last segment.
		cgroupParent = parts[len(parts)-2]
	} else {
		// non-systemd, take everything except the last segment.
		cgroupParent = strings.Join(parts[:len(parts)-1], "/")
	}
	log.V(5).Infof("found cgroup parent %v", cgroupParent)
	return cgroupParent, nil
}

// GetDockerAuthConfiguration provides a Docker authentication configuration when the
// PullSecret is specified.
func GetDockerAuthConfiguration(path string) (*docker.AuthConfigurations, error) {
	log.V(2).Infof("Checking for Docker config file for %s in path %s", dockercfg.PullAuthType, path)
	dockercfgPath := dockercfg.GetDockercfgFile(path)
	if len(dockercfgPath) == 0 {
		return nil, fmt.Errorf("no docker config file found in '%s'", os.Getenv(dockercfg.PullAuthType))
	}
	log.V(2).Infof("Using Docker config file %s", dockercfgPath)
	r, err := os.Open(dockercfgPath)
	if err != nil {
		return nil, fmt.Errorf("'%s': %s", dockercfgPath, err)
	}
	return docker.NewAuthConfigurations(r)
}
