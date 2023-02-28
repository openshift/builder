package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/buildah"
	cconfig "github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	docker "github.com/fsouza/go-dockerclient"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	"github.com/openshift/library-go/pkg/git"
	s2igit "github.com/openshift/source-to-image/pkg/scm/git"

	"github.com/openshift/builder/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/builder/pkg/build/builder/timing"
	builderutil "github.com/openshift/builder/pkg/build/builder/util"
)

const (
	// initialURLCheckTimeout is the initial timeout used to check the
	// source URL.  If fetching the URL exceeds the timeout, then a longer
	// timeout will be tried until the fetch either succeeds or the build
	// itself times out.
	initialURLCheckTimeout = 16 * time.Second

	// timeoutIncrementFactor is the factor to use when increasing
	// the timeout after each unsuccessful try
	timeoutIncrementFactor = 4
)

type gitAuthError string
type gitNotFoundError string

func (e gitAuthError) Error() string {
	return fmt.Sprintf("failed to fetch requested repository %q with provided credentials", string(e))
}

func (e gitNotFoundError) Error() string {
	return fmt.Sprintf("requested repository %q not found", string(e))
}

// GitClone clones the source associated with a build(if any) into the specified directory
func GitClone(ctx context.Context, gitClient GitClient, gitSource *buildapiv1.GitBuildSource, revision *buildapiv1.SourceRevision, dir string) (*git.SourceInfo, error) {

	// It is possible for the initcontainer to get restarted, thus we must wipe out the directory if it already exists.
	err := os.RemoveAll(dir)
	if err != nil {
		return nil, err
	}
	os.MkdirAll(dir, 0777)

	hasGitSource, err := extractGitSource(ctx, gitClient, gitSource, revision, dir, initialURLCheckTimeout)

	if err != nil {
		return nil, err
	}

	var sourceInfo *git.SourceInfo
	if hasGitSource {
		var errs []error
		sourceInfo, errs = gitClient.GetInfo(dir)
		if len(errs) > 0 {
			for _, e := range errs {
				log.V(0).Infof("error: Unable to retrieve Git info: %v", e)
			}
		}
		if sourceInfo != nil {
			sourceInfoJson, err := json.Marshal(*sourceInfo)
			if err != nil {
				log.V(0).Infof("error: Unable to serialized git source info: %v", err)
				return sourceInfo, nil
			}
			err = ioutil.WriteFile(filepath.Join(buildWorkDirMount, "sourceinfo.json"), sourceInfoJson, 0644)
			if err != nil {
				log.V(0).Infof("error: Unable to serialized git source info: %v", err)
				return sourceInfo, nil
			}
		}
	}
	return sourceInfo, nil
}

// ManageDockerfile manipulates the dockerfile for docker builds.
// It will write the inline dockerfile to the working directory (possibly
// overwriting an existing dockerfile) and then update the dockerfile
// in the working directory (accounting for contextdir+dockerfilepath)
// with new FROM image information based on the imagestream/imagetrigger
// and also adds some env and label values to the dockerfile based on
// the build information.
func ManageDockerfile(dir string, build *buildapiv1.Build) error {
	ctx := timing.NewContext(context.Background())
	defer func() {
		build.Status.Stages = timing.GetStages(ctx)
		clientConfig, err := restclient.InClusterConfig()
		if err != nil {
			return
		}
		buildsClient, err := buildclientv1.NewForConfig(clientConfig)
		if err != nil {
			return
		}
		HandleBuildStatusUpdate(build, buildsClient.Builds(build.Namespace), nil)
	}()
	os.MkdirAll(dir, 0777)
	log.V(5).Infof("Checking for presence of a Dockerfile")
	// a Dockerfile has been specified, create or overwrite into the destination
	if dockerfileSource := build.Spec.Source.Dockerfile; dockerfileSource != nil {
		baseDir := dir
		if len(build.Spec.Source.ContextDir) != 0 {
			baseDir = filepath.Join(baseDir, build.Spec.Source.ContextDir)
		}
		if err := ioutil.WriteFile(filepath.Join(baseDir, "Dockerfile"), []byte(*dockerfileSource), 0660); err != nil {
			build.Status.Phase = buildapiv1.BuildPhaseFailed
			build.Status.Reason = buildapiv1.StatusReasonManageDockerfileFailed
			build.Status.Message = builderutil.StatusMessageManageDockerfileFailed
			return err
		}
	}

	// We only mutate the dockerfile if this is a docker strategy build, otherwise
	// we leave it as it was provided.
	if build.Spec.Strategy.DockerStrategy != nil {
		sourceInfo, err := readSourceInfo()
		if err != nil {
			build.Status.Phase = buildapiv1.BuildPhaseFailed
			build.Status.Reason = buildapiv1.StatusReasonManageDockerfileFailed
			build.Status.Message = builderutil.StatusMessageManageDockerfileFailed
			return fmt.Errorf("error reading git source info: %v", err)
		}
		err = addBuildParameters(dir, build, sourceInfo)
		if err != nil {
			build.Status.Phase = buildapiv1.BuildPhaseFailed
			build.Status.Reason = buildapiv1.StatusReasonManageDockerfileFailed
			build.Status.Message = builderutil.StatusMessageManageDockerfileFailed
		}
		return err
	}
	return nil
}

