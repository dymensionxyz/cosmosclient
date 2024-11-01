package cosmosclient

import "testing"

func Test_packPubKey(t *testing.T) {
	type args struct {
		key OverridePubKey
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Test PackPubKey: ed25519",
			args: args{
				key: OverridePubKey{
					Name:   "sequencer",
					PubKey: "EGKQqTMRD0AWBgbhGwNfYK+c3W1P+TrcYaJHcfYREdo=",
					Type:   "ed25519",
				},
			},
		}, {
			name: "Test PackPubKey: ethsecp256k1",
			args: args{
				key: OverridePubKey{
					Name:   "sequencer",
					PubKey: "ApclyR9Az3kI7VEMXuscnffA4Gf5N6Mgq/VcmOqvjkUf",
					Type:   "ethsecp256k1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			PackPubKey(tt.args.key.PubKey, tt.args.key.Type)
		})
	}
}
