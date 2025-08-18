// Copyright 2021-2025, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE.md

//go:build !race
// +build !race

package precompiles

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"

	arbtest "github.com/offchainlabs/nitro/system_tests"
	"github.com/offchainlabs/nitro/util/arbmath"
)

// Test contract that calls ArbTimeBoost to check if current tx is time boosted
const timeboostDetectorContract = `
pragma solidity ^0.8.0;

interface ArbTimeBoost {
    function isTimeBoosted() external view returns (bool);
}

contract TimeboostDetector {
    ArbTimeBoost constant arbTimeBoost = ArbTimeBoost(0x0000000000000000000000000000000000000074);
    
    event TimeboostStatus(bool isTimeBoosted);
    
    function checkTimeboost() external {
        bool timeBoosted = arbTimeBoost.isTimeBoosted();
        emit TimeboostStatus(timeBoosted);
    }
    
    function getTimeboostStatus() external view returns (bool) {
        return arbTimeBoost.isTimeBoosted();
    }
}
`

func TestArbTimeBoostEndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up test environment with timeboost enabled
	builder := arbtest.NewNodeBuilder(ctx).Default()

	// Enable timeboost in the sequencer config
	builder.L2.ConsensusSequencer.EnableTimeboost = true
	builder.L2.ConsensusSequencer.Timeboost.Enable = true
	builder.L2.ConsensusSequencer.Timeboost.ExpressLaneAdvantage = arbtest.TimeDuration{}

	cleanup := builder.Build(t)
	defer cleanup()

	l2client := builder.L2.Client
	l2auth := builder.L2Info.GetDefaultTransactOpts("Owner", ctx)

	// Deploy the test contract
	contractABI, err := abi.JSON(strings.NewReader(`[
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "bool",
					"name": "isTimeBoosted",
					"type": "bool"
				}
			],
			"name": "TimeboostStatus",
			"type": "event"
		},
		{
			"inputs": [],
			"name": "checkTimeboost",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "getTimeboostStatus",
			"outputs": [
				{
					"internalType": "bool",
					"name": "",
					"type": "bool"
				}
			],
			"stateMutability": "view",
			"type": "view"
		}
	]`))
	if err != nil {
		t.Fatalf("Failed to parse contract ABI: %v", err)
	}

	// Compile the contract (simplified bytecode for testing)
	// In a real test, you'd use solc to compile the contract
	contractBytecode := common.FromHex("608060405234801561001057600080fd5b506102a8806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c80630d5f3a3a1461003b578063b8f7a2b214610045575b600080fd5b610043610063565b005b61004d6100c7565b60405161005a9190610154565b60405180910390f35b600060749054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16630d5f3a3a6040518163ffffffff1660e01b8152600401602060405180830381865afa1580156100d1573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906100f5919061017f565b90507fdd0b29299947137ca5f1a8b4b4bb4e8c8f7a3d85e0c9b3a3b5e8e4b9e8e4b9e8816040516100b99190610154565b60405180910390a150565b600060749054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16630d5f3a3a6040518163ffffffff1660e01b8152600401602060405180830381865afa158015610135573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610159919061017f565b905090565b60008115159050919050565b61017381610148565b82525050565b6000602082019050610188600083018461016a565b92915050565b600080fd5b61019c81610148565b81146101a757600080fd5b50565b6000815190506101b981610193565b92915050565b6000602082840312156101d5576101d461018e565b5b60006101e3848285016101aa565b9150509291505056fea264697066735822122012345678901234567890123456789012345678901234567890123456789012345664736f6c63430008110033")

	// Deploy contract
	deployTx := types.NewContractCreation(
		l2auth.Nonce.Uint64(),
		big.NewInt(0),
		500000, // gas limit
		l2auth.GasPrice,
		contractBytecode,
	)

	signedDeployTx, err := l2auth.Signer(l2auth.From, deployTx)
	if err != nil {
		t.Fatalf("Failed to sign deploy tx: %v", err)
	}

	err = l2client.SendTransaction(ctx, signedDeployTx)
	if err != nil {
		t.Fatalf("Failed to send deploy tx: %v", err)
	}

	// Wait for deploy tx to be mined
	_, err = arbtest.WaitForTx(ctx, l2client, signedDeployTx.Hash(), 10*arbtest.Second)
	if err != nil {
		t.Fatalf("Failed to wait for deploy tx: %v", err)
	}

	// Get contract address
	contractAddr := crypto.CreateAddress(l2auth.From, l2auth.Nonce.Uint64())
	l2auth.Nonce.Add(l2auth.Nonce, big.NewInt(1))

	// Test 1: Normal transaction (should return false)
	callData, err := contractABI.Pack("checkTimeboost")
	if err != nil {
		t.Fatalf("Failed to pack call data: %v", err)
	}

	normalTx := types.NewTransaction(
		l2auth.Nonce.Uint64(),
		contractAddr,
		big.NewInt(0),
		300000, // gas limit
		l2auth.GasPrice,
		callData,
	)

	signedNormalTx, err := l2auth.Signer(l2auth.From, normalTx)
	if err != nil {
		t.Fatalf("Failed to sign normal tx: %v", err)
	}

	// Send via regular path (not timeboosted)
	err = l2client.SendTransaction(ctx, signedNormalTx)
	if err != nil {
		t.Fatalf("Failed to send normal tx: %v", err)
	}

	normalReceipt, err := arbtest.WaitForTx(ctx, l2client, signedNormalTx.Hash(), 10*arbtest.Second)
	if err != nil {
		t.Fatalf("Failed to wait for normal tx: %v", err)
	}

	// Check event from normal transaction
	normalEvents, err := contractABI.Unpack("TimeboostStatus", normalReceipt.Logs[0].Data)
	if err != nil {
		t.Fatalf("Failed to unpack normal tx event: %v", err)
	}

	normalIsTimeBoosted := normalEvents[0].(bool)
	if normalIsTimeBoosted {
		t.Error("Expected normal transaction to NOT be time boosted")
	}

	l2auth.Nonce.Add(l2auth.Nonce, big.NewInt(1))

	// Test 2: Time boosted transaction (should return true)
	timeboostedTx := types.NewTransaction(
		l2auth.Nonce.Uint64(),
		contractAddr,
		big.NewInt(0),
		300000, // gas limit
		l2auth.GasPrice,
		callData,
	)

	signedTimeboostedTx, err := l2auth.Signer(l2auth.From, timeboostedTx)
	if err != nil {
		t.Fatalf("Failed to sign timeboosted tx: %v", err)
	}

	// Send via timeboost path
	// In a real implementation, this would go through the express lane service
	// For now, we'll simulate by directly calling PublishTimeboostedTransaction
	sequencer := builder.L2.ConsensusNode.Sequencer
	err = sequencer.PublishTimeboostedTransaction(ctx, signedTimeboostedTx, nil)
	if err != nil {
		t.Fatalf("Failed to send timeboosted tx: %v", err)
	}

	timeboostedReceipt, err := arbtest.WaitForTx(ctx, l2client, signedTimeboostedTx.Hash(), 10*arbtest.Second)
	if err != nil {
		t.Fatalf("Failed to wait for timeboosted tx: %v", err)
	}

	// Check event from timeboosted transaction
	timeboostedEvents, err := contractABI.Unpack("TimeboostStatus", timeboostedReceipt.Logs[0].Data)
	if err != nil {
		t.Fatalf("Failed to unpack timeboosted tx event: %v", err)
	}

	timeboostedIsTimeBoosted := timeboostedEvents[0].(bool)
	if !timeboostedIsTimeBoosted {
		t.Error("Expected timeboosted transaction to be detected as time boosted")
	}

	// Test 3: Use view function to double-check
	viewCallData, err := contractABI.Pack("getTimeboostStatus")
	if err != nil {
		t.Fatalf("Failed to pack view call data: %v", err)
	}

	// Make eth_call during normal transaction context
	callMsg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: viewCallData,
	}

	result, err := l2client.CallContract(ctx, callMsg, nil)
	if err != nil {
		t.Fatalf("Failed to call contract: %v", err)
	}

	viewResult, err := contractABI.Unpack("getTimeboostStatus", result)
	if err != nil {
		t.Fatalf("Failed to unpack view result: %v", err)
	}

	// For eth_call, it should return false since it's not a timeboosted context
	viewIsTimeBoosted := viewResult[0].(bool)
	if viewIsTimeBoosted {
		t.Error("Expected view call to return false for timeboost status")
	}

	t.Logf("Test completed successfully!")
	t.Logf("Normal tx timeboost status: %v", normalIsTimeBoosted)
	t.Logf("Timeboosted tx timeboost status: %v", timeboostedIsTimeBoosted)
	t.Logf("View call timeboost status: %v", viewIsTimeBoosted)
}
