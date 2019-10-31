package dockercfg

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/spf13/pflag"

	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/credentialprovider"

	"github.com/containers/image/v5/types"

	utillog "github.com/openshift/builder/pkg/build/builder/util/log"
)

const (
	PushAuthType       = "PUSH_DOCKERCFG_PATH"
	PullAuthType       = "PULL_DOCKERCFG_PATH"
	PullSourceAuthType = "PULL_SOURCE_DOCKERCFG_PATH_"
	// DockerConfigKey is the key of the required data for SecretTypeDockercfg secrets
	DockerConfigKey = ".dockercfg"
	// DockerConfigJsonKey is the key of the required data for SecretTypeDockerConfigJson secrets
	DockerConfigJsonKey = ".dockerconfigjson"
	dockerConfigFileKey = "config.json"
)

var (
	log                  = utillog.ToFile(os.Stderr, 2)
	dockerFilesToExamine = []string{dockerConfigFileKey, DockerConfigJsonKey, DockerConfigKey}
)

// Helper contains all the valid config options for reading the local dockercfg file
type Helper struct {
}

// NewHelper creates a Flags object with the default values set.
func NewHelper() *Helper {
	return &Helper{}
}

// InstallFlags installs the Docker flag helper into a FlagSet with the default
// options and default values from the Helper object.
func (h *Helper) InstallFlags(flags *pflag.FlagSet) {
}

// GetDockerAuthSearchPaths returns the list of possible locations for the raw
// docker auth file that we'll inspect, where the winning location will be set
// into the containers/image SystemContext.AuthFilePath field
func (h *Helper) GetDockerAuthSearchPaths(authType string) []string {
	log.V(3).Infof("Locating docker config paths for type %s", authType)
	var searchPaths []string
	if pathForAuthType := os.Getenv(authType); len(pathForAuthType) > 0 {
		searchPaths = []string{pathForAuthType}
	} else {
		searchPaths = getExtraSearchPaths()
	}
	log.V(3).Infof("Getting docker config in paths : %v", searchPaths)
	return searchPaths
}

// SetSystemContextFilePath properly seeds the container/image SystemContext
// with the authentication file based on the format implied by the file name
func SetSystemContextFilePath(sc *types.SystemContext, path string) {
	if filepath.Base(path) == dockerConfigFileKey {
		sc.AuthFilePath = path
		return
	}
	sc.LegacyFormatAuthFilePath = path
}

// GetDockerAuth returns a valid Docker AuthConfiguration entry, and whether it was read
// from the local dockercfg file
func (h *Helper) GetDockerAuth(imageName, authType string) (docker.AuthConfiguration, bool) {
	log.V(3).Infof("Locating docker auth for image %s and type %s", imageName, authType)
	searchPaths := h.GetDockerAuthSearchPaths(authType)

	cfg, err := GetDockerConfig(searchPaths)
	if err != nil {
		klog.Errorf("Reading docker config from %v failed: %v", searchPaths, err)
		return docker.AuthConfiguration{}, false
	}

	keyring := credentialprovider.BasicDockerKeyring{}
	keyring.Add(cfg)
	authConfs, found := keyring.Lookup(imageName)
	if !found || len(authConfs) == 0 {
		return docker.AuthConfiguration{}, false
	}
	log.V(3).Infof("Using %s user for Docker authentication for image %s", authConfs[0].Username, imageName)
	return docker.AuthConfiguration{
		Username:      authConfs[0].Username,
		Password:      authConfs[0].Password,
		Email:         authConfs[0].Email,
		ServerAddress: authConfs[0].ServerAddress,
	}, true
}

