package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	istorage "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	restclient "k8s.io/client-go/rest"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildutil "github.com/openshift/builder/pkg/build/builder/util"
	buildscheme "github.com/openshift/client-go/build/clientset/versioned/scheme"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	"github.com/openshift/library-go/pkg/git"
	"github.com/openshift/library-go/pkg/serviceability"
	s2iapi "github.com/openshift/source-to-image/pkg/api"
	s2igit "github.com/openshift/source-to-image/pkg/scm/git"

	bld "github.com/openshift/builder/pkg/build/builder"
	"github.com/openshift/builder/pkg/build/builder/cmd/scmauth"
	"github.com/openshift/builder/pkg/build/builder/timing"
	builderutil "github.com/openshift/builder/pkg/build/builder/util"
	utillog "github.com/openshift/builder/pkg/build/builder/util/log"
	"github.com/openshift/builder/pkg/version"
)

var (
	log = utillog.ToFile(os.Stderr, 2)

	buildScheme       = runtime.NewScheme()
	buildCodecFactory = serializer.NewCodecFactory(buildscheme.Scheme)
	buildJSONCodec    runtime.Codec
)

func init() {
	buildJSONCodec = buildCodecFactory.LegacyCodec(buildapiv1.SchemeGroupVersion)
}

type builder interface {
	Build(dockerClient bld.DockerClient, sock string, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build, cgLimits *s2iapi.CGroupLimits) error
}

type builderConfig struct {
	out             io.Writer
	build           *buildapiv1.Build
	sourceSecretDir string
	dockerClient    bld.DockerClient
	dockerEndpoint  string
	buildsClient    buildclientv1.BuildInterface
	cleanup         func()
	store           storage.Store
	blobCache       string
}

func newBuilderConfigFromEnvironment(out io.Writer, needsDocker bool, isolation, ociRuntime, storageDriver, storageOptions string) (*builderConfig, error) {
	cfg := &builderConfig{}
	var err error

	cfg.out = out

	cfg.build = &buildapiv1.Build{}

	if err := buildutil.GetBuildFromEnv(cfg.build); err != nil {
		return nil, err
	}

	if log.Is(4) {
		redactedBuild := builderutil.SafeForLoggingBuild(cfg.build)
		bytes, err := runtime.Encode(buildJSONCodec, redactedBuild)
		if err != nil {
			log.V(4).Infof("unable to print debug line: %v", err)
		} else {
			log.V(4).Infof("redacted build: %v", string(bytes))
		}
	}

	// sourceSecretsDir (SOURCE_SECRET_PATH)
	cfg.sourceSecretDir = os.Getenv("SOURCE_SECRET_PATH")

	if needsDocker {
		var systemContext types.SystemContext
		if registriesConfPath, ok := os.LookupEnv("BUILD_REGISTRIES_CONF_PATH"); ok && len(registriesConfPath) > 0 {
			if _, err := os.Stat(registriesConfPath); err == nil {
				systemContext.SystemRegistriesConfPath = registriesConfPath
			}
		}
		if registriesDirPath, ok := os.LookupEnv("BUILD_REGISTRIES_DIR_PATH"); ok && len(registriesDirPath) > 0 {
			if _, err := os.Stat(registriesDirPath); err == nil {
				systemContext.RegistriesDirPath = registriesDirPath
			}
		}
		if signaturePolicyPath, ok := os.LookupEnv("BUILD_SIGNATURE_POLICY_PATH"); ok && len(signaturePolicyPath) > 0 {
			if _, err := os.Stat(signaturePolicyPath); err == nil {
				systemContext.SignaturePolicyPath = signaturePolicyPath
			}
		}

		storeOptions, err := storage.DefaultStoreOptions(false, 0)
		if err != nil {
			return nil, err
		}
		if storageDriver != "" {
			storeOptions.GraphDriverName = storageDriver
		}
		if storageOptions != "" {
			if err := json.Unmarshal([]byte(storageOptions), &storeOptions.GraphDriverOptions); err != nil {
				log.V(0).Infof("Error parsing storage options (%q): %v", storageOptions, err)
				return nil, err
			}
		}
		if storageConfPath, ok := os.LookupEnv("BUILD_STORAGE_CONF_PATH"); ok && len(storageConfPath) > 0 {
			if _, err := os.Stat(storageConfPath); err == nil {
				storage.ReloadConfigurationFile(storageConfPath, &storeOptions)
			}
		}

		store, err := storage.GetStore(storeOptions)
		cfg.store = store
		if err != nil {
			return nil, err
		}
		cfg.cleanup = func() {
			if _, err := store.Shutdown(false); err != nil {
				log.V(0).Infof("Error shutting down storage: %v", err)
			}
		}
		istorage.Transport.SetStore(store)

		// Default to using /var/cache/blobs as a blob cache, but allow its location
		// to be changed by setting $BUILD_BLOBCACHE_DIR.  Setting the location to an
		// empty value disables the cache.
		cfg.blobCache = "/var/cache/blobs"
		if blobCacheDir, isSet := os.LookupEnv("BUILD_BLOBCACHE_DIR"); isSet {
			cfg.blobCache = blobCacheDir
		}

		imageOptimizationPolicy := buildapiv1.ImageOptimizationNone
		if s := cfg.build.Spec.Strategy.DockerStrategy; s != nil {
			// Default to possibly-multiple-layer builds for Dockerfile-based builds, unless something else was specified.
			if policy := s.ImageOptimizationPolicy; policy != nil {
				imageOptimizationPolicy = *policy
			}
		}
		if s := cfg.build.Spec.Strategy.SourceStrategy; s != nil {
			// Always use base-image+single-layer builds for S2I builds.
			imageOptimizationPolicy = buildapiv1.ImageOptimizationSkipLayers
		}

		dockerClient, err := bld.GetDaemonlessClient(systemContext, store, cfg.blobCache, isolation, ociRuntime, imageOptimizationPolicy)
		if err != nil {
			return nil, fmt.Errorf("no daemonless store: %v", err)
		}
		cfg.dockerClient = dockerClient

		// S2I requires this to be set, even though we aren't going to use
		// docker because we're just generating a dockerfile.
		// TODO: update the validation in s2i to be smarter and then
		// remove this.
		cfg.dockerEndpoint = "n/a"
	}

	// buildsClient (KUBERNETES_SERVICE_HOST, KUBERNETES_SERVICE_PORT)
	clientConfig, err := restclient.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to the server: %v", err)
	}
	buildsClient, err := buildclientv1.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %v", err)
	}
	cfg.buildsClient = buildsClient.Builds(cfg.build.Namespace)

	return cfg, nil
}

