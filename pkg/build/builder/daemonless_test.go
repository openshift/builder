package builder

import (
	"os"
	"strings"
	"testing"

	builderutil "github.com/openshift/builder/pkg/build/builder/util"
)

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
