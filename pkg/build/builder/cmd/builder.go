package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	restclient "k8s.io/client-go/rest"

	istorage "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"

	buildapiv1 "github.com/openshift/api/build/v1"
	bld "github.com/openshift/builder/pkg/build/builder"
	"github.com/openshift/builder/pkg/build/builder/cmd/scmauth"
	"github.com/openshift/builder/pkg/build/builder/timing"
	builderutil "github.com/openshift/builder/pkg/build/builder/util"
	utillog "github.com/openshift/builder/pkg/build/builder/util/log"
	"github.com/openshift/builder/pkg/version"
	buildscheme "github.com/openshift/client-go/build/clientset/versioned/scheme"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	"github.com/openshift/library-go/pkg/git"
	"github.com/openshift/library-go/pkg/serviceability"
	s2iapi "github.com/openshift/source-to-image/pkg/api"
	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
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

func newBuilderConfigFromEnvironment(out io.Writer, needsDocker bool) (*builderConfig, error) {
	cfg := &builderConfig{}
	var err error

	cfg.out = out

	buildStr := os.Getenv("BUILD")

	cfg.build = &buildapiv1.Build{}

	obj, _, err := buildJSONCodec.Decode([]byte(buildStr), nil, cfg.build)
	if err != nil {
		return nil, fmt.Errorf("unable to parse build string: %v", err)
	}
	ok := false
	cfg.build, ok = obj.(*buildapiv1.Build)
	if !ok {
		return nil, fmt.Errorf("build string %s is not a build: %#v", buildStr, obj)
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
		if driver, ok := os.LookupEnv("BUILD_STORAGE_DRIVER"); ok {
			storeOptions.GraphDriverName = driver
		}
		if storageOptions, ok := os.LookupEnv("BUILD_STORAGE_OPTIONS"); ok {
			if err := json.Unmarshal([]byte(storageOptions), &storeOptions.GraphDriverOptions); err != nil {
				log.V(0).Infof("Error parsing BUILD_STORAGE_OPTIONS (%q): %v", storageOptions, err)
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

		dockerClient, err := bld.GetDaemonlessClient(systemContext, store, os.Getenv("BUILD_ISOLATION"), cfg.blobCache, imageOptimizationPolicy)
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

func (c *builderConfig) setupGitEnvironment() (string, []string, error) {

	// For now, we only handle git. If not specified, we're done
	gitSource := c.build.Spec.Source.Git
	if gitSource == nil {
		return "", []string{}, nil
	}

	sourceSecret := c.build.Spec.Source.SourceSecret
	gitEnv := []string{"GIT_ASKPASS=true"}
	// If a source secret is present, set it up and add its environment variables
	if sourceSecret != nil {
		// TODO: this should be refactored to let each source type manage which secrets
		// it accepts
		sourceURL, err := s2igit.Parse(gitSource.URI)
		if err != nil {
			return "", nil, fmt.Errorf("cannot parse build URL: %s", gitSource.URI)
		}
		scmAuths := scmauth.GitAuths(sourceURL)

		secretsEnv, overrideURL, err := scmAuths.Setup(c.sourceSecretDir)
		if err != nil {
			return c.sourceSecretDir, nil, fmt.Errorf("cannot setup source secret: %v", err)
		}
		if overrideURL != nil {
			gitSource.URI = overrideURL.String()
		}
		gitEnv = append(gitEnv, secretsEnv...)
	}
	if gitSource.HTTPProxy != nil && len(*gitSource.HTTPProxy) > 0 {
		gitEnv = append(gitEnv, fmt.Sprintf("HTTP_PROXY=%s", *gitSource.HTTPProxy))
		gitEnv = append(gitEnv, fmt.Sprintf("http_proxy=%s", *gitSource.HTTPProxy))
	}
	if gitSource.HTTPSProxy != nil && len(*gitSource.HTTPSProxy) > 0 {
		gitEnv = append(gitEnv, fmt.Sprintf("HTTPS_PROXY=%s", *gitSource.HTTPSProxy))
		gitEnv = append(gitEnv, fmt.Sprintf("https_proxy=%s", *gitSource.HTTPSProxy))
	}
	if gitSource.NoProxy != nil && len(*gitSource.NoProxy) > 0 {
		gitEnv = append(gitEnv, fmt.Sprintf("NO_PROXY=%s", *gitSource.NoProxy))
		gitEnv = append(gitEnv, fmt.Sprintf("no_proxy=%s", *gitSource.NoProxy))
	}
	return c.sourceSecretDir, bld.MergeEnv(os.Environ(), gitEnv), nil
}

// clone is responsible for cloning the source referenced in the buildconfig
func (c *builderConfig) clone() error {
	ctx := timing.NewContext(context.Background())
	var sourceRev *buildapiv1.SourceRevision
	defer func() {
		c.build.Status.Stages = timing.GetStages(ctx)
		bld.HandleBuildStatusUpdate(c.build, c.buildsClient, sourceRev)
	}()
	secretTmpDir, gitEnv, err := c.setupGitEnvironment()
	if err != nil {
		return err
	}
	defer os.RemoveAll(secretTmpDir)

	gitClient := git.NewRepositoryWithEnv(gitEnv)

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
	return bld.ExtractImageContent(ctx, c.dockerClient, c.store, buildDir, c.build, c.blobCache)
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

func runBuild(out io.Writer, builder builder) error {
	logVersion()
	cfg, err := newBuilderConfigFromEnvironment(out, true)
	if err != nil {
		return err
	}
	if cfg.cleanup != nil {
		defer cfg.cleanup()
	}
	return cfg.execute(builder)
}

// RunDockerBuild creates a docker builder and runs its build
func RunDockerBuild(out io.Writer) error {
	switch {
	case log.Is(6):
		serviceability.InitLogrus("DEBUG")
	case log.Is(2):
		serviceability.InitLogrus("INFO")
	case log.Is(0):
		serviceability.InitLogrus("WARN")
	}
	return runBuild(out, dockerBuilder{})
}

// RunS2IBuild creates a S2I builder and runs its build
func RunS2IBuild(out io.Writer) error {
	switch {
	case log.Is(6):
		serviceability.InitLogrus("DEBUG")
	case log.Is(2):
		serviceability.InitLogrus("INFO")
	case log.Is(0):
		serviceability.InitLogrus("WARN")
	}
	return runBuild(out, s2iBuilder{})
}

// RunGitClone performs a git clone using the build defined in the environment
func RunGitClone(out io.Writer) error {
	switch {
	case log.Is(6):
		serviceability.InitLogrus("DEBUG")
	case log.Is(2):
		serviceability.InitLogrus("INFO")
	case log.Is(0):
		serviceability.InitLogrus("WARN")
	}
	logVersion()
	cfg, err := newBuilderConfigFromEnvironment(out, false)
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
	switch {
	case log.Is(6):
		serviceability.InitLogrus("DEBUG")
	case log.Is(2):
		serviceability.InitLogrus("INFO")
	case log.Is(0):
		serviceability.InitLogrus("WARN")
	}
	logVersion()
	cfg, err := newBuilderConfigFromEnvironment(out, false)
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
	switch {
	case log.Is(6):
		serviceability.InitLogrus("DEBUG")
	case log.Is(2):
		serviceability.InitLogrus("INFO")
	case log.Is(0):
		serviceability.InitLogrus("WARN")
	}
	logVersion()
	cfg, err := newBuilderConfigFromEnvironment(out, true)
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
}
