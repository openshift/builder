package builder

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/credentialprovider"

	builderutil "github.com/openshift/builder/pkg/build/builder/util"
)

func Test_mergeNodeCredentials(t *testing.T) {
	for _, tt := range []struct {
		name      string
		nsCreds   string
		nodeCreds string
		errstr    string
		expected  credentialprovider.DockerConfig
	}{
		{
			name:   "invalid namespace credentials file path",
			errstr: "no such file or directory",
		},
		{
			name:    "invalid namespace credentials file",
			nsCreds: "testdata/empty.txt",
			errstr:  "unexpected end of JSON input",
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

			cfg, err := mergeNodeCredentials(tt.nsCreds)
			if err != nil {
				if tt.errstr == "" || !strings.Contains(err.Error(), tt.errstr) {
					t.Errorf("unexpected error: %v", err)
					return
				}
				return
			} else if tt.errstr != "" {
				t.Errorf("expected error %q, nil received instead", tt.errstr)
				return
			}

			if !reflect.DeepEqual(cfg.Auths, tt.expected) {
				t.Errorf("expected %+v, received: %+v", tt.expected, cfg.Auths)
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
