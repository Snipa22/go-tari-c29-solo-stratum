package minerTracking

import (
	"fmt"
	"github.com/Snipa22/go-tari-grpc-lib/v3/tari_generated"
	"reflect"
	"sync"
	"testing"
)

func TestMinerJob_diffToTarget(t *testing.T) {
	type fields struct {
		BlockResult *tari_generated.GetNewBlockResult
		UsedNonces  []uint64
		Target      uint64
		NonceMutex  sync.RWMutex
	}
	tests := []struct {
		name    string
		fields  fields
		want    uint64
		wantErr bool
	}{
		{name: "Diff Target 100000000399",
			fields: fields{
				Target: 100000000399},
			want: uint64(184467440),
		},
		{name: "Diff Target 300000000000",
			fields: fields{
				Target: 300000000000},
			want: uint64(61489146),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &MinerJob{
				BlockResult: tt.fields.BlockResult,
				UsedNonces:  tt.fields.UsedNonces,
				Target:      tt.fields.Target,
				NonceMutex:  tt.fields.NonceMutex,
			}
			got := job.diffToTarget()
			fmt.Printf("%016x\n", got)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("diffToTarget() got = %v, want %v", got, tt.want)
			}
		})
	}
}
