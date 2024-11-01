package cosmosclient

import (
	"encoding/base64"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/evmos/evmos/v12/crypto/ethsecp256k1"
)

type keyringFake struct {
	keyring.Keyring
	pubKeyOverrides map[string]*types.Any
}

func (k keyringFake) Key(uid string) (*keyring.Record, error) {
	key, err := k.Keyring.Key(uid)
	if err != nil {
		return nil, err
	}

	pubKey := k.getPubKeyOverride(key.Name)
	if pubKey != nil {
		key.PubKey = pubKey
	}

	return key, nil
}

func (k keyringFake) KeyByAddress(addr sdk.Address) (*keyring.Record, error) {
	key, err := k.Keyring.KeyByAddress(addr)
	if err != nil {
		return nil, err
	}

	pubKey := k.getPubKeyOverride(key.Name)
	if pubKey != nil {
		key.PubKey = pubKey
	}

	return key, nil
}

func (k keyringFake) getPubKeyOverride(uid string) *types.Any {
	if k.pubKeyOverrides == nil {
		return nil
	}
	return k.pubKeyOverrides[uid]
}

func PackPubKey(encodedKey string, typ string) *types.Any {
	s, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		panic(fmt.Sprintf("failed to decode base64 PubKey: %v", err))
	}

	var key cryptotypes.PubKey
	switch typ {
	case "ed25519":
		key = &ed25519.PubKey{Key: s}
	case "ethsecp256k1":
		key = &ethsecp256k1.PubKey{Key: s}
	case "secp256k1":
		key = &secp256k1.PubKey{Key: s}
	default:
		panic(fmt.Sprintf("unknown pubkey type: %s", typ))
	}

	anyPubKey, err := codectypes.NewAnyWithValue(key)
	if err != nil {
		panic(fmt.Sprintf("failed to pack PubKey into Any: %v", err))
	}

	return anyPubKey
}
