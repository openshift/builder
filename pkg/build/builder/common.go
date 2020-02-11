package builder

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	dockercmd "github.com/openshift/imagebuilder/dockerfile/command"
	"github.com/openshift/imagebuilder/dockerfile/parser"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/imagebuilder"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/util"

	buildapiv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/builder/pkg/build/builder/timing"
	builderutil "github.com/openshift/builder/pkg/build/builder/util"
	"github.com/openshift/builder/pkg/build/builder/util/dockerfile"
	utillog "github.com/openshift/builder/pkg/build/builder/util/log"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	"github.com/openshift/library-go/pkg/git"
)

// postCommitAlias is a unique key to use for an alias in
// buildPostCommit so that we don't duplicate anyone's alias
var postCommitAlias = "appimage" + strings.Replace(string(uuid.NewUUID()), "-", "", -1)

const (
	// containerNamePrefix prefixes the name of containers launched by a build.
	// We cannot reuse the prefix "k8s" because we don't want the containers to
	// be managed by a kubelet.
	containerNamePrefix = "openshift"
	// configMapBuildSourceBaseMountPath is the path that the controller will have
	// mounted configmap input content within the build pod
	configMapBuildSourceBaseMountPath = "/var/run/configs/openshift.io/build"
	// SecretBuildSourceBaseMountPath is the path that the controller will have
	// mounted secret input content within the build pod
	secretBuildSourceBaseMountPath = "/var/run/secrets/openshift.io/build"
	// BuildWorkDirMount is the working directory within the build pod, mounted as a volume.
	buildWorkDirMount = "/tmp/build"
)

var (
	// log is a placeholder until the builders pass an output stream down
	// client facing libraries should not be using log
	log = utillog.ToFile(os.Stderr, 2)

	// InputContentPath is the path at which the build inputs will be available
	// to all the build containers.
	InputContentPath = filepath.Join(buildWorkDirMount, "inputs")
)

// KeyValue can be used to build ordered lists of key-value pairs.
type KeyValue struct {
	Key   string
	Value string
}

// GitClient performs git operations
type GitClient interface {
	CloneWithOptions(dir string, url string, args ...string) error
	Fetch(dir string, url string, ref string) error
	Checkout(dir string, ref string) error
	PotentialPRRetryAsFetch(dir string, url string, ref string, err error) error
	SubmoduleUpdate(dir string, init, recursive bool) error
	TimedListRemote(timeout time.Duration, url string, args ...string) (string, string, error)
	GetInfo(location string) (*git.SourceInfo, []error)
}

// localObjectBuildSource is a build source that is copied into a build from a Kubernetes
// key-value store, such as a `Secret` or `ConfigMap`.
type localObjectBuildSource interface {
	// LocalObjectRef returns a reference to a local Kubernetes object by name.
	LocalObjectRef() corev1.LocalObjectReference
	// DestinationPath returns the directory where the files from the build source should be
	// available for the build time.
	// For the Source build strategy, these will be injected into a container
	// where the assemble script runs.
	// For the Docker build strategy, these will be copied into the build
	// directory, where the Dockerfile is located, so users can ADD or COPY them
	// during docker build.
	DestinationPath() string
	// IsSecret returns `true` if the build source is a `Secret` containing sensitive data.
	IsSecret() bool
}

type configMapSource buildapiv1.ConfigMapBuildSource

func (c configMapSource) LocalObjectRef() corev1.LocalObjectReference {
	return c.ConfigMap
}

func (c configMapSource) DestinationPath() string {
	return c.DestinationDir
}

func (c configMapSource) IsSecret() bool {
	return false
}

type secretSource buildapiv1.SecretBuildSource

func (s secretSource) LocalObjectRef() corev1.LocalObjectReference {
	return s.Secret
}

func (s secretSource) DestinationPath() string {
	return s.DestinationDir
}

func (s secretSource) IsSecret() bool {
	return true
}

// buildInfo returns a slice of KeyValue pairs with build metadata to be
// inserted into Docker images produced by build.
func buildInfo(build *buildapiv1.Build, sourceInfo *git.SourceInfo) []KeyValue {
	kv := []KeyValue{
		{"OPENSHIFT_BUILD_NAME", build.Name},
		{"OPENSHIFT_BUILD_NAMESPACE", build.Namespace},
	}
	if build.Spec.Source.Git != nil {
		kv = append(kv, KeyValue{"OPENSHIFT_BUILD_SOURCE", build.Spec.Source.Git.URI})
		if build.Spec.Source.Git.Ref != "" {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_REFERENCE", build.Spec.Source.Git.Ref})
		}

		if sourceInfo != nil && len(sourceInfo.CommitID) != 0 {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_COMMIT", sourceInfo.CommitID})
		} else if build.Spec.Revision != nil && build.Spec.Revision.Git != nil && build.Spec.Revision.Git.Commit != "" {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_COMMIT", build.Spec.Revision.Git.Commit})
		}
	}
	if build.Spec.Strategy.SourceStrategy != nil {
		env := build.Spec.Strategy.SourceStrategy.Env
		for _, e := range env {
			kv = append(kv, KeyValue{e.Name, e.Value})
		}
	}
	return kv
}

