package tasks

import "testing"

type ResultFlag bool

const (
	Reserve ResultFlag = true
	Skip    ResultFlag = false
)

type Want struct {
	Name   string
	Result ResultFlag // false if skip
}

func TestGetSkipComponentsForVirtualCluster(t *testing.T) {
	tests := []struct {
		name      string
		input     []*SkipComponentCondition
		want      []Want
		skipCount int
	}{
		{
			name: "test-single",
			input: []*SkipComponentCondition{
				{
					Condition:     true,
					ComponentName: "skip-1",
				},
			},
			want: []Want{
				{
					Name:   "skip-1",
					Result: Skip,
				},
			},
			skipCount: 1,
		},
		{
			name: "test-double",
			input: []*SkipComponentCondition{
				{
					Condition:     true,
					ComponentName: "skip-1",
				},
				{
					Condition:     true,
					ComponentName: "skip-2",
				},
			},
			want: []Want{
				{
					Name:   "skip-1",
					Result: Skip,
				},
				{
					Name:   "skip-2",
					Result: Skip,
				},
			},
			skipCount: 2,
		},
		{
			name: "test-middle",
			input: []*SkipComponentCondition{
				{
					Condition:     true,
					ComponentName: "skip-1",
				},
				{
					Condition:     false,
					ComponentName: "skip-2",
				},
				{
					Condition:     true,
					ComponentName: "skip-3",
				},
			},
			want: []Want{
				{
					Name:   "skip-1",
					Result: Skip,
				},
				{
					Name:   "skip-2",
					Result: Reserve,
				},
				{
					Name:   "skip-3",
					Result: Skip,
				},
			},
			skipCount: 2,
		},
		{
			name: "test-all-reserve",
			input: []*SkipComponentCondition{
				{
					Condition:     false,
					ComponentName: "skip-1",
				},
				{
					Condition:     false,
					ComponentName: "skip-2",
				},
				{
					Condition:     false,
					ComponentName: "skip-3",
				},
			},
			want: []Want{
				{
					Name:   "skip-1",
					Result: Reserve,
				},
				{
					Name:   "skip-2",
					Result: Reserve,
				},
				{
					Name:   "skip-3",
					Result: Reserve,
				},
			},
			skipCount: 0,
		},
		{
			name: "test-all-skip",
			input: []*SkipComponentCondition{
				{
					Condition:     true,
					ComponentName: "skip-1",
				},
				{
					Condition:     true,
					ComponentName: "skip-2",
				},
				{
					Condition:     true,
					ComponentName: "skip-3",
				},
			},
			want: []Want{
				{
					Name:   "skip-1",
					Result: Skip,
				},
				{
					Name:   "skip-2",
					Result: Skip,
				},
				{
					Name:   "skip-3",
					Result: Skip,
				},
			},
			skipCount: 3,
		},
		{
			name: "test-first-skip",
			input: []*SkipComponentCondition{
				{
					Condition:     true,
					ComponentName: "skip-1",
				},
				{
					Condition:     false,
					ComponentName: "skip-2",
				},
				{
					Condition:     false,
					ComponentName: "skip-3",
				},
			},
			want: []Want{
				{
					Name:   "skip-1",
					Result: Skip,
				},
				{
					Name:   "skip-2",
					Result: Reserve,
				},
				{
					Name:   "skip-3",
					Result: Reserve,
				},
			},
			skipCount: 1,
		},
		{
			name: "test-big-data",
			input: []*SkipComponentCondition{
				{
					Condition:     true,
					ComponentName: "skip-1",
				},
				{
					Condition:     false,
					ComponentName: "skip-2",
				},
				{
					Condition:     false,
					ComponentName: "skip-3",
				},
				{
					Condition:     false,
					ComponentName: "skip-4",
				},
				{
					Condition:     false,
					ComponentName: "skip-5",
				},
				{
					Condition:     false,
					ComponentName: "skip-6",
				},
				{
					Condition:     false,
					ComponentName: "skip-7",
				},
				{
					Condition:     false,
					ComponentName: "skip-8",
				},
				{
					Condition:     false,
					ComponentName: "skip-9",
				},
				{
					Condition:     false,
					ComponentName: "skip-10",
				},
			},
			want: []Want{
				{
					Name:   "skip-1",
					Result: Skip,
				},
				{
					Name:   "skip-2",
					Result: Reserve,
				},
				{
					Name:   "skip-3",
					Result: Reserve,
				},
				{
					Name:   "skip-4",
					Result: Reserve,
				},
				{
					Name:   "skip-5",
					Result: Reserve,
				},
				{
					Name:   "skip-6",
					Result: Reserve,
				},
				{
					Name:   "skip-7",
					Result: Reserve,
				},
				{
					Name:   "skip-8",
					Result: Reserve,
				},
				{
					Name:   "skip-9",
					Result: Reserve,
				},
				{
					Name:   "skip-10",
					Result: Reserve,
				},
			},
			skipCount: 1,
		},
		{
			name: "test-big-data",
			input: []*SkipComponentCondition{
				{
					Condition:     true,
					ComponentName: "skip-1",
				},
				{
					Condition:     false,
					ComponentName: "skip-2",
				},
				{
					Condition:     false,
					ComponentName: "skip-3",
				},
				{
					Condition:     false,
					ComponentName: "skip-4",
				},
				{
					Condition:     false,
					ComponentName: "skip-5",
				},
				{
					Condition:     false,
					ComponentName: "skip-6",
				},
				{
					Condition:     true,
					ComponentName: "skip-7",
				},
				{
					Condition:     true,
					ComponentName: "skip-8",
				},
				{
					Condition:     true,
					ComponentName: "skip-9",
				},
			},
			want: []Want{
				{
					Name:   "skip-1",
					Result: Skip,
				},
				{
					Name:   "skip-2",
					Result: Reserve,
				},
				{
					Name:   "skip-3",
					Result: Reserve,
				},
				{
					Name:   "skip-4",
					Result: Reserve,
				},
				{
					Name:   "skip-5",
					Result: Reserve,
				},
				{
					Name:   "skip-6",
					Result: Reserve,
				},
				{
					Name:   "skip-7",
					Result: Skip,
				},
				{
					Name:   "skip-8",
					Result: Skip,
				},
				{
					Name:   "skip-9",
					Result: Skip,
				},
			},
			skipCount: 4,
		},
		{
			name: "test-big-data",
			input: []*SkipComponentCondition{
				{
					Condition:     true,
					ComponentName: "skip-1",
				},
				{
					Condition:     false,
					ComponentName: "skip-2",
				},
				{
					Condition:     false,
					ComponentName: "skip-3",
				},
				{
					Condition:     false,
					ComponentName: "skip-4",
				},
				{
					Condition:     false,
					ComponentName: "skip-5",
				},
				{
					Condition:     false,
					ComponentName: "skip-6",
				},
				{
					Condition:     true,
					ComponentName: "skip-7",
				},
				{
					Condition:     true,
					ComponentName: "skip-8",
				},
				{
					Condition:     true,
					ComponentName: "skip-9",
				},
				{
					Condition:     true,
					ComponentName: "skip-10",
				},
				{
					Condition:     true,
					ComponentName: "skip-11",
				},
			},
			want: []Want{
				{
					Name:   "skip-1",
					Result: Skip,
				},
				{
					Name:   "skip-2",
					Result: Reserve,
				},
				{
					Name:   "skip-3",
					Result: Reserve,
				},
				{
					Name:   "skip-4",
					Result: Reserve,
				},
				{
					Name:   "skip-5",
					Result: Reserve,
				},
				{
					Name:   "skip-6",
					Result: Reserve,
				},
				{
					Name:   "skip-7",
					Result: Skip,
				},
				{
					Name:   "skip-8",
					Result: Skip,
				},
				{
					Name:   "skip-9",
					Result: Skip,
				},
				{
					Name:   "skip-10",
					Result: Skip,
				},
				{
					Name:   "skip-11",
					Result: Skip,
				},
			},
			skipCount: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipComponents := getSkipComponentsForVirtualCluster(tt.input)
			count := 0
			for _, want := range tt.want {
				if v, ok := skipComponents[want.Name]; ok && v {
					count++
					continue
				}
				if !want.Result {
					t.Errorf("getSkipComponentsForVirtualCluster() name: %v, want %v", want.Name, want.Result)
				}
			}
			if count != tt.skipCount {
				t.Errorf("getSkipComponentsForVirtualCluster() name: %v, want %v", count, tt.skipCount)
			}
		})
	}
}
