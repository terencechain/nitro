// Copyright 2021-2025, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE.md

package precompiles

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/offchainlabs/nitro/util/testhelpers"
)

func TestArbTimeBoost(t *testing.T) {
	evm := testhelpers.NewMockEVMForTesting()
	caller := testhelpers.RandomAddress()
	precompileCtx := testContext(caller, evm)

	arbTimeBoost := &ArbTimeBoost{Address: types.ArbTimeBoostAddress}

	// Test IsTimeBoosted method - should return false by default
	isTimeBoosted, err := arbTimeBoost.IsTimeBoosted(precompileCtx, evm)
	if err != nil {
		t.Fatalf("IsTimeBoosted failed: %v", err)
	}

	if isTimeBoosted {
		t.Error("Expected IsTimeBoosted to return false by default")
	}

	// Test with timeboosted transaction
	if precompileCtx.txProcessor != nil {
		precompileCtx.txProcessor.SetTimeBoosted(true)
		isTimeBoosted, err = arbTimeBoost.IsTimeBoosted(precompileCtx, evm)
		if err != nil {
			t.Fatalf("IsTimeBoosted failed: %v", err)
		}

		if !isTimeBoosted {
			t.Error("Expected IsTimeBoosted to return true when set")
		}
	}
}

func TestArbTimeBoostPrecompileRegistration(t *testing.T) {
	// Test that the precompile is properly registered
	precompiles := Precompiles()
	
	arbTimeBoostPrecompile, exists := precompiles[types.ArbTimeBoostAddress]
	if !exists {
		t.Fatal("ArbTimeBoost precompile not found in precompiles map")
	}

	if arbTimeBoostPrecompile == nil {
		t.Fatal("ArbTimeBoost precompile is nil")
	}

	// Verify the address is correct
	if arbTimeBoostPrecompile.Precompile().address != types.ArbTimeBoostAddress {
		t.Errorf("Expected address %v, got %v", types.ArbTimeBoostAddress, arbTimeBoostPrecompile.Precompile().address)
	}
}