// setupGitEnvironment configures the context for git commands to run. Returns
// the following:
//
// 1. Path to the source secrets directory
// 2. A list of set environment variables for git
// 3. The path to the context's .gitconfig file if set, and
// 4. An error if raised
func (c *builderConfig) setupGitEnvironment() (string, []string, string, error) {

	// For now, we only handle git. If not specified, we're done
	gitSource := c.build.Spec.Source.Git
	if gitSource == nil {
		return "", []string{}, "", nil
	}

	sourceSecret := c.build.Spec.Source.SourceSecret
	gitEnv := []string{"GIT_ASKPASS=true"}
	var outGitConfigFile string
	// If a source secret is present, set it up and add its environment variables
	if sourceSecret != nil {
		// TODO: this should be refactored to let each source type manage which secrets
		// it accepts
		sourceURL, err := s2igit.Parse(gitSource.URI)
		if err != nil {
			return "", nil, "", fmt.Errorf("cannot parse build URL: %s", gitSource.URI)
		}
		scmAuths := scmauth.GitAuths(sourceURL)

		secretsEnv, overrideURL, gitConfigFile, err := scmAuths.Setup(c.sourceSecretDir)
		outGitConfigFile = gitConfigFile
		if err != nil {
			return c.sourceSecretDir, nil, outGitConfigFile, fmt.Errorf("cannot setup source secret: %v", err)
		}
		if overrideURL != nil {
			gitSource.URI = overrideURL.String()
		}
		gitEnv = append(gitEnv, secretsEnv...)
	}
	// Bug 1875639: git commands fail if HTTP_PROXY and HTTPS_PROXY are set alongside
	// NO_PROXY
	if gitSource.NoProxy != nil && len(*gitSource.NoProxy) > 0 {
		gitEnv = append(gitEnv, fmt.Sprintf("NO_PROXY=%s", *gitSource.NoProxy))
		gitEnv = append(gitEnv, fmt.Sprintf("no_proxy=%s", *gitSource.NoProxy))
	}
	return c.sourceSecretDir, bld.MergeEnv(os.Environ(), gitEnv), outGitConfigFile, nil
}

