package builder

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	kvalidation "k8s.io/apimachinery/pkg/util/validation"

	buildapiv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/library-go/pkg/build/naming"
	s2iapi "github.com/openshift/source-to-image/pkg/api"
	s2iutil "github.com/openshift/source-to-image/pkg/util"

	builderutil "github.com/openshift/builder/pkg/build/builder/util"
)

// Mount paths for certificate authorities
const (
	ConfigMapCertsMountPath = "/var/run/configs/openshift.io/certs"
	SecretCertsMountPath    = "/var/run/secrets/kubernetes.io/serviceaccount"
)

var (
	// procCGroupPattern is a regular expression that parses the entries in /proc/self/cgroup
	procCGroupPattern = regexp.MustCompile(`\d+:([a-z_,]+):/.*/(\w+-|)([a-z0-9]+).*`)

	// ClientTypeUnknown is an error returned when we can't figure out
	// which type of "client" we're using.
	ClientTypeUnknown = errors.New("internal error: method not implemented for this client type")
)

// MergeEnv will take an existing environment and merge it with a new set of
// variables. For variables with the same name in both, only the one in the
// new environment will be kept.
func MergeEnv(oldEnv, newEnv []string) []string {
	key := func(e string) string {
		i := strings.Index(e, "=")
		if i == -1 {
			return e
		}
		return e[:i]
	}
	result := []string{}
	newVars := map[string]struct{}{}
	for _, e := range newEnv {
		newVars[key(e)] = struct{}{}
	}
	result = append(result, newEnv...)
	for _, e := range oldEnv {
		if _, exists := newVars[key(e)]; exists {
			continue
		}
		result = append(result, e)
	}
	return result
}

func reportPushFailure(err error, authPresent bool, pushAuthConfig docker.AuthConfiguration) error {
	// write extended error message to assist in problem resolution
	if authPresent {
		log.V(0).Infof("Registry server Address: %s", pushAuthConfig.ServerAddress)
		log.V(0).Infof("Registry server User Name: %s", pushAuthConfig.Username)
		log.V(0).Infof("Registry server Email: %s", pushAuthConfig.Email)
		passwordPresent := "<<empty>>"
		if len(pushAuthConfig.Password) > 0 {
			passwordPresent = "<<non-empty>>"
		}
		log.V(0).Infof("Registry server Password: %s", passwordPresent)
	}
	return fmt.Errorf("Failed to push image: %v", err)
}

// addBuildLabels adds some common image labels describing the build that produced
// this image.
func addBuildLabels(labels map[string]string, build *buildapiv1.Build) {
	labels[builderutil.DefaultDockerLabelNamespace+"build.name"] = build.Name
	labels[builderutil.DefaultDockerLabelNamespace+"build.namespace"] = build.Namespace
}

// SafeForLoggingEnvironmentList returns a copy of an s2i EnvironmentList array with
// proxy credential values redacted.
func SafeForLoggingEnvironmentList(env s2iapi.EnvironmentList) s2iapi.EnvironmentList {
	newEnv := make(s2iapi.EnvironmentList, len(env))
	copy(newEnv, env)
	proxyRegex := regexp.MustCompile("(?i)proxy")
	for i, env := range newEnv {
		if proxyRegex.MatchString(env.Name) {
			newEnv[i].Value, _ = s2iutil.SafeForLoggingURL(env.Value)
		}
	}
	return newEnv
}

// SafeForLoggingS2IConfig returns a copy of an s2i Config with
// proxy credentials redacted.
func SafeForLoggingS2IConfig(config *s2iapi.Config) *s2iapi.Config {
	newConfig := *config
	newConfig.Environment = SafeForLoggingEnvironmentList(config.Environment)
	if config.ScriptDownloadProxyConfig != nil {
		newProxy := *config.ScriptDownloadProxyConfig
		newConfig.ScriptDownloadProxyConfig = &newProxy
		if newConfig.ScriptDownloadProxyConfig.HTTPProxy != nil {
			newConfig.ScriptDownloadProxyConfig.HTTPProxy = builderutil.SafeForLoggingURL(newConfig.ScriptDownloadProxyConfig.HTTPProxy)
		}

		if newConfig.ScriptDownloadProxyConfig.HTTPProxy != nil {
			newConfig.ScriptDownloadProxyConfig.HTTPSProxy = builderutil.SafeForLoggingURL(newConfig.ScriptDownloadProxyConfig.HTTPProxy)
		}
	}
	newConfig.ScriptsURL, _ = s2iutil.SafeForLoggingURL(newConfig.ScriptsURL)
	return &newConfig
}

