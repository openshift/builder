// +build linux

package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/buildah/util"
	ireference "github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	docker "github.com/fsouza/go-dockerclient"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"k8s.io/kubernetes/pkg/credentialprovider"

	buildapiv1 "github.com/openshift/api/build/v1"

	"github.com/openshift/builder/pkg/build/builder/cmd/dockercfg"
	builderutil "github.com/openshift/builder/pkg/build/builder/util"
)

var (
	nodeCredentialsFile = "/var/lib/kubelet/config.json"
)

// The build controller doesn't expect the CAP_ prefix to be used in the
// entries in the list in the environment, but our runtime configuration
// expects it to be provided, so massage the values into a suitabe list.
func dropCapabilities() []string {
	var dropCapabilities []string
	if dropCaps, ok := os.LookupEnv(builderutil.DropCapabilities); ok && dropCaps != "" {
		dropCapabilities = strings.Split(os.Getenv(builderutil.DropCapabilities), ",")
		for i := range dropCapabilities {
			dropCapabilities[i] = strings.ToUpper(dropCapabilities[i])
			if !strings.HasPrefix(dropCapabilities[i], "CAP_") {
				dropCapabilities[i] = "CAP_" + dropCapabilities[i]
			}
		}
	}
	return dropCapabilities
}

// parsePullCredentials parses credentials from provided file.
func parsePullCredentials(credsPath string) (credentialprovider.DockerConfig, error) {
	var creds credentialprovider.DockerConfig
	var err error

	if filepath.Base(credsPath) == dockercfg.DockerConfigKey {
		if creds, err = credentialprovider.ReadDockercfgFile(
			[]string{filepath.Dir(credsPath)},
		); err != nil {
			return nil, err
		}
	} else {
		if creds, err = credentialprovider.ReadSpecificDockerConfigJSONFile(
			credsPath,
		); err != nil {
			return nil, err
		}
	}

	if creds == nil {
		creds = make(map[string]credentialprovider.DockerConfigEntry)
	}

	return creds, nil
}

// mergeNodeCredentials merges node credentials with credentials file provided.
func mergeNodeCredentials(credsPath string) (*credentialprovider.DockerConfigJSON, error) {
	nodeCreds, err := parsePullCredentials(nodeCredentialsFile)
	if err != nil {
		log.V(2).Infof("proceeding without node credentials: %v", err)
	}

	namespaceCreds, err := parsePullCredentials(credsPath)
	if err != nil {
		return nil, fmt.Errorf("error reading pull credentials: %v", err)
	}

	for regurl, cfg := range nodeCreds {
		if _, ok := namespaceCreds[regurl]; !ok {
			namespaceCreds[regurl] = cfg
		}
	}

	return &credentialprovider.DockerConfigJSON{
		Auths: namespaceCreds,
	}, nil
}

func pullDaemonlessImage(sc types.SystemContext, store storage.Store, imageName string, searchPaths []string, blobCacheDirectory string) error {
	log.V(2).Infof("Asked to pull fresh copy of %q.", imageName)

	if imageName == "" {
		return fmt.Errorf("unable to pull using empty image name")
	}

	_, err := alltransports.ParseImageName("docker://" + imageName)
	if err != nil {
		return fmt.Errorf("error parsing image name to pull %s: %v", "docker://"+imageName, err)
	}

	mergedCreds, err := mergeNodeCredentials(
		dockercfg.GetDockerConfigPath(searchPaths),
	)
	if err != nil {
		return err
	}

	dstFile, err := ioutil.TempFile("", "config")
	if err != nil {
		return fmt.Errorf("error creating tmp credentials file: %v", err)
	}
	defer func() {
		_ = dstFile.Close()
		if err := os.Remove(dstFile.Name()); err != nil {
			log.V(2).Infof("unable to remove tmp credentials file: %v", err)
		}
	}()

	if err := json.NewEncoder(dstFile).Encode(mergedCreds); err != nil {
		return fmt.Errorf("error encoding credentials: %v", err)
	}

	systemContext := sc
	systemContext.AuthFilePath = dstFile.Name()

	options := buildah.PullOptions{
		ReportWriter:  os.Stderr,
		Store:         store,
		SystemContext: &systemContext,
		BlobDirectory: blobCacheDirectory,
	}
	_, err = buildah.Pull(context.TODO(), "docker://"+imageName, options)
	return err
}