func ExtractImageContent(ctx context.Context, dockerClient DockerClient, store storage.Store, dir string, build *buildapiv1.Build, blobCacheDirectory string) error {
	os.MkdirAll(dir, 0777)
	forcePull := false
	switch {
	case build.Spec.Strategy.SourceStrategy != nil:
		forcePull = build.Spec.Strategy.SourceStrategy.ForcePull
	case build.Spec.Strategy.DockerStrategy != nil:
		forcePull = build.Spec.Strategy.DockerStrategy.ForcePull
	case build.Spec.Strategy.CustomStrategy != nil:
		forcePull = build.Spec.Strategy.CustomStrategy.ForcePull
	}
	// extract source from an Image if specified
	for i, image := range build.Spec.Source.Images {
		if len(image.Paths) == 0 {
			continue
		}
		imageSecretIndex := i
		if image.PullSecret == nil {
			imageSecretIndex = -1
		}
		err := extractSourceFromImage(ctx, dockerClient, store, image.From.Name, dir, imageSecretIndex, image.Paths, forcePull, blobCacheDirectory)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkRemoteGit validates the specified Git URL. It returns GitNotFoundError
// when the remote repository not found and GitAuthenticationError when the
// remote repository failed to authenticate.
// Since this is calling the 'git' binary, the proxy settings should be
// available for this command.
func checkRemoteGit(gitClient GitClient, url string, initialTimeout time.Duration) error {

	var (
		out    string
		errOut string
		err    error
	)

	timeout := initialTimeout
	for {
		log.V(4).Infof("git ls-remote --heads %s", url)
		out, errOut, err = gitClient.TimedListRemote(timeout, url, "--heads")
		if len(out) != 0 {
			log.V(4).Infof(out)
		}
		if len(errOut) != 0 {
			log.V(4).Infof(errOut)
		}
		if err != nil {
			if _, ok := err.(*git.TimeoutError); ok {
				timeout = timeout * timeoutIncrementFactor
				log.Infof("WARNING: timed out waiting for git server, will wait %s", timeout)
				continue
			}
		}
		break
	}
	if err != nil {
		combinedOut := out + errOut
		switch {
		case strings.Contains(combinedOut, "Authentication failed"):
			return gitAuthError(url)
		case strings.Contains(combinedOut, "not found"):
			return gitNotFoundError(url)
		}
	}
	return err
}

// checkSourceURI performs a check on the URI associated with the build
// to make sure that it is valid.
func checkSourceURI(gitClient GitClient, rawurl string, timeout time.Duration) error {
	_, err := s2igit.Parse(rawurl)
	if err != nil {
		return fmt.Errorf("Invalid git source url %q: %v", rawurl, err)
	}
	return checkRemoteGit(gitClient, rawurl, timeout)
}

// ExtractInputBinary processes the provided input stream as directed by BinaryBuildSource
// into dir.
func ExtractInputBinary(in io.Reader, source *buildapiv1.BinaryBuildSource, dir string) error {
	os.MkdirAll(dir, 0777)
	if source == nil {
		return nil
	}

	var path string
	if len(source.AsFile) > 0 {
		log.V(0).Infof("Receiving source from STDIN as file %s", source.AsFile)
		path = filepath.Join(dir, source.AsFile)

		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0664)
		if err != nil {
			return err
		}
		defer f.Close()
		n, err := io.Copy(f, os.Stdin)
		if err != nil {
			return err
		}
		log.V(4).Infof("Received %d bytes into %s", n, path)
		return nil
	}

	log.V(0).Infof("Receiving source from STDIN as archive ...")

	args := []string{"-x", "-o", "-m", "-f", "-", "-C", dir}
	if log.Is(6) {
		args = append(args, "-v")
	}

	cmd := exec.Command("bsdtar", args...)
	cmd.Stdin = in
	out, err := cmd.CombinedOutput()
	log.V(4).Infof("Extracting...\n%s", string(out))
	if err != nil {
		return fmt.Errorf("unable to extract binary build input, must be a zip, tar, or gzipped tar, or specified as a file: %v", err)
	}

	return nil
}

func extractGitSource(ctx context.Context, gitClient GitClient, gitSource *buildapiv1.GitBuildSource, revision *buildapiv1.SourceRevision, dir string, timeout time.Duration) (bool, error) {
	if gitSource == nil {
		return false, nil
	}

	log.V(0).Infof("Cloning %q ...", gitSource.URI)

	// Check source URI by trying to connect to the server
	if err := checkSourceURI(gitClient, gitSource.URI, timeout); err != nil {
		return true, err
	}

	cloneOptions := []string{}
	usingRevision := revision != nil && revision.Git != nil && len(revision.Git.Commit) != 0
	usingRef := len(gitSource.Ref) != 0 || usingRevision

	// check if we specify a commit, ref, or branch to check out
	// Recursive clone if we're not going to checkout a ref and submodule update later
	if !usingRef {
		cloneOptions = append(cloneOptions, "--recursive")
		cloneOptions = append(cloneOptions, git.Shallow)
	}

	log.V(3).Infof("Cloning source from %s", gitSource.URI)

	// Only use the quiet flag if Verbosity is not 5 or greater
	if !log.Is(5) {
		cloneOptions = append(cloneOptions, "--quiet")
	}
	startTime := metav1.Now()
	if err := gitClient.CloneWithOptions(dir, gitSource.URI, cloneOptions...); err != nil {
		return true, err
	}

	timing.RecordNewStep(ctx, buildapiv1.StageFetchInputs, buildapiv1.StepFetchGitSource, startTime, metav1.Now())

	// if we specify a commit, ref, or branch to checkout, do so, and update submodules
	if usingRef {
		commit := gitSource.Ref

		if usingRevision {
			commit = revision.Git.Commit
		}

		if err := gitClient.Checkout(dir, commit); err != nil {
			err = gitClient.PotentialPRRetryAsFetch(dir, gitSource.URI, commit, err)
			if err != nil {
				return true, err
			}
		}

		// Recursively update --init
		if err := gitClient.SubmoduleUpdate(dir, true, true); err != nil {
			return true, err
		}
	}

	if information, gitErr := gitClient.GetInfo(dir); len(gitErr) == 0 {
		log.Infof("\tCommit:\t%s (%s)\n", information.CommitID, information.Message)
		log.Infof("\tAuthor:\t%s <%s>\n", information.AuthorName, information.AuthorEmail)
		log.Infof("\tDate:\t%s\n", information.Date)
	}

	return true, nil
}

func copyImageSourceFromFilesytem(sourceDir, destDir string) error {
	// Setup destination directory
	fi, err := os.Stat(destDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		log.V(4).Infof("Creating image destination directory: %s", destDir)
		err := os.MkdirAll(destDir, 0755)
		if err != nil {
			return err
		}
	} else {
		if !fi.IsDir() {
			return fmt.Errorf("destination %s must be a directory", destDir)
		}
	}

	args := []string{"-r"}
	if log.Is(5) {
		args = append(args, "-v")
	}
	args = append(args, sourceDir, destDir)
	out, err := exec.Command("cp", args...).CombinedOutput()
	log.V(4).Infof("copying image content: %s", string(out))
	if err != nil {
		return err
	}
	return nil
}

func extractSourceFromImage(ctx context.Context, dockerClient DockerClient, store storage.Store, image, buildDir string, imageSecretIndex int, paths []buildapiv1.ImageSourcePath, forcePull bool, blobCacheDirectory string) error {
	log.V(4).Infof("Extracting image source from image %s", image)

	pullPolicy := buildah.PullIfMissing
	if forcePull {
		pullPolicy = buildah.PullAlways
	}

	/*
		storeOptions := storage.DefaultStoreOptions
		storeOptions.GraphDriverName = "overlay"
		store, err := storage.GetStore(storeOptions)
		if err != nil {
			return err
		}
	*/

	auths, err := GetDockerAuthConfiguration(nodeCredentialsFile)
	if err != nil {
		klog.V(2).Infof("proceeding without node credentials: %v", err)
		auths = &docker.AuthConfigurations{
			Configs: map[string]docker.AuthConfiguration{},
		}
	}

	if imageSecretIndex != -1 {
		pullSecretPath := os.Getenv(fmt.Sprintf("%s%d", dockercfg.PullSourceAuthType, imageSecretIndex))
		if len(pullSecretPath) > 0 {
			secretAuths, err := GetDockerAuthConfiguration(pullSecretPath)
			if err != nil {
				return fmt.Errorf("error reading docker auth configuration: %v", err)
			}

			for reg, auth := range secretAuths.Configs {
				auths.Configs[reg] = auth
			}
		}
	}

	var systemContext types.SystemContext
	systemContext.AuthFilePath = "/tmp/config.json"

	for registry, ac := range auths.Configs {
		normalizedRegistry := normalizeRegistryLocation(registry)
		log.V(5).Infof("Setting authentication for registry %q (originally %q) at %q.", normalizedRegistry, registry, ac.ServerAddress)
		if err := config.SetAuthentication(&systemContext, registry, ac.Username, ac.Password); err != nil {
			return err
		}
		if normalizedServerAddress := normalizeRegistryLocation(ac.ServerAddress); normalizedServerAddress != normalizedRegistry {
			if err := config.SetAuthentication(&systemContext, normalizedServerAddress, ac.Username, ac.Password); err != nil {
				return err
			}
		}
	}

	defaultContainerConfig, err := cconfig.Default()
	if err != nil {
		return err
	}
	capabilities, err := defaultContainerConfig.Capabilities("", nil, dropCapabilities())
	if err != nil {
		return err
	}

	builderOptions := buildah.BuilderOptions{
		FromImage:     image,
		PullPolicy:    pullPolicy,
		ReportWriter:  os.Stdout,
		SystemContext: &systemContext,
		Capabilities:  capabilities,
		CommonBuildOpts: &buildah.CommonBuildOptions{
			HTTPProxy: true,
		},
		MaxPullRetries: DefaultPushOrPullRetryCount,
		PullRetryDelay: DefaultPushOrPullRetryDelay,
	}

	builder, err := buildah.NewBuilder(ctx, store, builderOptions)
	if err != nil {
		return fmt.Errorf("error creating buildah builder: %v", err)
	}

	mountPath, err := builder.Mount("")
	defer func() {
		err := builder.Unmount()
		if err != nil {
			klog.Errorf("failed to unmount: %v", err)
		}
	}()
	if err != nil {
		return fmt.Errorf("error mounting image content from image %s: %v", image, err)
	}

	for _, path := range paths {
		destPath := filepath.Join(buildDir, path.DestinationDir)
		// Paths ending with "/." are truncated by filepath.Join
		// Add it back to preserve copy behavior per docs:
		// https://docs.okd.io/latest/dev_guide/builds/build_inputs.html#image-source
		sourcePath := filepath.Join(mountPath, path.SourcePath)
		if strings.HasSuffix(path.SourcePath, "/.") {
			sourcePath = sourcePath + "/."
		}
		log.V(4).Infof("Extracting path %s from image %s to %s", path.SourcePath, image, path.DestinationDir)
		err := copyImageSourceFromFilesytem(sourcePath, destPath)
		if err != nil {
			return fmt.Errorf("error copying source path %s to %s: %v", path.SourcePath, path.DestinationDir, err)
		}
	}

	return nil
}