// setupProxyConfig sets up a git proxy configuration with the provided git client, and directory
// containing a local `.gitconfig` file if this directory contains a source secret.
//
// This is a work-around for Bug 1875639 - see https://bugzilla.redhat.com/show_bug.cgi?id=1875639
func (c *builderConfig) setupProxyConfig(gitClient git.Repository, gitConfigFile string) error {
	gitSource := c.build.Spec.Source.Git
	if gitSource == nil {
		return nil
	}
	if gitSource.HTTPProxy != nil && len(*gitSource.HTTPProxy) > 0 {
		if err := c.addGitConfig(gitClient, gitConfigFile, "http.proxy", *gitSource.HTTPProxy); err != nil {
			return err
		}
	}
	if gitSource.HTTPSProxy != nil && len(*gitSource.HTTPSProxy) > 0 {
		if err := c.addGitConfig(gitClient, gitConfigFile, "https.proxy", *gitSource.HTTPSProxy); err != nil {
			return err
		}
	}
	return nil
}

func (c *builderConfig) addGitConfig(gitClient git.Repository, gitConfigFile string, parameter string, value string) error {
	if len(gitConfigFile) == 0 {
		log.V(5).Infof("Adding parameter %s to global .gitconfig", parameter)
		return gitClient.AddGlobalConfig(parameter, value)
	}
	log.V(5).Infof("Adding parameter %s to .gitconfig %s", parameter, gitConfigFile)
	return gitClient.AddConfig("", parameter, value)
}

// clone is responsible for cloning the source referenced in the buildconfig
func (c *builderConfig) clone() error {
	ctx := timing.NewContext(context.Background())
	var sourceRev *buildapiv1.SourceRevision
	defer func() {
		c.build.Status.Stages = timing.GetStages(ctx)
		bld.HandleBuildStatusUpdate(c.build, c.buildsClient, sourceRev)
	}()
	secretTmpDir, gitEnv, gitConfigFile, err := c.setupGitEnvironment()
	if err != nil {
		return err
	}
	defer os.RemoveAll(secretTmpDir)

	gitClient := git.NewRepositoryWithEnv(gitEnv)
	if err = c.setupProxyConfig(gitClient, gitConfigFile); err != nil {
		return err
	}

	buildDir := bld.InputContentPath
	sourceInfo, err := bld.GitClone(ctx, gitClient, c.build.Spec.Source.Git, c.build.Spec.Revision, buildDir)
	if err != nil {
		c.build.Status.Phase = buildapiv1.BuildPhaseFailed
		c.build.Status.Reason = buildapiv1.StatusReasonFetchSourceFailed
		c.build.Status.Message = builderutil.StatusMessageFetchSourceFailed
		return err
	}

	if sourceInfo != nil {
		sourceRev = bld.GetSourceRevision(c.build, sourceInfo)
	}

	err = bld.ExtractInputBinary(os.Stdin, c.build.Spec.Source.Binary, buildDir)
	if err != nil {
		c.build.Status.Phase = buildapiv1.BuildPhaseFailed
		c.build.Status.Reason = buildapiv1.StatusReasonFetchSourceFailed
		c.build.Status.Message = builderutil.StatusMessageFetchSourceFailed
		return err
	}

	if len(c.build.Spec.Source.ContextDir) > 0 {
		if _, err := os.Stat(filepath.Join(buildDir, c.build.Spec.Source.ContextDir)); os.IsNotExist(err) {
			err = fmt.Errorf("provided context directory does not exist: %s", c.build.Spec.Source.ContextDir)
			c.build.Status.Phase = buildapiv1.BuildPhaseFailed
			c.build.Status.Reason = buildapiv1.StatusReasonInvalidContextDirectory
			c.build.Status.Message = builderutil.StatusMessageInvalidContextDirectory
			return err
		}
	}

	return nil
}

func (c *builderConfig) extractImageContent() error {
	ctx := timing.NewContext(context.Background())
	defer func() {
		c.build.Status.Stages = timing.GetStages(ctx)
		bld.HandleBuildStatusUpdate(c.build, c.buildsClient, nil)
	}()

	buildDir := bld.InputContentPath
	err := bld.ExtractImageContent(ctx, c.dockerClient, c.store, buildDir, c.build, c.blobCache)
	if err != nil {
		c.build.Status.Phase = buildapiv1.BuildPhaseFailed
		c.build.Status.Reason = buildapiv1.StatusReasonFetchImageContentFailed
		c.build.Status.Message = builderutil.StatusMessageFetchImageContentFailed
	}
	return err
}