// randomBuildTag generates a random tag used for building images in such a way
// that the built image can be referred to unambiguously even in the face of
// concurrent builds with the same name in the same namespace.
func randomBuildTag(namespace, name string) string {
	repo := fmt.Sprintf("temp.builder.openshift.io/%s/%s", namespace, name)
	randomTag := fmt.Sprintf("%08x", rand.Uint32())
	maxRepoLen := reference.NameTotalLengthMax - len(randomTag) - 1
	if len(repo) > maxRepoLen {
		repo = fmt.Sprintf("%x", sha1.Sum([]byte(repo)))
	}
	return fmt.Sprintf("%s:%s", repo, randomTag)
}

// containerName creates names for Docker containers launched by a build. It is
// meant to resemble Kubernetes' pkg/kubelet/dockertools.BuildDockerName.
func containerName(strategyName, buildName, namespace, containerPurpose string) string {
	uid := fmt.Sprintf("%08x", rand.Uint32())
	return fmt.Sprintf("%s_%s-build_%s_%s_%s_%s",
		containerNamePrefix,
		strategyName,
		buildName,
		namespace,
		containerPurpose,
		uid)
}

// buildPostCommit transforms the supplied BuildPostCommitSpec into dockerfile commands.
func buildPostCommit(postCommitSpec buildapiv1.BuildPostCommitSpec) string {
	command := postCommitSpec.Command
	args := postCommitSpec.Args
	script := postCommitSpec.Script

	if script == "" && len(command) == 0 && len(args) == 0 {
		// Post commit hook is not set, return early.
		return ""
	}

	log.V(4).Infof("Post commit hook spec: %+v", postCommitSpec)

	if script != "" {
		// The `-i` flag is needed to support CentOS and RHEL images
		// that use Software Collections (SCL), in order to have the
		// appropriate collections enabled in the shell. E.g., in the
		// Ruby image, this is necessary to make `ruby`, `bundle` and
		// other binaries available in the PATH.
		command = []string{"/bin/sh", "-ic"}
		args = append([]string{script}, args...)

		return strings.TrimSpace(fmt.Sprintf("%s '%s'", strings.Join(command, " "), strings.Join(args, " ")))

	}

	return strings.TrimSpace(fmt.Sprintf("%s %s", strings.Join(command, " "), strings.Join(args, " ")))
}

// GetSourceRevision returns a SourceRevision object either from the build (if it already had one)
// or by creating one from the sourceInfo object passed in.
func GetSourceRevision(build *buildapiv1.Build, sourceInfo *git.SourceInfo) *buildapiv1.SourceRevision {
	if build.Spec.Revision != nil {
		return build.Spec.Revision
	}
	return &buildapiv1.SourceRevision{
		Git: &buildapiv1.GitSourceRevision{
			Commit:  sourceInfo.CommitID,
			Message: sourceInfo.Message,
			Author: buildapiv1.SourceControlUser{
				Name:  sourceInfo.AuthorName,
				Email: sourceInfo.AuthorEmail,
			},
			Committer: buildapiv1.SourceControlUser{
				Name:  sourceInfo.CommitterName,
				Email: sourceInfo.CommitterEmail,
			},
		},
	}
}

// HandleBuildStatusUpdate handles updating the build status
// retries occur on update conflict and unreachable api server
func HandleBuildStatusUpdate(build *buildapiv1.Build, client buildclientv1.BuildInterface, sourceRev *buildapiv1.SourceRevision) {
	var latestBuild *buildapiv1.Build
	var err error

	updateBackoff := wait.Backoff{
		Steps:    10,
		Duration: 25 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
	}

	wait.ExponentialBackoff(updateBackoff, func() (bool, error) {
		// before updating, make sure we are using the latest version of the build
		if latestBuild == nil {
			latestBuild, err = client.Get(build.Name, metav1.GetOptions{})
			if err != nil {
				latestBuild = nil
				return false, nil
			}
			if latestBuild.Name == "" {
				latestBuild = nil
				err = fmt.Errorf("latest version of build %s is empty", build.Name)
				return false, nil
			}
		}

		if sourceRev != nil {
			latestBuild.Spec.Revision = sourceRev
			latestBuild.ResourceVersion = ""
		}
		latestBuild.Status.Phase = build.Status.Phase
		latestBuild.Status.Reason = build.Status.Reason
		latestBuild.Status.Message = build.Status.Message
		latestBuild.Status.Output.To = build.Status.Output.To
		latestBuild.Status.Stages = timing.AppendStageAndStepInfo(latestBuild.Status.Stages, build.Status.Stages)

		_, err = client.UpdateDetails(latestBuild.Name, latestBuild)

		switch {
		case err == nil:
			return true, nil
		case errors.IsConflict(err):
			latestBuild = nil
		}

		log.V(4).Infof("Retryable error occurred, retrying.  error: %v", err)

		return false, nil

	})

	if err != nil {
		log.Infof("error: Unable to update build status: %v", err)
	}
}

