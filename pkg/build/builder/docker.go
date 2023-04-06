package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	dockercmd "github.com/openshift/imagebuilder/dockerfile/command"
	"github.com/openshift/imagebuilder/dockerfile/parser"
	s2iapi "github.com/openshift/source-to-image/pkg/api"

	"github.com/openshift/builder/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/builder/pkg/build/builder/timing"
	builderutil "github.com/openshift/builder/pkg/build/builder/util"
	"github.com/openshift/builder/pkg/build/builder/util/dockerfile"
)

// defaultDockerfilePath is the default path of the Dockerfile
const defaultDockerfilePath = "Dockerfile"

// DockerBuilder builds Docker images given a git repository URL
type DockerBuilder struct {
	dockerClient DockerClient
	build        *buildapiv1.Build
	client       buildclientv1.BuildInterface
	cgLimits     *s2iapi.CGroupLimits
	inputDir     string
}

// NewDockerBuilder creates a new instance of DockerBuilder
func NewDockerBuilder(dockerClient DockerClient, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build, cgLimits *s2iapi.CGroupLimits) *DockerBuilder {
	return &DockerBuilder{
		dockerClient: dockerClient,
		build:        build,
		client:       buildsClient,
		cgLimits:     cgLimits,
		inputDir:     InputContentPath,
	}
}

// Build executes a Docker build
func (d *DockerBuilder) Build() error {

	var err error
	ctx := timing.NewContext(context.Background())
	defer func() {
		d.build.Status.Stages = timing.AppendStageAndStepInfo(d.build.Status.Stages, timing.GetStages(ctx))
		HandleBuildStatusUpdate(d.build, d.client, nil)
	}()

	if d.build.Spec.Source.Git == nil && d.build.Spec.Source.Binary == nil &&
		d.build.Spec.Source.Dockerfile == nil && d.build.Spec.Source.Images == nil {
		return fmt.Errorf("must provide a value for at least one of source, binary, images, or dockerfile")
	}
	var push bool
	pushTag := d.build.Status.OutputDockerImageReference

	// this is where the git-fetch container put the code during the clone operation
	buildDir := d.inputDir

	log.V(4).Infof("Starting Docker build from build config %s ...", d.build.Name)
	// if there is no output target, set one up so the docker build logic
	// (which requires a tag) will still work, but we won't push it at the end.
	if d.build.Spec.Output.To == nil || len(d.build.Spec.Output.To.Name) == 0 {
		d.build.Status.OutputDockerImageReference = d.build.Name
	} else {
		push = true
	}

	buildTag := randomBuildTag(d.build.Namespace, d.build.Name)
	dockerfilePath := getDockerfilePath(buildDir, d.build)

	imageNames, err := findReferencedImages(dockerfilePath, d.build.Spec.Strategy.DockerStrategy.BuildArgs)
	if err != nil {
		return err
	}
	if len(imageNames) == 0 {
		return fmt.Errorf("no FROM image in Dockerfile")
	}
	for _, imageName := range imageNames {
		if imageName == "scratch" {
			log.V(4).Infof("\nSkipping image \"scratch\"")
			continue
		}
		imageExists := true
		_, err = d.dockerClient.InspectImage(imageName)
		if err != nil {
			if err != docker.ErrNoSuchImage {
				log.V(4).Infof("\nError inspecting image \"%s\": %v, continuing", imageName, err)
				continue
			}
			imageExists = false
		}
		// if forcePull or the image does not exist on the node we should pull the image first
		if d.build.Spec.Strategy.DockerStrategy.ForcePull || !imageExists {
			searchPaths := dockercfg.NewHelper().GetDockerAuthSearchPaths(dockercfg.PullAuthType)
			log.V(0).Infof("\nPulling image %s ...", imageName)
			startTime := metav1.Now()
			err = d.pullImage(imageName, searchPaths)

			timing.RecordNewStep(ctx, buildapiv1.StagePullImages, buildapiv1.StepPullBaseImage, startTime, metav1.Now())

			if err != nil {
				d.build.Status.Phase = buildapiv1.BuildPhaseFailed
				d.build.Status.Reason = buildapiv1.StatusReasonPullBuilderImageFailed
				d.build.Status.Message = builderutil.StatusMessagePullBuilderImageFailed
				HandleBuildStatusUpdate(d.build, d.client, nil)
				return fmt.Errorf("failed to pull image: %v", err)
			}

		}
	}

	startTime := metav1.Now()
	err = d.dockerBuild(ctx, buildDir, buildTag)

	timing.RecordNewStep(ctx, buildapiv1.StageBuild, buildapiv1.StepDockerBuild, startTime, metav1.Now())

	if err != nil {
		d.build.Status.Phase = buildapiv1.BuildPhaseFailed
		d.build.Status.Reason = buildapiv1.StatusReasonDockerBuildFailed
		d.build.Status.Message = builderutil.StatusMessageDockerBuildFailed
		HandleBuildStatusUpdate(d.build, d.client, nil)
		return err
	}

	if push {
		if err := tagImage(d.dockerClient, buildTag, pushTag); err != nil {
			return err
		}
	}

	if err := removeImage(d.dockerClient, buildTag); err != nil {
		log.V(0).Infof("warning: Failed to remove temporary build tag %v: %v", buildTag, err)
	}

	if push && pushTag != "" {
		// Get the Docker push authentication
		pushAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(
			pushTag,
			dockercfg.PushAuthType,
		)
		if authPresent {
			log.V(4).Infof("Authenticating Docker push with user %q", pushAuthConfig.Username)
		}
		log.V(0).Infof("\nPushing image %s ...", pushTag)
		startTime = metav1.Now()
		digest, err := d.pushImage(pushTag, pushAuthConfig)

		timing.RecordNewStep(ctx, buildapiv1.StagePushImage, buildapiv1.StepPushDockerImage, startTime, metav1.Now())

		if err != nil {
			d.build.Status.Phase = buildapiv1.BuildPhaseFailed
			d.build.Status.Reason = buildapiv1.StatusReasonPushImageToRegistryFailed
			d.build.Status.Message = builderutil.StatusMessagePushImageToRegistryFailed
			HandleBuildStatusUpdate(d.build, d.client, nil)
			return reportPushFailure(err, authPresent, pushAuthConfig)
		}

		if len(digest) > 0 {
			d.build.Status.Output.To = &buildapiv1.BuildStatusOutputTo{
				ImageDigest: digest,
			}
			HandleBuildStatusUpdate(d.build, d.client, nil)
		}
		log.V(0).Infof("Push successful")
	}
	return nil
}