// execute is responsible for running a build
func (c *builderConfig) execute(b builder) error {
	cgLimits, err := bld.GetCGroupLimits()
	if err != nil {
		return fmt.Errorf("failed to retrieve cgroup limits: %v", err)
	}
	log.V(4).Infof("Running build with cgroup limits: %#v", *cgLimits)

	if err := b.Build(c.dockerClient, c.dockerEndpoint, c.buildsClient, c.build, cgLimits); err != nil {
		return fmt.Errorf("build error: %v", err)
	}

	if c.build.Spec.Output.To == nil || len(c.build.Spec.Output.To.Name) == 0 {
		fmt.Fprintf(c.out, "Build complete, no image push requested\n")
	}

	return nil
}

type dockerBuilder struct{}

// Build starts a Docker build.
func (dockerBuilder) Build(dockerClient bld.DockerClient, sock string, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build, cgLimits *s2iapi.CGroupLimits) error {
	return bld.NewDockerBuilder(dockerClient, buildsClient, build, cgLimits).Build()
}

type s2iBuilder struct{}

// Build starts an S2I build.
func (s2iBuilder) Build(dockerClient bld.DockerClient, sock string, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build, cgLimits *s2iapi.CGroupLimits) error {
	return bld.NewS2IBuilder(dockerClient, sock, buildsClient, build, cgLimits).Build()
}

func runBuild(out io.Writer, builder builder, isolation, ociRuntime, storageDriver, storageOptions string) error {
	logVersion()
	cfg, err := newBuilderConfigFromEnvironment(out, true, isolation, ociRuntime, storageDriver, storageOptions)
	if err != nil {
		return err
	}
	if cfg.cleanup != nil {
		defer cfg.cleanup()
	}
	return cfg.execute(builder)
}

// RunDockerBuild creates a docker builder and runs its build
func RunDockerBuild(out io.Writer, isolation, ociRuntime, storageDriver, storageOptions string) error {
	serviceability.InitLogrusFromKlog()
	return runBuild(out, dockerBuilder{}, isolation, ociRuntime, storageDriver, storageOptions)
}

// RunS2IBuild creates a S2I builder and runs its build
func RunS2IBuild(out io.Writer, isolation, ociRuntime, storageDriver, storageOptions string) error {
	serviceability.InitLogrusFromKlog()
	return runBuild(out, s2iBuilder{}, isolation, ociRuntime, storageDriver, storageOptions)
}

// RunGitClone performs a git clone using the build defined in the environment
func RunGitClone(out io.Writer) error {
	serviceability.InitLogrusFromKlog()
	logVersion()
	cfg, err := newBuilderConfigFromEnvironment(out, false, "", "", "", "")
	if err != nil {
		return err
	}
	if cfg.cleanup != nil {
		defer cfg.cleanup()
	}
	return cfg.clone()
}

// RunManageDockerfile manipulates the dockerfile for docker builds.
// It will write the inline dockerfile to the working directory (possibly
// overwriting an existing dockerfile) and then update the dockerfile
// in the working directory (accounting for contextdir+dockerfilepath)
// with new FROM image information based on the imagestream/imagetrigger
// and also adds some env and label values to the dockerfile based on
// the build information.
func RunManageDockerfile(out io.Writer) error {
	serviceability.InitLogrusFromKlog()
	logVersion()
	cfg, err := newBuilderConfigFromEnvironment(out, false, "", "", "", "")
	if err != nil {
		return err
	}
	if cfg.cleanup != nil {
		defer cfg.cleanup()
	}
	return bld.ManageDockerfile(bld.InputContentPath, cfg.build)
}

// RunExtractImageContent extracts files from existing images
// into the build working directory.
func RunExtractImageContent(out io.Writer) error {
	serviceability.InitLogrusFromKlog()
	logVersion()
	cfg, err := newBuilderConfigFromEnvironment(out, true, "", "", "", "")
	if err != nil {
		return err
	}
	if cfg.cleanup != nil {
		defer cfg.cleanup()
	}
	return cfg.extractImageContent()
}

// logVersion logs the version of openshift-builder.
func logVersion() {
	log.V(5).Infof("openshift-builder %v", version.Get())
	log.V(5).Infof("Powered by buildah %s", version.BuildahVersion())
}
