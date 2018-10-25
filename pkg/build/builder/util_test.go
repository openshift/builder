package builder

import (
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
