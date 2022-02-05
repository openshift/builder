package cmd

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildv1 "github.com/openshift/api/build/v1"

	"github.com/openshift/builder/test/unit/fake"
)

func TestSetupProxyConfig(t *testing.T) {
	fakeHTTPProxy := "http://proxy.io"
	fakeHTTPSProxy := "https://proxy.io"

	cases := []struct {
		name                  string
		expectError           bool
		build                 *buildv1.Build
		inputGitConfig        string
		expectedConfigs       map[string]string
		expectedLocalConfigs  map[string]string
		expectedGlobalConfigs map[string]string
	}{
		{
			name:  "no git",
			build: &buildv1.Build{},
		},
		{
			name: "git clone no proxy no auth",
			build: &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build-1",
					Namespace: "test",
				},
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Source: buildv1.BuildSource{
							Type: buildv1.BuildSourceGit,
							Git: &buildv1.GitBuildSource{
								URI: "https://githost.dev/myorg/myrepo.git",
							},
						},
					},
				},
			},
		},
		{
			name: "git clone proxy no auth",
			build: &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build-1",
					Namespace: "test",
				},
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Source: buildv1.BuildSource{
							Type: buildv1.BuildSourceGit,
							Git: &buildv1.GitBuildSource{
								URI: "https://githost.dev/myorg/myrepo.git",
								ProxyConfig: buildv1.ProxyConfig{
									HTTPProxy:  &fakeHTTPProxy,
									HTTPSProxy: &fakeHTTPSProxy,
								},
							},
						},
					},
				},
			},
			expectedGlobalConfigs: map[string]string{
				"http.proxy":  fakeHTTPProxy,
				"https.proxy": fakeHTTPSProxy,
			},
		},
		{
			name: "git clone no proxy auth",
			build: &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build-1",
					Namespace: "test",
				},
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Source: buildv1.BuildSource{
							Type: buildv1.BuildSourceGit,
							Git: &buildv1.GitBuildSource{
								URI: "https://githost.dev/myorg/myrepo.git",
							},
							SourceSecret: &corev1.LocalObjectReference{
								Name: "git-auth",
							},
						},
					},
				},
			},
			inputGitConfig: ".gitconfig.test",
		},
		{
			name: "git clone proxy auth",
			build: &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build-1",
					Namespace: "test",
				},
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Source: buildv1.BuildSource{
							Type: buildv1.BuildSourceGit,
							Git: &buildv1.GitBuildSource{
								URI: "https://githost.dev/myorg/myrepo.git",
								ProxyConfig: buildv1.ProxyConfig{
									HTTPProxy:  &fakeHTTPProxy,
									HTTPSProxy: &fakeHTTPSProxy,
								},
							},
							SourceSecret: &corev1.LocalObjectReference{
								Name: "git-auth",
							},
						},
					},
				},
			},
			inputGitConfig: ".gitconfig.test",
			expectedConfigs: map[string]string{
				"http.proxy":  fakeHTTPProxy,
				"https.proxy": fakeHTTPSProxy,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			builder := &builderConfig{}
			builder.build = tc.build
			gitClient := fake.NewFakeGitRepository()
			err := builder.setupProxyConfig(gitClient, tc.inputGitConfig)
			if err != nil && !tc.expectError {
				t.Errorf("received unexpected error: %v", err)
			}
			if err == nil && tc.expectError {
				t.Error("expected to receive error, got none")
			}

			if len(tc.expectedConfigs) != len(gitClient.Configs) {
				t.Errorf("expected default configs to have %d entries, got %d", len(tc.expectedConfigs), len(gitClient.Configs))
			}

			if len(tc.expectedLocalConfigs) != len(gitClient.LocalConfigs) {
				t.Errorf("expected default configs to have %d entries, got %d", len(tc.expectedLocalConfigs), len(gitClient.LocalConfigs))
			}

			if len(tc.expectedGlobalConfigs) != len(gitClient.GlobalConfigs) {
				t.Errorf("expected global configs to have %d entries, got %d", len(tc.expectedGlobalConfigs), len(gitClient.GlobalConfigs))
			}

			verifyGitConfigs("default", tc.expectedConfigs, gitClient.Configs, t)
			verifyGitConfigs("local", tc.expectedLocalConfigs, gitClient.LocalConfigs, t)
			verifyGitConfigs("global", tc.expectedGlobalConfigs, gitClient.GlobalConfigs, t)
		})
	}
}

func verifyGitConfigs(name string, expectedMap, actualMap map[string]string, t *testing.T) {
	for config, expected := range expectedMap {
		actual, present := actualMap[config]
		if !present {
			t.Errorf("expected %s git configuration %s was not set", name, config)
		}
		if actual != expected {
			t.Errorf("expected %s git config %s to be %s, got %s", name, config, expected, actual)
		}
	}
}