func (d *DockerBuilder) pullImage(name string, searchPaths []string) error {
	repository, tag := docker.ParseRepositoryTag(name)
	options := docker.PullImageOptions{
		Repository: repository,
		Tag:        tag,
	}

	if options.Tag == "" && strings.Contains(name, "@") {
		options.Repository = name
	}

	return retryImageAction("Pull", func() (pullErr error) {
		return d.dockerClient.PullImage(options, searchPaths)
	})
}

func (d *DockerBuilder) pushImage(name string, authConfig docker.AuthConfiguration) (string, error) {
	repository, tag := docker.ParseRepositoryTag(name)
	options := docker.PushImageOptions{
		Name: repository,
		Tag:  tag,
	}
	var err error
	sha := ""
	retryImageAction("Push", func() (pushErr error) {
		sha, err = d.dockerClient.PushImage(options, authConfig)
		return err
	})
	return sha, err
}

// copyConfigMaps copies all files from the directory where the configMap is
// mounted in the builder pod to a directory where the is the Dockerfile, so
// users can ADD or COPY the files inside their Dockerfile.
func (d *DockerBuilder) copyConfigMaps(configs []buildapiv1.ConfigMapBuildSource, targetDir string) error {
	var err error
	for _, c := range configs {
		err = d.copyLocalObject(configMapSource(c), configMapBuildSourceBaseMountPath, targetDir)
		if err != nil {
			return err
		}
	}
	return nil
}

