package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anryton/anryton/v2/x/wasm/types"
)

func TestParseAccessConfigUpdates(t *testing.T) {
	specs := map[string]struct {
		src    []string
		exp    []types.AccessConfigUpdate
		expErr bool
	}{
		"nobody": {
			src: []string{"1:nobody"},
			exp: []types.AccessConfigUpdate{{
				CodeID:                1,
				InstantiatePermission: types.AccessConfig{Permission: types.AccessTypeNobody},
			}},
		},
		"everybody": {
			src: []string{"1:everybody"},
			exp: []types.AccessConfigUpdate{{
				CodeID:                1,
				InstantiatePermission: types.AccessConfig{Permission: types.AccessTypeEverybody},
			}},
		},
		"any of addresses - single": {
			src: []string{"1:cosmos1vx8knpllrj7n963p9ttd80w47kpacrhuts497x"},
			exp: []types.AccessConfigUpdate{
				{
					CodeID: 1,
					InstantiatePermission: types.AccessConfig{
						Permission: types.AccessTypeAnyOfAddresses,
						Addresses:  []string{"cosmos1vx8knpllrj7n963p9ttd80w47kpacrhuts497x"},
					},
				},
			},
		},
		"any of addresses - multiple": {
			src: []string{"1:cosmos1vx8knpllrj7n963p9ttd80w47kpacrhuts497x,cosmos14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9s4hmalr"},
			exp: []types.AccessConfigUpdate{
				{
					CodeID: 1,
					InstantiatePermission: types.AccessConfig{
						Permission: types.AccessTypeAnyOfAddresses,
						Addresses:  []string{"cosmos1vx8knpllrj7n963p9ttd80w47kpacrhuts497x", "cosmos14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9s4hmalr"},
					},
				},
			},
		},
		"multiple code ids with different permissions": {
			src: []string{"1:cosmos1vx8knpllrj7n963p9ttd80w47kpacrhuts497x,cosmos14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9s4hmalr", "2:nobody"},
			exp: []types.AccessConfigUpdate{
				{
					CodeID: 1,
					InstantiatePermission: types.AccessConfig{
						Permission: types.AccessTypeAnyOfAddresses,
						Addresses:  []string{"cosmos1vx8knpllrj7n963p9ttd80w47kpacrhuts497x", "cosmos14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9s4hmalr"},
					},
				}, {
					CodeID: 2,
					InstantiatePermission: types.AccessConfig{
						Permission: types.AccessTypeNobody,
					},
				},
			},
		},
		"any of addresses - empty list": {
			src:    []string{"1:"},
			expErr: true,
		},
		"any of addresses - invalid address": {
			src:    []string{"1:foo"},
			expErr: true,
		},
		"any of addresses - duplicate address": {
			src:    []string{"1:cosmos1vx8knpllrj7n963p9ttd80w47kpacrhuts497x,cosmos1vx8knpllrj7n963p9ttd80w47kpacrhuts497x"},
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			got, gotErr := parseAccessConfigUpdates(spec.src)
			if spec.expErr {
				require.Error(t, gotErr)
				return
			}
			require.NoError(t, gotErr)
			assert.Equal(t, spec.exp, got)
		})
	}
}

func TestParseCodeInfoFlags(t *testing.T) {
	correctSource := "https://github.com/anryton/anryton/v2/blob/main/x/wasm/keeper/testdata/hackatom.wasm"
	correctBuilderRef := "cosmwasm/workspace-optimizer:0.12.9"

	wasmBin, err := os.ReadFile("../../keeper/testdata/hackatom.wasm.gzip")
	require.NoError(t, err)

	checksumStr := "5ca46abb8e9b1b754a5c906f9c0f4eec9121ee09e3cee55ea0faba54763706e2"

	specs := map[string]struct {
		args   []string
		expErr bool
	}{
		"source missing": {
			args:   []string{"--builder=" + correctBuilderRef, "--code-hash=" + checksumStr},
			expErr: true,
		},
		"builder missing": {
			args:   []string{"--code-source-url=" + correctSource, "--code-hash=" + checksumStr},
			expErr: true,
		},
		"code hash missing": {
			args:   []string{"--code-source-url=" + correctSource, "--builder=" + correctBuilderRef},
			expErr: true,
		},
		"source format wrong": {
			args:   []string{"--code-source-url=" + "format_wrong", "--builder=" + correctBuilderRef, "--code-hash=" + checksumStr},
			expErr: true,
		},
		"builder format wrong": {
			args:   []string{"--code-source-url=" + correctSource, "--builder=" + "format//", "--code-hash=" + checksumStr},
			expErr: true,
		},
		"code hash wrong": {
			args:   []string{"--code-source-url=" + correctSource, "--builder=" + correctBuilderRef, "--code-hash=" + "AA"},
			expErr: true,
		},
		"happy path, none set": {
			args:   []string{},
			expErr: false,
		},
		"happy path all set": {
			args:   []string{"--code-source-url=" + correctSource, "--builder=" + correctBuilderRef, "--code-hash=" + checksumStr},
			expErr: false,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			flags := ProposalStoreAndInstantiateContractCmd().Flags()
			require.NoError(t, flags.Parse(spec.args))
			_, _, _, gotErr := parseVerificationFlags(wasmBin, flags)
			if spec.expErr {
				require.Error(t, gotErr)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}