// GetDockercfgFile returns the path to the dockercfg file
func GetDockercfgFile(path string) string {
	var cfgPath string
	if path != "" {
		cfgPath = path
		// There are 3 valid ways to specify docker config in a secret.
		// 1) with a .dockerconfigjson key pointing to a .docker/config.json file (the key used by k8s for
		//    dockerconfigjson type secrets and the new docker cfg format)
		// 2) with a .dockercfg key+file (the key used by k8s for dockercfg type secrets and the old docker format)
		// 3) with a config.json file because you created your secret using "oc secrets new mysecret .docker/config.json"
		//    so you automatically got a key named config.json containing the new docker cfg format content.
		// we will check to see which one was provided in that priority order.
		if _, err := os.Stat(filepath.Join(path, DockerConfigJsonKey)); err == nil {
			cfgPath = filepath.Join(path, DockerConfigJsonKey)
		} else if _, err := os.Stat(filepath.Join(path, DockerConfigKey)); err == nil {
			cfgPath = filepath.Join(path, DockerConfigKey)
		} else if _, err := os.Stat(filepath.Join(path, "config.json")); err == nil {
			cfgPath = filepath.Join(path, "config.json")
		}
	} else if os.Getenv("DOCKERCFG_PATH") != "" {
		cfgPath = os.Getenv("DOCKERCFG_PATH")
	} else if currentUser, err := user.Current(); err == nil {
		cfgPath = filepath.Join(currentUser.HomeDir, ".docker", "config.json")
	}
	log.V(5).Infof("Using Docker authentication configuration in '%s'", cfgPath)
	return cfgPath
}

func readSpecificDockerConfigJSONFile(filePath string) bool {
	var contents []byte
	var err error

	if contents, err = ioutil.ReadFile(filePath); err != nil {
		log.V(4).Infof("error reading file: %v", err)
		return false
	}
	return readDockerConfigJSONFileFromBytes(contents)
}

func readDockerConfigJSONFileFromBytes(contents []byte) bool {
	var cfgJSON credentialprovider.DockerConfigJson
	if err := json.Unmarshal(contents, &cfgJSON); err != nil {
		log.V(4).Infof("while trying to parse blob %q: %v", contents, err)
		return false
	}
	return true
}

// GetDockerConfigPath returns the first path that provides a valid DockerConfig;
// modified elements from credentialprovider methods called from GetDockerConfig, following the same order of precedenced
// articulated in GetDockerConfig, via the order of file names set in dockerFilesToExamine
func GetDockerConfigPath(paths []string) string {
	for _, configPath := range paths {
		for _, file := range dockerFilesToExamine {
			absDockerConfigFileLocation, err := filepath.Abs(filepath.Join(configPath, file))
			if err != nil {
				log.V(4).Infof("while trying to canonicalize %s: %v", configPath, err)
				continue
			}
			log.V(4).Infof("looking for %s at %s", file, absDockerConfigFileLocation)
			found := readSpecificDockerConfigJSONFile(absDockerConfigFileLocation)
			if found {
				log.V(4).Infof("found valid %s at %s", file, absDockerConfigFileLocation)
				return absDockerConfigFileLocation
			}
		}
	}
	return ""
}

// GetDockerConfig return docker config info by checking given paths
func GetDockerConfig(path []string) (cfg credentialprovider.DockerConfig, err error) {
	if cfg, err = credentialprovider.ReadDockerConfigJSONFile(path); err != nil {
		if cfg, err = ReadDockerConfigJsonFileGeneratedFromSecret(path); err != nil {
			cfg, err = credentialprovider.ReadDockercfgFile(path)
		}
	}
	return cfg, err
}

// ReadDockerConfigJsonFileGeneratedFromSecret return DockerConfig by reading specific file named .dockerconfigjson
// generated by secret from given paths.
func ReadDockerConfigJsonFileGeneratedFromSecret(path []string) (cfg credentialprovider.DockerConfig, err error) {
	for _, filePath := range path {
		cfg, err = credentialprovider.ReadSpecificDockerConfigJsonFile(filepath.Join(filePath, DockerConfigJsonKey))
		if err == nil {
			return cfg, nil
		}
	}
	return nil, err
}

//getExtraSearchPaths get extra paths that may contain docker-config type files.
//this invocation we do not need to handle user.Current() since upstream k8s have handled HOME path
func getExtraSearchPaths() (searchPaths []string) {
	if dockerCfgPath := os.Getenv("DOCKERCFG_PATH"); dockerCfgPath != "" {
		dockerCfgDir := filepath.Dir(dockerCfgPath)
		searchPaths = append(searchPaths, dockerCfgDir)
	}

	return searchPaths
}
