package main

import "testing"

func Test_areEqual(t *testing.T) {
	type args struct {
		arr1 []string
		arr2 []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Same order",
			args: args{
				arr1: []string{"abc", "def", "ghi"},
				arr2: []string{"abc", "def", "ghi"},
			},
			want: true,
		},
		{
			name: "Different order",
			args: args{
				arr1: []string{"abc", "def", "ghi"},
				arr2: []string{"def", "ghi", "abc"},
			},
			want: true,
		},
		{
			name: "Not same",
			args: args{
				arr1: []string{"abc", "def", "ghi"},
				arr2: []string{"123", "def", "ghi"},
			},
			want: false,
		},
		{
			name: "Arr1 nil",
			args: args{
				arr1: nil,
				arr2: []string{"123", "def", "ghi"},
			},
			want: false,
		},
		{
			name: "Arr2 nil",
			args: args{
				arr1: []string{"123", "def", "ghi"},
				arr2: nil,
			},
			want: false,
		},
		{
			name: "Both nil",
			args: args{
				arr1: nil,
				arr2: nil,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := areEqual(tt.args.arr1, tt.args.arr2); got != tt.want {
				t.Errorf("areEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