func daemonlessProcessLimits() (defaultProcessLimits []string) {
	rlim := unix.Rlimit{Cur: 1048576, Max: 1048576}
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &rlim); err == nil {
		defaultProcessLimits = append(defaultProcessLimits, fmt.Sprintf("nofile=%d:%d", rlim.Cur, rlim.Max))
	} else {
		if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlim); err == nil {
			defaultProcessLimits = append(defaultProcessLimits, fmt.Sprintf("nofile=%d:%d", rlim.Cur, rlim.Max))
		}
	}
	rlim = unix.Rlimit{Cur: 1048576, Max: 1048576}
	if err := unix.Setrlimit(unix.RLIMIT_NPROC, &rlim); err == nil {
		defaultProcessLimits = append(defaultProcessLimits, fmt.Sprintf("nproc=%d:%d", rlim.Cur, rlim.Max))
	} else {
		if err := unix.Getrlimit(unix.RLIMIT_NPROC, &rlim); err == nil {
			defaultProcessLimits = append(defaultProcessLimits, fmt.Sprintf("nproc=%d:%d", rlim.Cur, rlim.Max))
		}
	}
	return defaultProcessLimits
}

func buildDaemonlessImage(sc types.SystemContext, store storage.Store, isolation buildah.Isolation, contextDir string, optimization buildapiv1.ImageOptimizationPolicy, opts *docker.BuildImageOptions, blobCacheDirectory string) error {
	log.V(2).Infof("Building...")

	args := make(map[string]string)
	for _, ev := range opts.BuildArgs {
		args[ev.Name] = ev.Value
	}

	pullPolicy := buildah.PullIfMissing
	if opts.Pull {
		log.V(2).Infof("Forcing fresh pull of base image.")
		pullPolicy = buildah.PullAlways
	}

	layers := false
	switch optimization {
	case buildapiv1.ImageOptimizationSkipLayers, buildapiv1.ImageOptimizationSkipLayersAndWarn:
		layers = false
	case buildapiv1.ImageOptimizationNone:
		layers = true
	default:
		return fmt.Errorf("internal error: image optimization policy %q not fully implemented", string(optimization))
	}

	systemContext := sc
	// if credsDir, ok := os.LookupEnv("PULL_DOCKERCFG_PATH"); ok {
	// 	systemContext.AuthFilePath = filepath.Join(credsDir, "config.json")
	// }
	systemContext.AuthFilePath = "/tmp/config.json"

	for registry, ac := range opts.AuthConfigs.Configs {
		log.V(5).Infof("Setting authentication for registry %q at %q.", registry, ac.ServerAddress)
		if err := config.SetAuthentication(&systemContext, registry, ac.Username, ac.Password); err != nil {
			return err
		}
		if err := config.SetAuthentication(&systemContext, ac.ServerAddress, ac.Username, ac.Password); err != nil {
			return err
		}
	}

	var transientMounts []string
	if st, err := os.Stat("/run/secrets"); err == nil && st.IsDir() {
		// Add a bind of /run/secrets, to pass along anything that the
		// runtime mounted from the node into our /run/secrets.
		transientMounts = append(transientMounts, "/run/secrets:/run/secrets:ro,nodev,noexec,nosuid")
	}

	options := imagebuildah.BuildOptions{
		ContextDirectory: contextDir,
		PullPolicy:       pullPolicy,
		Isolation:        isolation,
		TransientMounts:  transientMounts,
		Args:             args,
		Output:           opts.Name,
		Out:              opts.OutputStream,
		Err:              opts.OutputStream,
		ReportWriter:     opts.OutputStream,
		OutputFormat:     buildah.Dockerv2ImageManifest,
		SystemContext:    &systemContext,
		NamespaceOptions: buildah.NamespaceOptions{
			{Name: string(specs.NetworkNamespace), Host: true},
		},
		CommonBuildOpts: &buildah.CommonBuildOptions{
			HTTPProxy:    true,
			Memory:       opts.Memory,
			MemorySwap:   opts.Memswap,
			CgroupParent: opts.CgroupParent,
			Ulimit:       daemonlessProcessLimits(),
		},
		Layers:                  layers,
		NoCache:                 opts.NoCache,
		RemoveIntermediateCtrs:  opts.RmTmpContainer,
		ForceRmIntermediateCtrs: true,
		BlobDirectory:           blobCacheDirectory,
		DropCapabilities:        dropCapabilities(),
	}

	_, _, err := imagebuildah.BuildDockerfiles(opts.Context, store, options, opts.Dockerfile)
	return err
}

