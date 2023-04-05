package builder

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeEnv(t *testing.T) {
	tests := []struct {
		oldEnv   []string
		newEnv   []string
		expected []string
	}{
		{
			oldEnv:   []string{"one=1", "two=2"},
			newEnv:   []string{"three=3", "four=4"},
			expected: []string{"one=1", "two=2", "three=3", "four=4"},
		},
		{
			oldEnv:   []string{"one=1", "two=2", "four=4"},
			newEnv:   []string{"three=3", "four=4=5=6"},
			expected: []string{"one=1", "two=2", "three=3", "four=4=5=6"},
		},
		{
			oldEnv:   []string{"one=1", "two=2", "three=3"},
			newEnv:   []string{"two=002", "four=4"},
			expected: []string{"one=1", "two=002", "three=3", "four=4"},
		},
		{
			oldEnv:   []string{"one=1", "=2"},
			newEnv:   []string{"=3", "two=2"},
			expected: []string{"one=1", "=3", "two=2"},
		},
		{
			oldEnv:   []string{"one=1", "two"},
			newEnv:   []string{"two=2", "three=3"},
			expected: []string{"one=1", "two=2", "three=3"},
		},
	}
	for _, tc := range tests {
		result := MergeEnv(tc.oldEnv, tc.newEnv)
		toCheck := map[string]struct{}{}
		for _, e := range tc.expected {
			toCheck[e] = struct{}{}
		}
		for _, e := range result {
			if _, exists := toCheck[e]; !exists {
				t.Errorf("old = %s, new = %s: %s not expected in result",
					strings.Join(tc.oldEnv, ","), strings.Join(tc.newEnv, ","), e)
				continue
			}
			delete(toCheck, e)
		}
		if len(toCheck) > 0 {
			t.Errorf("old = %s, new = %s: did not get expected values in result: %#v",
				strings.Join(tc.oldEnv, ","), strings.Join(tc.newEnv, ","), toCheck)
		}
	}
}
func TestNameForBuildVolume(t *testing.T) {
	type args struct {
		objName string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Secret One",
			args: args{objName: "secret-one"},
			want: fmt.Sprintf("secret-one-%s", buildVolumeSuffix),
		},
		{
			name: "ConfigMap One",
			args: args{objName: "configmap-one"},
			want: fmt.Sprintf("configmap-one-%s", buildVolumeSuffix),
		},
		{
			name: "Greater than 47 characters",
			args: args{objName: "build-volume-larger-than-47-characters-but-less-than-63"},
			want: fmt.Sprintf("build-volume-larger-than-47-characte-8c2b6813-%s", buildVolumeSuffix),
		},
		{
			name: "Should convert to lowercase",
			args: args{objName: "Secret-One"},
			want: fmt.Sprintf("secret-one-%s", buildVolumeSuffix),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NameForBuildVolume(tt.args.objName); got != tt.want {
				t.Errorf("NameForBuildVolume() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathForBuildVolume(t *testing.T) {
	type args struct {
		objName string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Secret One",
			args: args{"secret-one"},
			want: filepath.Join(buildVolumeMountPath, fmt.Sprintf("secret-one-%s", buildVolumeSuffix)),
		},
		{
			name: "ConfigMap One",
			args: args{"configmap-one"},
			want: filepath.Join(buildVolumeMountPath, fmt.Sprintf("configmap-one-%s", buildVolumeSuffix)),
		},
		{
			name: "Greater than 47 characters",
			args: args{objName: "build-volume-larger-than-47-characters-but-less-than-63"},
			want: filepath.Join(buildVolumeMountPath, fmt.Sprintf("build-volume-larger-than-47-characte-8c2b6813-%s", buildVolumeSuffix)),
		},
		{
			name: "Should convert to lowercase",
			args: args{"Secret-One"},
			want: filepath.Join(buildVolumeMountPath, fmt.Sprintf("secret-one-%s", buildVolumeSuffix)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PathForBuildVolume(tt.args.objName); got != tt.want {
				t.Errorf("PathForBuildVolume() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeRegistryLocation(t *testing.T) {
	tests := []struct{ have, want string }{
		{"", ""},
		{"https://docker.io/my-namespace/my-user/my-image", "docker.io/my-namespace/my-user/my-image"},
		{"https://docker.io/my-namespace", "docker.io/my-namespace"},
		{"my-registry.local", "my-registry.local"},
		{"my-registry.local/username", "my-registry.local/username"},
		{"my-registry.local/v1/username", "my-registry.local/v1/username"},
		{"my-registry.local/v2/username", "my-registry.local/v2/username"},
		{"http://my-registry.local/username/", "my-registry.local/username"},
		{"http://my-registry.local/username", "my-registry.local/username"},
		{"docker.io/library/alpine", "docker.io/library/alpine"},
		{"docker.io", "docker.io"},
		{"quay.io/openshift/art", "quay.io/openshift/art"},
		{"quay.io/openshift", "quay.io/openshift"},
		{"quay.io", "quay.io"},
		{"quay.io/a/b/c", "quay.io/a/b/c"},
		{"https://registry-1.docker.io/v2/", "docker.io"},
		{"https://registry-1.docker.io/v2", "docker.io"},
		{"http://registry-1.docker.io/v1/", "docker.io"},
		{"http://registry-1.docker.io/v1", "docker.io"},
		{"https://registry-1.docker.io/", "docker.io"},
		{"https://registry-1.docker.io", "docker.io"},
		{"http://registry-1.docker.io/", "docker.io"},
		{"http://registry-1.docker.io", "docker.io"},
		{"https://registry-1.docker.io/v2/a", "docker.io/a"},
		{"https://registry-1.docker.io/v1/b", "docker.io/b"},
		{"http://registry-1.docker.io/v2/c", "docker.io/c"},
		{"http://registry-1.docker.io/v1/d", "docker.io/d"},
		{"https://index.docker.io/v2/", "docker.io"},
		{"https://index.docker.io/v2", "docker.io"},
		{"https://index.docker.io/v1/", "docker.io"},
		{"https://index.docker.io/v1", "docker.io"},
		{"http://index.docker.io/v2/", "docker.io"},
		{"http://index.docker.io/v2", "docker.io"},
		{"http://index.docker.io/v1/", "docker.io"},
		{"http://index.docker.io/v1", "docker.io"},
		{"https://index.docker.io/v2/a", "docker.io/a"},
		{"https://index.docker.io/v1/b", "docker.io/b"},
		{"http://index.docker.io/v2/c", "docker.io/c"},
		{"http://index.docker.io/v1/d", "docker.io/d"},
		{"https://quay.io/v2/", "quay.io"},
		{"https://quay.io/v2/a", "quay.io/a"},
		{"https://quay.io/v2", "quay.io"},
		{"http://quay.io/v2/", "quay.io"},
		{"http://quay.io/v2", "quay.io"},
		{"https://quay.io/v1/", "quay.io"},
		{"https://quay.io/v1", "quay.io"},
		{"http://quay.io/v1/", "quay.io"},
		{"http://quay.io/v1/b", "quay.io/b"},
		{"http://quay.io/v1", "quay.io"},
	}
	for _, tt := range tests {
		t.Run(tt.have, func(t *testing.T) {
			if got := normalizeRegistryLocation(tt.have); got != tt.want {
				t.Errorf("normalizeRegistryLocation(%q) = %q, expected %q", tt.have, got, tt.want)
			}
		})
	}
}