// buildEnv converts the buildInfo output to a format that appendEnv can
// consume.
func buildEnv(build *buildapiv1.Build, sourceInfo *git.SourceInfo) []dockerfile.KeyValue {
	bi := buildInfo(build, sourceInfo)
	kv := make([]dockerfile.KeyValue, len(bi))
	for i, item := range bi {
		kv[i] = dockerfile.KeyValue{Key: item.Key, Value: item.Value}
	}
	return kv
}

// TODO: remove this shim (required to adapt vendored types)
func toS2ISourceInfo(sourceInfo *git.SourceInfo) *s2igit.SourceInfo {
	return &s2igit.SourceInfo{
		Ref:            sourceInfo.Ref,
		CommitID:       sourceInfo.CommitID,
		Date:           sourceInfo.Date,
		AuthorName:     sourceInfo.AuthorName,
		AuthorEmail:    sourceInfo.AuthorEmail,
		CommitterName:  sourceInfo.CommitterName,
		CommitterEmail: sourceInfo.CommitterEmail,
		Message:        sourceInfo.Message,
		Location:       sourceInfo.Location,
		ContextDir:     sourceInfo.ContextDir,
	}
}

// buildLabels returns a slice of KeyValue pairs in a format that appendLabel can
// consume.
func buildLabels(build *buildapiv1.Build, sourceInfo *git.SourceInfo) []dockerfile.KeyValue {
	labels := map[string]string{}
	if sourceInfo == nil {
		sourceInfo = &git.SourceInfo{}
	}
	if len(build.Spec.Source.ContextDir) > 0 {
		sourceInfo.ContextDir = build.Spec.Source.ContextDir
	}
	labels = util.GenerateLabelsFromSourceInfo(labels, toS2ISourceInfo(sourceInfo), builderutil.DefaultDockerLabelNamespace)
	if build != nil && build.Spec.Source.Git != nil && build.Spec.Source.Git.Ref != "" {
		// override the io.openshift.build.commit.ref label to match what we
		// were originally told to check out, as well as the
		// OPENSHIFT_BUILD_REFERENCE environment variable.  This can sometimes
		// differ from git's view (see PotentialPRRetryAsFetch for details).
		labels[builderutil.DefaultDockerLabelNamespace+"build.commit.ref"] = build.Spec.Source.Git.Ref
	}
	addBuildLabels(labels, build)

	kv := make([]dockerfile.KeyValue, 0, len(labels)+len(build.Spec.Output.ImageLabels))
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		kv = append(kv, dockerfile.KeyValue{Key: k, Value: labels[k]})
	}
	// override autogenerated labels with user provided labels
	for _, lbl := range build.Spec.Output.ImageLabels {
		kv = append(kv, dockerfile.KeyValue{Key: lbl.Name, Value: lbl.Value})
	}
	return kv
}