// ReadLines reads the content of the given file into a string slice
func ReadLines(fileName string) ([]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// ParseProxyURL parses a proxy URL and allows fallback to non-URLs like
// myproxy:80 (for example) which url.Parse no longer accepts in Go 1.8.  The
// logic is copied from net/http.ProxyFromEnvironment to try to maintain
// backwards compatibility.
func ParseProxyURL(proxy string) (*url.URL, error) {
	proxyURL, err := url.Parse(proxy)

	// logic copied from net/http.ProxyFromEnvironment
	if err != nil || !strings.HasPrefix(proxyURL.Scheme, "http") {
		// proxy was bogus. Try prepending "http://" to it and see if that
		// parses correctly. If not, we fall through and complain about the
		// original one.
		if proxyURL, err := url.Parse("http://" + proxy); err == nil {
			return proxyURL, nil
		}
	}

	return proxyURL, err
}

// NameForBuildVolume returns a valid pod volume name for the provided build volume name.
func NameForBuildVolume(objName string) string {
	// Volume names must be a valid DNS Label - see https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
	return naming.GetName(strings.ToLower(objName), buildVolumeSuffix, kvalidation.DNS1123LabelMaxLength)
}

// PathForBuildVolume returns the path in the builder container where the build volume is mounted.
// This should not be confused with the destination path for the volume inside buildah's runtime environment.
func PathForBuildVolume(objName string) string {
	return filepath.Join(buildVolumeMountPath, NameForBuildVolume(objName))
}

// normalizeRegistryLocation munges a registry location that's mistakenly been
// provided in the old http/https URL format, which containers/image now
// rejects, into a proper location prefix.  Locations which don't look like
// http: or https: URLs are returned unmodified.
//
// URL path components are preserved, which is a change from how this would
// have worked in earlier 4.x, but brings behavior closer to how pushing
// credentials are handled (and how this worked in 3.x): those credentials are
// selected by matching the registry name using filename-style matching of the
// host name component, and by checking if the path component is a string (not
// path) prefix of the repository that the image will be pushed to, as is
// typical for Kubernetes.
//
// The callers of this function are using its result to supply credentials to
// containers/image to select from while pulling an image.
// containers/image doesn't implement the wildcard or string-prefix matching
// logic when it's deciding which of the credentials it's been passed should be
// used, and in older versions which accepted http: and https: URLS, it ignored
// the path component, so in addition to letting the builder accept them again,
// this allows path components in URLs to be matched using container/image
// rules (i.e., no wildcard and prefix matching).
//
// An alternate approach would have been to populate a
// k8s.io/kubernetes/pkg/credentialprovider.BasicDockerKeyring with all of the
// secrets we know, pass the name of every image we'll pull to its Lookup()
// method, and pass the image library the returned credential for each image's
// location.  The challenge of predicting which images will be pulled along
// every code path, and factoring in search registries, made this approach more
// attractive.
func normalizeRegistryLocation(location string) string {
	if !strings.HasPrefix(location, "http://") && !strings.HasPrefix(location, "https://") {
		return location
	}
	cleaned := strings.Split(strings.TrimPrefix(strings.TrimPrefix(location, "http://"), "https://"), "/")
	switch cleaned[0] {
	case "index.docker.io", "registry-1.docker.io":
		cleaned[0] = "docker.io"
	}
	if len(cleaned) > 1 {
		if cleaned[1] == "v1" || cleaned[1] == "v2" {
			cleaned = append(cleaned[:1], cleaned[2:]...)
		}
	}
	return strings.TrimSuffix(strings.Join(cleaned, "/"), "/")
}
