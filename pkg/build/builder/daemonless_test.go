package builder

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/credentialprovider"

	docker "github.com/fsouza/go-dockerclient"
	builderutil "github.com/openshift/builder/pkg/build/builder/util"
)

func TestMergeNodeCredentials(t *testing.T) {
	for _, tt := range []struct {
		name      string
		nsCreds   string
		nodeCreds string
		expected  credentialprovider.DockerConfig
	}{
		{
			name: "invalid namespace credentials file path",
		},
		{
			name:    "invalid namespace credentials file",
			nsCreds: "testdata/empty.txt",
		},
		{
			name:     "empty namespace credentials",
			nsCreds:  "testdata/credentials-empty.json",
			expected: map[string]credentialprovider.DockerConfigEntry{},
		},
		{
			name:    "valid namespace credentials",
			nsCreds: "testdata/credentials-quayio-user0.json",
			expected: map[string]credentialprovider.DockerConfigEntry{
				"quay.io": {
					Username: "user0",
					Password: "pass0",
					Email:    "user0@redhat.com",
				},
			},
		},
		{
			name:      "merge namespace with node credentials",
			nsCreds:   "testdata/credentials-quayio-user0.json",
			nodeCreds: "testdata/credentials-redhatio-nodeuser.json",
			expected: map[string]credentialprovider.DockerConfigEntry{
				"quay.io": {
					Username: "user0",
					Password: "pass0",
					Email:    "user0@redhat.com",
				},
				"registry.redhat.io": {
					Username: "nodeuser",
					Password: "nodepass",
					Email:    "nodeuser@redhat.com",
				},
			},
		},
		{
			name:      "overwriting node credentials",
			nodeCreds: "testdata/credentials-redhatio-nodeuser.json",
			nsCreds:   "testdata/credentials-redhatio-nsuser.json",
			expected: map[string]credentialprovider.DockerConfigEntry{
				"registry.redhat.io": {
					Username: "nsuser",
					Password: "nspass",
					Email:    "nsuser@redhat.com",
				},
			},
		},
		{
			name:      "invalid node credentials",
			nsCreds:   "testdata/credentials-quayio-user0.json",
			nodeCreds: "testdata/empty.txt",
			expected: map[string]credentialprovider.DockerConfigEntry{
				"quay.io": {
					Username: "user0",
					Password: "pass0",
					Email:    "user0@redhat.com",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.nodeCreds != "" {
				origNodeCredentialsFile := nodeCredentialsFile
				nodeCredentialsFile = tt.nodeCreds
				defer func() {
					nodeCredentialsFile = origNodeCredentialsFile
				}()
			}

			cfg := mergeNodeCredentials(tt.nsCreds)

			if !reflect.DeepEqual(cfg.Auths, tt.expected) {
				t.Errorf("expected %+v, received: %+v", tt.expected, cfg.Auths)
			}
		})
	}
}

func TestMergeNodeCredentialsDockerAuth(t *testing.T) {
	for _, tt := range []struct {
		name      string
		nsCreds   string
		nodeCreds string
		expected  map[string]docker.AuthConfiguration
	}{
		{
			name:     "invalid namespace credentials file",
			nsCreds:  "testdata/empty.txt",
			expected: map[string]docker.AuthConfiguration{},
		},
		{
			name:     "empty namespace credentials",
			nsCreds:  "testdata/credentials-empty.json",
			expected: map[string]docker.AuthConfiguration{},
		},
		{
			name:    "valid namespace credentials",
			nsCreds: "testdata/credentials-quayio-user0.json",
			expected: map[string]docker.AuthConfiguration{
				"quay.io": {
					Username:      "user0",
					Password:      "pass0",
					Email:         "user0@redhat.com",
					ServerAddress: "quay.io",
				},
			},
		},
		{
			name:      "merge namespace with node credentials",
			nsCreds:   "testdata/credentials-quayio-user0.json",
			nodeCreds: "testdata/credentials-redhatio-nodeuser.json",
			expected: map[string]docker.AuthConfiguration{
				"quay.io": {
					Username:      "user0",
					Password:      "pass0",
					Email:         "user0@redhat.com",
					ServerAddress: "quay.io",
				},
				"registry.redhat.io": {
					Username:      "nodeuser",
					Password:      "nodepass",
					Email:         "nodeuser@redhat.com",
					ServerAddress: "registry.redhat.io",
				},
			},
		},
		{
			name:      "overwriting node credentials",
			nodeCreds: "testdata/credentials-redhatio-nodeuser.json",
			nsCreds:   "testdata/credentials-redhatio-nsuser.json",
			expected: map[string]docker.AuthConfiguration{
				"registry.redhat.io": {
					Username:      "nsuser",
					Password:      "nspass",
					Email:         "nsuser@redhat.com",
					ServerAddress: "registry.redhat.io",
				},
			},
		},
		{
			name:      "invalid node credentials",
			nsCreds:   "testdata/credentials-quayio-user0.json",
			nodeCreds: "testdata/empty.txt",
			expected: map[string]docker.AuthConfiguration{
				"quay.io": {
					Username:      "user0",
					Password:      "pass0",
					Email:         "user0@redhat.com",
					ServerAddress: "quay.io",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.nodeCreds != "" {
				origNodeCredentialsFile := nodeCredentialsFile
				nodeCredentialsFile = tt.nodeCreds
				defer func() {
					nodeCredentialsFile = origNodeCredentialsFile
				}()
			}

			cfg := mergeNodeCredentialsDockerAuth(tt.nsCreds)
			if cfg == nil || cfg.Configs == nil {
				if len(tt.expected) > 0 {
					t.Errorf("expected %+v, received nil", tt.expected)
				}
				return
			}

			if !reflect.DeepEqual(cfg.Configs, tt.expected) {
				t.Errorf("expected %+v, received: %+v", tt.expected, cfg.Configs)
			}
		})
	}
}

func TestParseDropCapabilities(t *testing.T) {
	tests := map[string][]string{
		"SYS_ADMIN": {"CAP_SYS_ADMIN"},
		"cap_chown,dac_override,cap_dac_read_search,FOWNER,CAP_FSETID": {"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_DAC_READ_SEARCH", "CAP_FOWNER", "CAP_FSETID"},
	}
	preserveEnv, preserveSet := os.LookupEnv(builderutil.DropCapabilities)
	for input, expected := range tests {
		if err := os.Setenv(builderutil.DropCapabilities, input); err != nil {
			t.Errorf("%s: %v", input, err)
			continue
		}
		actual := dropCapabilities()
		if strings.Join(actual, ";") != strings.Join(expected, ";") {
			t.Errorf("%s: expected %v, got %v", input, expected, actual)
		}
	}
	if preserveSet {
		os.Setenv(builderutil.DropCapabilities, preserveEnv)
	} else {
		os.Unsetenv(builderutil.DropCapabilities)
	}
}

func TestAppendCATrustMount(t *testing.T) {
	cases := []struct {
		name        string
		envVar      string
		expectMount bool
	}{
		{
			name: "not set",
		},
		{
			name:        "set env var true",
			envVar:      "true",
			expectMount: true,
		},
		{
			name:        "set env var false",
			envVar:      "false",
			expectMount: false,
		},
		{
			name:        "bad env var",
			envVar:      "foo",
			expectMount: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			currentVal, isSet := os.LookupEnv("BUILD_MOUNT_ETC_PKI_CATRUST")
			if !isSet {
				defer os.Unsetenv("BUILD_MOUNT_ETC_PKI_CATRUST")
			} else {
				defer os.Setenv("BUILD_MOUNT_ETC_PKI_CATRUST", currentVal)
			}
			if len(tc.envVar) > 0 {
				os.Setenv("BUILD_MOUNT_ETC_PKI_CATRUST", tc.envVar)
			}

			// If stat fails in our test environment, always expect the function to not mount
			_, err := os.Stat("/etc/pki/ca-trust")
			if err != nil {
				tc.expectMount = false
			}
			mounts := []string{}
			mounts = appendCATrustMount(mounts)

			if tc.expectMount && len(mounts) == 0 {
				t.Fatal("expected mount for /etc/pki/ca-trust")
			}
			expectedMount := "/etc/pki/ca-trust:/etc/pki/ca-trust:ro"
			if tc.expectMount && mounts[0] != expectedMount {
				t.Errorf("expected mount %q, got %q", expectedMount, mounts[0])
			}
		})
	}
}