// readSourceInfo reads the persisted git info from disk (if any) back into a SourceInfo
// object.
func readSourceInfo() (*git.SourceInfo, error) {
	sourceInfoPath := filepath.Join(buildWorkDirMount, "sourceinfo.json")
	if _, err := os.Stat(sourceInfoPath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := ioutil.ReadFile(sourceInfoPath)
	if err != nil {
		return nil, err
	}
	sourceInfo := &git.SourceInfo{}
	err = json.Unmarshal(data, &sourceInfo)
	if err != nil {
		return nil, err
	}

	log.V(4).Infof("Found git source info: %#v", *sourceInfo)
	return sourceInfo, nil
}

// addBuildParameters checks if a Image is set to replace the default base image.
// If that's the case then change the Dockerfile to make the build with the given image.
// Also append the environment variables and labels in the Dockerfile.
func addBuildParameters(dir string, build *buildapiv1.Build, sourceInfo *git.SourceInfo) error {
	dockerfilePath := getDockerfilePath(dir, build)

	in, err := ioutil.ReadFile(dockerfilePath)
	if err != nil {
		return err
	}
	node, err := imagebuilder.ParseDockerfile(bytes.NewBuffer(in))
	if err != nil {
		return err
	}

	// Update base image if build strategy specifies the From field.
	if build.Spec.Strategy.DockerStrategy != nil && build.Spec.Strategy.DockerStrategy.From != nil && build.Spec.Strategy.DockerStrategy.From.Kind == "DockerImage" {
		// Reduce the name to a minimal canonical form for the daemon
		name := build.Spec.Strategy.DockerStrategy.From.Name
		if ref, err := imagereference.Parse(name); err == nil {
			name = ref.DaemonMinimal().Exact()
		}
		err := replaceLastFrom(node, name, "")
		if err != nil {
			return err
		}
	}

	// Append build info as environment variables.
	if err := appendEnv(node, buildEnv(build, sourceInfo)); err != nil {
		return err
	}

	// Append build labels.
	if err := appendLabel(node, buildLabels(build, sourceInfo)); err != nil {
		return err
	}

	// Append post commit
	if err := appendPostCommit(node, buildPostCommit(build.Spec.PostCommit)); err != nil {
		return err
	}

	// Insert environment variables defined in the build strategy.
	if err := insertEnvAfterFrom(node, build.Spec.Strategy.DockerStrategy.Env); err != nil {
		return err
	}

	if err := replaceImagesFromSource(node, build.Spec.Source.Images); err != nil {
		return err
	}

	out := dockerfile.Write(node)
	log.V(4).Infof("Replacing dockerfile\n%s\nwith:\n%s", string(in), string(out))
	return overwriteFile(dockerfilePath, out)
}

// replaceImagesFromSource updates a single or multi-stage Dockerfile with any replacement
// image sources ('FROM <name>' and 'COPY --from=<name>'). It operates on exact string matches
// and performs no interpretation of names from the Dockerfile.
func replaceImagesFromSource(node *parser.Node, imageSources []buildapiv1.ImageSource) error {
	replacements := make(map[string]string)
	for _, image := range imageSources {
		if image.From.Kind != "DockerImage" || len(image.From.Name) == 0 {
			continue
		}
		for _, name := range image.As {
			replacements[name] = image.From.Name
		}
	}
	names := make(map[string]string)
	stages, err := imagebuilder.NewStages(node, imagebuilder.NewBuilder(make(map[string]string)))
	if err != nil {
		return err
	}
	for _, stage := range stages {
		for _, child := range stage.Node.Children {
			switch {
			case child.Value == dockercmd.From && child.Next != nil:
				image := child.Next.Value
				if replacement, ok := replacements[image]; ok {
					child.Next.Value = replacement
				}
				names[stage.Name] = image
			case child.Value == dockercmd.Copy:
				if ref, ok := nodeHasFromRef(child); ok {
					if len(ref) > 0 {
						if _, ok := names[ref]; !ok {
							if replacement, ok := replacements[ref]; ok {
								nodeReplaceFromRef(child, replacement)
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// findReferencedImages returns all qualified images referenced by the Dockerfile, or returns an error.
func findReferencedImages(dockerfilePath string) ([]string, error) {
	if len(dockerfilePath) == 0 {
		return nil, nil
	}
	node, err := imagebuilder.ParseFile(dockerfilePath)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string)
	images := sets.NewString()
	stages, err := imagebuilder.NewStages(node, imagebuilder.NewBuilder(make(map[string]string)))
	if err != nil {
		return nil, err
	}
	for _, stage := range stages {
		for _, child := range stage.Node.Children {
			switch {
			case child.Value == dockercmd.From && child.Next != nil:
				image := child.Next.Value
				names[stage.Name] = image
				if _, ok := names[image]; !ok {
					images.Insert(image)
				}
			case child.Value == dockercmd.Copy:
				if ref, ok := nodeHasFromRef(child); ok {
					if len(ref) > 0 {
						if _, ok := names[ref]; !ok {
							images.Insert(ref)
						}
					}
				}
			}
		}
	}
	return images.List(), nil
}

func overwriteFile(name string, out []byte) error {
	f, err := os.OpenFile(name, os.O_TRUNC|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := f.Write(out); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func nodeHasFromRef(node *parser.Node) (string, bool) {
	for _, arg := range node.Flags {
		switch {
		case strings.HasPrefix(arg, "--from="):
			from := strings.TrimPrefix(arg, "--from=")
			return from, true
		}
	}
	return "", false
}

func nodeReplaceFromRef(node *parser.Node, name string) {
	for i, arg := range node.Flags {
		switch {
		case strings.HasPrefix(arg, "--from="):
			node.Flags[i] = fmt.Sprintf("--from=%s", name)
		}
	}
}