func tagDaemonlessImage(sc types.SystemContext, store storage.Store, buildTag, pushTag string) error {
	log.V(2).Infof("Tagging local image %q with name %q.", buildTag, pushTag)

	if buildTag == "" {
		return fmt.Errorf("unable to add tag to image with empty image name")
	}
	if pushTag == "" {
		return fmt.Errorf("unable to add empty tag to image")
	}

	systemContext := sc

	_, img, err := util.FindImage(store, "", &systemContext, buildTag)
	if err != nil {
		return err
	}
	if img == nil {
		return storage.ErrImageUnknown
	}
	if err := util.AddImageNames(store, "", &systemContext, img, []string{pushTag}); err != nil {
		return err
	}
	log.V(2).Infof("Added name %q to local image.", pushTag)

	return nil
}

func removeDaemonlessImage(sc types.SystemContext, store storage.Store, buildTag string) error {
	log.V(2).Infof("Removing name %q from local image.", buildTag)

	if buildTag == "" {
		return fmt.Errorf("unable to remove image using empty image name")
	}

	systemContext := sc

	_, img, err := util.FindImage(store, "", &systemContext, buildTag)
	if err != nil {
		return err
	}
	if img == nil {
		return storage.ErrImageUnknown
	}

	filtered := make([]string, 0, len(img.Names))
	for _, name := range img.Names {
		if name != buildTag {
			filtered = append(filtered, name)
		}
	}
	if err := store.SetNames(img.ID, filtered); err != nil {
		return err
	}

	return nil
}

func pushDaemonlessImage(sc types.SystemContext, store storage.Store, imageName string, authConfig docker.AuthConfiguration, blobCacheDirectory string) (string, error) {
	log.V(2).Infof("Pushing image %q from local storage.", imageName)

	if imageName == "" {
		return "", fmt.Errorf("unable to push using empty destination image name")
	}

	dest, err := alltransports.ParseImageName("docker://" + imageName)
	if err != nil {
		return "", fmt.Errorf("error parsing destination image name %s: %v", "docker://"+imageName, err)
	}

	systemContext := sc
	systemContext.AuthFilePath = "/tmp/config.json"

	if authConfig.Username != "" && authConfig.Password != "" {
		log.V(2).Infof("Setting authentication secret for %q.", authConfig.ServerAddress)
		systemContext.DockerAuthConfig = &types.DockerAuthConfig{
			Username: authConfig.Username,
			Password: authConfig.Password,
		}
	} else {
		log.V(2).Infof("No authentication secret provided for pushing to registry.")
	}

	options := buildah.PushOptions{
		Compression:   archive.Gzip,
		ReportWriter:  os.Stdout,
		Store:         store,
		SystemContext: &systemContext,
		BlobDirectory: blobCacheDirectory,
	}

	// return the digest of the image
	_, digest, err := buildah.Push(context.TODO(), imageName, dest, options)
	logName := imageName
	if dref := dest.DockerReference(); dref != nil {
		if named, ok := dref.(ireference.Named); ok {
			if canonical, err := ireference.WithDigest(ireference.TrimNamed(named), digest); err == nil {
				logName = canonical.String()
			}
		}
	}
	log.V(0).Infof("Successfully pushed %s", logName)
	return string(digest), err
}

