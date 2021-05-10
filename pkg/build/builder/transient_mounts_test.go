package builder

import (
	"reflect"
	"testing"
)

func TestTransientMountsMap_append(t *testing.T) {
	tests := []struct {
		name       string
		mounts     []TransientMount
		want       TransientMounts
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should succeed",
			mounts: []TransientMount{
				{
					Source:      "/volume/test1",
					Destination: "/test1",
					Options:     TransientMountOptions{},
				},
				{
					Source:      "/volume/test2",
					Destination: "/test2",
					Options:     TransientMountOptions{},
				},
				{
					Source:      "/volume/test3",
					Destination: "/test3",
					Options:     TransientMountOptions{},
				},
			},
			want: TransientMounts{
				"/test1": {
					Source:      "/volume/test1",
					Destination: "/test1",
					Options:     TransientMountOptions{},
				},
				"/test2": {
					Source:      "/volume/test2",
					Destination: "/test2",
					Options:     TransientMountOptions{},
				},
				"/test3": {
					Source:      "/volume/test3",
					Destination: "/test3",
					Options:     TransientMountOptions{},
				},
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "duplicate mount should fail with error message",
			mounts: []TransientMount{
				{
					Source:      "/volume/test1",
					Destination: "/test1",
					Options:     TransientMountOptions{},
				},
				{
					Source:      "/volume/test2",
					Destination: "/test1",
					Options:     TransientMountOptions{},
				},
			},
			want: TransientMounts{
				"/test1": {
					Source:      "/volume/test1",
					Destination: "/test1",
					Options:     TransientMountOptions{},
				},
			},
			wantErr:    true,
			wantErrMsg: "duplicate transient mount destination detected, \"/volume/test2:/test1\" already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mountsMap := make(TransientMounts)

			var err error

			for _, m := range tt.mounts {
				err = mountsMap.append(m)
				if err != nil {
					break
				}

			}
			if !tt.wantErr && err != nil {
				t.Errorf("should not have encountered an error but did anyways: %v", err)
			}
			if tt.wantErr && err == nil {
				t.Errorf("should have produced an error but didn't")
			}
			if tt.wantErr && err != nil && tt.wantErrMsg != err.Error() {
				t.Errorf("incorrect error message, wanted: %v, got %v", tt.wantErrMsg, err.Error())
			}
			if !reflect.DeepEqual(tt.want, mountsMap) {
				t.Errorf("appending mounts did not produce the correct result, wanted: %#v, got %#v", tt.want, mountsMap)
			}
		})
	}
}

func TestTransientMountsMap_asSlice(t *testing.T) {
	tests := []struct {
		name   string
		mounts []TransientMount
		want   []string
	}{
		{
			name: "should succeed",
			mounts: []TransientMount{
				{
					Source:      "/volume/test1",
					Destination: "/test1",
					Options:     TransientMountOptions{},
				},
				{
					Source:      "/volume/test2",
					Destination: "/test2",
					Options: TransientMountOptions{
						NoDev:  true,
						NoExec: true,
					},
				},
				{
					Source:      "/volume/test3",
					Destination: "/test3",
					Options: TransientMountOptions{
						NoDev:  false,
						NoExec: true,
					},
				},
			},
			want: []string{
				"/volume/test1:/test1",
				"/volume/test2:/test2:nodev,noexec",
				"/volume/test3:/test3:noexec",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mountsMap := make(TransientMounts)

			var err error

			for _, m := range tt.mounts {
				err = mountsMap.append(m)
				if err != nil {
					break
				}

			}
			gotMounts := mountsMap.asSlice()

			if !reflect.DeepEqual(tt.want, gotMounts) {
				t.Errorf("TransientMountsMap.asSlice() did not output correct data, wanted: %#v, got %#v", tt.want, gotMounts)
			}
		})
	}
}

func TestTransientMount_String(t *testing.T) {
	type fields struct {
		Source      string
		Destination string
		Options     TransientMountOptions
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "mount with all true options",
			fields: fields{
				Source:      "/volumes/test1",
				Destination: "/test1",
				Options: TransientMountOptions{
					NoDev:  true,
					NoExec: true,
					NoSuid: true,
				},
			},
			want: "/volumes/test1:/test1:nodev,noexec,nosuid",
		},
		{
			name: "mount with all false",
			fields: fields{
				Source:      "/volumes/test1",
				Destination: "/test1",
				Options: TransientMountOptions{
					NoDev:  false,
					NoExec: false,
					NoSuid: false,
				},
			},
			want: "/volumes/test1:/test1",
		},
		{
			name: "mount with no options specified",
			fields: fields{
				Source:      "/volumes/test1",
				Destination: "/test1",
			},
			want: "/volumes/test1:/test1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := TransientMount{
				Source:      tt.fields.Source,
				Destination: tt.fields.Destination,
				Options:     tt.fields.Options,
			}
			if got := tm.String(); got != tt.want {
				t.Errorf("TransientMount.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