// copySecrets copies all files from the directory where the secret is
// mounted in the builder pod to a directory where the is the Dockerfile, so
// users can ADD or COPY the files inside their Dockerfile.
func (d *DockerBuilder) copySecrets(secrets []buildapiv1.SecretBuildSource, targetDir string) error {
	var err error
	for _, s := range secrets {
		err = d.copyLocalObject(secretSource(s), secretBuildSourceBaseMountPath, targetDir)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DockerBuilder) copyLocalObject(s localObjectBuildSource, sourceDir, targetDir string) error {
	dstDir := filepath.Join(targetDir, s.DestinationPath())
	if err := os.MkdirAll(dstDir, 0777); err != nil {
		return err
	}
	log.V(3).Infof("Copying files from the build source %q to %q", s.LocalObjectRef().Name, dstDir)

	// Build sources contain nested directories and fairly baroque links. To prevent extra data being
	// copied, perform the following steps:
	//
	// 1. Only top level files and directories within the secret directory are candidates
	// 2. Any item starting with '..' is ignored
	// 3. Destination directories are created first with 0777
	// 4. Use the '-L' option to cp to copy only contents.
	//
	srcDir := filepath.Join(sourceDir, s.LocalObjectRef().Name)
	if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if srcDir == path {
			return nil
		}

		// skip any contents that begin with ".."
		if strings.HasPrefix(filepath.Base(path), "..") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// ensure all directories are traversable
		if info.IsDir() {
			if err := os.MkdirAll(dstDir, 0777); err != nil {
				return err
			}
		}
		out, err := exec.Command("cp", "-vLRf", path, dstDir+"/").Output()
		if err != nil {
			log.V(4).Infof("Build source %q failed to copy: %q", s.LocalObjectRef().Name, string(out))
			return err
		}
		// See what is copied when debugging.
		log.V(5).Infof("Result of build source copy %s\n%s", s.LocalObjectRef().Name, string(out))
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// dockerBuild performs a docker build on the source that has been retrieved
func (d *DockerBuilder) dockerBuild(ctx context.Context, dir string, tag string) error {
	var noCache bool
	var forcePull bool
	var buildArgs []docker.BuildArg
	dockerfilePath := defaultDockerfilePath
	if d.build.Spec.Strategy.DockerStrategy != nil {
		if d.build.Spec.Source.ContextDir != "" {
			dir = filepath.Join(dir, d.build.Spec.Source.ContextDir)
		}
		if d.build.Spec.Strategy.DockerStrategy.DockerfilePath != "" {
			dockerfilePath = d.build.Spec.Strategy.DockerStrategy.DockerfilePath
		}
		for _, ba := range d.build.Spec.Strategy.DockerStrategy.BuildArgs {
			buildArgs = append(buildArgs, docker.BuildArg{Name: ba.Name, Value: ba.Value})
		}
		noCache = d.build.Spec.Strategy.DockerStrategy.NoCache
		forcePull = d.build.Spec.Strategy.DockerStrategy.ForcePull
	}

	auth := mergeNodeCredentialsDockerAuth(os.Getenv(dockercfg.PullAuthType))

	if err := d.copySecrets(d.build.Spec.Source.Secrets, dir); err != nil {
		return err
	}
	if err := d.copyConfigMaps(d.build.Spec.Source.ConfigMaps, dir); err != nil {
		return err
	}

	opts := docker.BuildImageOptions{
		Context:             ctx,
		Name:                tag,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		OutputStream:        os.Stdout,
		Dockerfile:          dockerfilePath,
		NoCache:             noCache,
		Pull:                forcePull,
		BuildArgs:           buildArgs,
		ContextDir:          dir,
	}

	// Though we are capped on memory and cpu at the cgroup parent level,
	// some build containers care what their memory limit is so they can
	// adapt, thus we need to set the memory limit at the container level
	// too, so that information is available to them.
	if d.cgLimits != nil {
		opts.CPUPeriod = d.cgLimits.CPUPeriod
		opts.CPUQuota = d.cgLimits.CPUQuota
		opts.CPUShares = d.cgLimits.CPUShares
		opts.Memory = d.cgLimits.MemoryLimitBytes
		opts.Memswap = d.cgLimits.MemorySwap
		opts.CgroupParent = d.cgLimits.Parent
	}

	if auth != nil {
		opts.AuthConfigs = *auth
	}

	return d.dockerClient.BuildImage(opts)
}

func getDockerfilePath(dir string, build *buildapiv1.Build) string {
	var contextDirPath string
	if build.Spec.Strategy.DockerStrategy != nil && len(build.Spec.Source.ContextDir) > 0 {
		contextDirPath = filepath.Join(dir, build.Spec.Source.ContextDir)
	} else {
		contextDirPath = dir
	}

	var dockerfilePath string
	if build.Spec.Strategy.DockerStrategy != nil && len(build.Spec.Strategy.DockerStrategy.DockerfilePath) > 0 {
		dockerfilePath = filepath.Join(contextDirPath, build.Spec.Strategy.DockerStrategy.DockerfilePath)
	} else {
		dockerfilePath = filepath.Join(contextDirPath, defaultDockerfilePath)
	}
	return dockerfilePath
}

// replaceLastFrom changes the last FROM instruction of node to point to the
// given image with an optional alias.
func replaceLastFrom(node *parser.Node, image string, alias string) {
	if node == nil {
		return
	}
	for i := len(node.Children) - 1; i >= 0; i-- {
		child := node.Children[i]
		if child != nil && child.Value == dockercmd.From {
			if child.Next == nil {
				child.Next = &parser.Node{}
			}

			log.Infof("Replaced Dockerfile FROM image %s", child.Next.Value)
			child.Next.Value = image
			if len(alias) != 0 {
				if child.Next.Next == nil {
					child.Next.Next = &parser.Node{}
				}
				child.Next.Next.Value = "as"
				if child.Next.Next.Next == nil {
					child.Next.Next.Next = &parser.Node{}
				}
				child.Next.Next.Next.Value = alias
			}
			return
		}
	}
}

// getLastFrom gets the image name of the last FROM instruction
// in the dockerfile
func getLastFrom(node *parser.Node) (string, string) {
	if node == nil {
		return "", ""
	}
	var image, alias string
	for i := len(node.Children) - 1; i >= 0; i-- {
		child := node.Children[i]
		if child != nil && child.Value == dockercmd.From {
			if child.Next != nil {
				image = child.Next.Value
				if child.Next.Next != nil && strings.ToUpper(child.Next.Next.Value) == "AS" {
					if child.Next.Next.Next != nil {
						alias = child.Next.Next.Next.Value
					}
				}
				break
			}
		}
	}
	return image, alias
}

// appendEnv appends an ENV Dockerfile instruction as the last child of node
// with keys and values from m.
func appendEnv(node *parser.Node, m []dockerfile.KeyValue) error {
	return appendKeyValueInstruction(dockerfile.Env, node, m)
}

// appendLabel appends a LABEL Dockerfile instruction as the last child of node
// with keys and values from m.
func appendLabel(node *parser.Node, m []dockerfile.KeyValue) error {
	if len(m) == 0 {
		return nil
	}
	return appendKeyValueInstruction(dockerfile.Label, node, m)
}

// appendPostCommit appends a RUN <cmd> Dockerfile instruction as the last child of node
func appendPostCommit(node *parser.Node, cmd string) error {
	if len(cmd) == 0 {
		return nil
	}

	image, alias := getLastFrom(node)
	if len(alias) == 0 {
		alias = postCommitAlias
		replaceLastFrom(node, image, alias)
	}

	if err := appendStringInstruction(dockerfile.From, node, alias); err != nil {
		return err
	}

	if err := appendStringInstruction(dockerfile.Run, node, cmd); err != nil {
		return err
	}

	if err := appendStringInstruction(dockerfile.From, node, alias); err != nil {
		return err
	}

	return nil
}

// appendStringInstruction is a primitive used to avoid code duplication.
// Callers should use a derivative of this such as appendPostCommit.
// appendStringInstruction appends a Dockerfile instruction with string
// syntax created by f as the last child of node with the string from cmd.
func appendStringInstruction(f func(string) (string, error), node *parser.Node, cmd string) error {
	if node == nil {
		return nil
	}
	instruction, err := f(cmd)
	if err != nil {
		return err
	}
	return dockerfile.InsertInstructions(node, len(node.Children), instruction)
}

// appendKeyValueInstruction is a primitive used to avoid code duplication.
// Callers should use a derivative of this such as appendEnv or appendLabel.
// appendKeyValueInstruction appends a Dockerfile instruction with key-value
// syntax created by f as the last child of node with keys and values from m.
func appendKeyValueInstruction(f func([]dockerfile.KeyValue) (string, error), node *parser.Node, m []dockerfile.KeyValue) error {
	if node == nil {
		return nil
	}
	instruction, err := f(m)
	if err != nil {
		return err
	}
	return dockerfile.InsertInstructions(node, len(node.Children), instruction)
}

// insertEnvAfterFrom inserts an ENV instruction with the environment variables
// from env after every FROM instruction in node.
func insertEnvAfterFrom(node *parser.Node, env []corev1.EnvVar) error {
	if node == nil || len(env) == 0 {
		return nil
	}

	// Build ENV instruction.
	var m []dockerfile.KeyValue
	for _, e := range env {
		m = append(m, dockerfile.KeyValue{Key: e.Name, Value: e.Value})
	}
	buildEnv, err := dockerfile.Env(m)
	if err != nil {
		return err
	}

	// Insert the buildEnv after every FROM instruction.
	// We iterate in reverse order, otherwise indices would have to be
	// recomputed after each step, because we're changing node in-place.
	indices := dockerfile.FindAll(node, dockercmd.From)
	for i := len(indices) - 1; i >= 0; i-- {
		err := dockerfile.InsertInstructions(node, indices[i]+1, buildEnv)
		if err != nil {
			return err
		}
	}

	return nil
}