func inspectDaemonlessImage(sc types.SystemContext, store storage.Store, name string) (*docker.Image, error) {
	systemContext := sc

	ref, img, err := util.FindImage(store, "", &systemContext, name)
	if err != nil {
		switch errors.Cause(err) {
		case storage.ErrImageUnknown, docker.ErrNoSuchImage:
			log.V(2).Infof("Local copy of %q is not present.", name)
			return nil, docker.ErrNoSuchImage
		}
		return nil, err
	}
	if img == nil {
		return nil, docker.ErrNoSuchImage
	}

	image, err := ref.NewImage(context.TODO(), &systemContext)
	if err != nil {
		return nil, err
	}
	defer image.Close()

	size, err := image.Size()
	if err != nil {
		return nil, err
	}
	oconfig, err := image.OCIConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	var rootfs *docker.RootFS
	if len(oconfig.RootFS.DiffIDs) > 0 {
		rootfs = &docker.RootFS{
			Type: oconfig.RootFS.Type,
		}
		for _, d := range oconfig.RootFS.DiffIDs {
			rootfs.Layers = append(rootfs.Layers, d.String())
		}
	}

	exposedPorts := make(map[docker.Port]struct{})
	for port := range oconfig.Config.ExposedPorts {
		exposedPorts[docker.Port(port)] = struct{}{}
	}

	config := docker.Config{
		User:         oconfig.Config.User,
		ExposedPorts: exposedPorts,
		Env:          oconfig.Config.Env,
		Entrypoint:   oconfig.Config.Entrypoint,
		Cmd:          oconfig.Config.Cmd,
		Volumes:      oconfig.Config.Volumes,
		WorkingDir:   oconfig.Config.WorkingDir,
		Labels:       oconfig.Config.Labels,
		StopSignal:   oconfig.Config.StopSignal,
	}

	var created time.Time
	if oconfig.Created != nil {
		created = *oconfig.Created
	}

	return &docker.Image{
		ID:              img.ID,
		RepoTags:        []string{},
		Parent:          "",
		Comment:         "",
		Created:         created,
		Container:       "",
		ContainerConfig: config,
		DockerVersion:   "",
		Author:          oconfig.Author,
		Config:          &config,
		Architecture:    oconfig.Architecture,
		Size:            size,
		VirtualSize:     size,
		RepoDigests:     []string{},
		RootFS:          rootfs,
		OS:              oconfig.OS,
	}, nil
}

// daemonlessRun mimics the 'docker run --rm' CLI command well enough. It creates and
// starts a container and streams its logs. The container is removed after it terminates.
func daemonlessRun(ctx context.Context, store storage.Store, isolation buildah.Isolation, createOpts docker.CreateContainerOptions, attachOpts docker.AttachToContainerOptions, blobCacheDirectory string) error {
	if createOpts.Config == nil {
		return fmt.Errorf("error calling daemonlessRun: expected a Config")
	}
	if createOpts.HostConfig == nil {
		return fmt.Errorf("error calling daemonlessRun: expected a HostConfig")
	}

	builderOptions := buildah.BuilderOptions{
		Container: createOpts.Name,
		FromImage: createOpts.Config.Image,
		CommonBuildOpts: &buildah.CommonBuildOptions{
			HTTPProxy:    true,
			Memory:       createOpts.HostConfig.Memory,
			MemorySwap:   createOpts.HostConfig.MemorySwap,
			CgroupParent: createOpts.HostConfig.CgroupParent,
			Ulimit:       daemonlessProcessLimits(),
		},
		BlobDirectory: blobCacheDirectory,
	}

	builder, err := buildah.NewBuilder(ctx, store, builderOptions)
	if err != nil {
		return err
	}
	defer func() {
		if err := builder.Delete(); err != nil {
			log.V(0).Infof("Error deleting container %q(%s): %v", builder.Container, builder.ContainerID, err)
		}
	}()

	entrypoint := createOpts.Config.Entrypoint
	if len(entrypoint) == 0 {
		entrypoint = builder.Entrypoint()
	}
	runOptions := buildah.RunOptions{
		Isolation:        isolation,
		Entrypoint:       entrypoint,
		Cmd:              createOpts.Config.Cmd,
		Stdout:           attachOpts.OutputStream,
		Stderr:           attachOpts.ErrorStream,
		DropCapabilities: dropCapabilities(),
	}

	return builder.Run(append(entrypoint, createOpts.Config.Cmd...), runOptions)
}

// DaemonlessClient is a daemonless DockerClient-like implementation.
type DaemonlessClient struct {
	SystemContext           types.SystemContext
	Store                   storage.Store
	Isolation               buildah.Isolation
	BlobCacheDirectory      string
	ImageOptimizationPolicy buildapiv1.ImageOptimizationPolicy
	builders                map[string]*buildah.Builder
}

// GetDaemonlessClient returns a valid implemenatation of the DockerClient
// interface, or an error if the implementation couldn't be created.
func GetDaemonlessClient(systemContext types.SystemContext, store storage.Store, isolationSpec, blobCacheDirectory string, imageOptimizationPolicy buildapiv1.ImageOptimizationPolicy) (client DockerClient, err error) {
	isolation := buildah.IsolationDefault
	switch strings.ToLower(isolationSpec) {
	case "chroot":
		isolation = buildah.IsolationChroot
	case "oci":
		isolation = buildah.IsolationOCI
	case "rootless":
		isolation = buildah.IsolationOCIRootless
	case "":
	default:
		return nil, fmt.Errorf("unrecognized BUILD_ISOLATION setting %q", strings.ToLower(isolationSpec))
	}

	if blobCacheDirectory != "" {
		log.V(0).Infof("Caching blobs under %q.", blobCacheDirectory)
	}

	return &DaemonlessClient{
		SystemContext:           systemContext,
		Store:                   store,
		Isolation:               isolation,
		BlobCacheDirectory:      blobCacheDirectory,
		ImageOptimizationPolicy: imageOptimizationPolicy,
		builders:                make(map[string]*buildah.Builder),
	}, nil
}

func (d *DaemonlessClient) BuildImage(opts docker.BuildImageOptions) error {
	return buildDaemonlessImage(d.SystemContext, d.Store, d.Isolation, opts.ContextDir, d.ImageOptimizationPolicy, &opts, d.BlobCacheDirectory)
}

func (d *DaemonlessClient) PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) (string, error) {
	imageName := opts.Name
	if opts.Tag != "" {
		imageName = imageName + ":" + opts.Tag
	}
	return pushDaemonlessImage(d.SystemContext, d.Store, imageName, auth, d.BlobCacheDirectory)
}

func (d *DaemonlessClient) RemoveImage(name string) error {
	return removeDaemonlessImage(d.SystemContext, d.Store, name)
}

func (d *DaemonlessClient) CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error) {
	options := buildah.BuilderOptions{
		FromImage:     opts.Config.Image,
		Container:     opts.Name,
		BlobDirectory: d.BlobCacheDirectory,
	}
	builder, err := buildah.NewBuilder(opts.Context, d.Store, options)
	if err != nil {
		return nil, err
	}
	builder.SetCmd(opts.Config.Cmd)
	builder.SetEntrypoint(opts.Config.Entrypoint)
	if builder.Container != "" {
		d.builders[builder.Container] = builder
	}
	if builder.ContainerID != "" {
		d.builders[builder.ContainerID] = builder
	}
	return &docker.Container{ID: builder.ContainerID}, nil
}

func (d *DaemonlessClient) RemoveContainer(opts docker.RemoveContainerOptions) error {
	builder, ok := d.builders[opts.ID]
	if !ok {
		return errors.Errorf("no such container as %q", opts.ID)
	}
	name := builder.Container
	id := builder.ContainerID
	err := builder.Delete()
	if err == nil {
		if name != "" {
			if _, ok := d.builders[name]; ok {
				delete(d.builders, name)
			}
		}
		if id != "" {
			if _, ok := d.builders[id]; ok {
				delete(d.builders, id)
			}
		}
	}
	return err
}

func (d *DaemonlessClient) PullImage(opts docker.PullImageOptions, searchPaths []string) error {
	imageName := opts.Repository
	if opts.Tag != "" {
		imageName = imageName + ":" + opts.Tag
	}
	return pullDaemonlessImage(d.SystemContext, d.Store, imageName, searchPaths, d.BlobCacheDirectory)
}

func (d *DaemonlessClient) TagImage(name string, opts docker.TagImageOptions) error {
	imageName := opts.Repo
	if opts.Tag != "" {
		imageName = imageName + ":" + opts.Tag
	}
	return tagDaemonlessImage(d.SystemContext, d.Store, name, imageName)
}

func (d *DaemonlessClient) InspectImage(name string) (*docker.Image, error) {
	return inspectDaemonlessImage(d.SystemContext, d.Store, name)
}
