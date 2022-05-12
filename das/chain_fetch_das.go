// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/nitro/blob/master/LICENSE

package das

import (
	"bytes"
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/offchainlabs/nitro/solgen/go/bridgegen"
)

type ChainFetchDAS struct {
	DataAvailabilityService
	seqInboxCaller   *bridgegen.SequencerInboxCaller
	seqInboxFilterer *bridgegen.SequencerInboxFilterer
}

func NewChainFetchDAS(inner DataAvailabilityService, l1Url string, seqInboxAddr common.Address) (*ChainFetchDAS, error) {
	l1client, err := ethclient.Dial(l1Url)
	if err != nil {
		return nil, err
	}
	seqInbox, err := bridgegen.NewSequencerInbox(seqInboxAddr, l1client)
	if err != nil {
		return nil, err
	}

	return &ChainFetchDAS{inner, &seqInbox.SequencerInboxCaller, &seqInbox.SequencerInboxFilterer}, nil
}

func (das *ChainFetchDAS) KeysetFromHash(ctx context.Context, ksHash []byte) ([]byte, error) {
	// try to fetch from the inner DAS
	innerRes, err := das.DataAvailabilityService.KeysetFromHash(ctx, ksHash)
	if err == nil && bytes.Equal(ksHash, crypto.Keccak256(innerRes)) {
		return innerRes, nil
	}

	// try to fetch from the L1 chain
	var ksHash32 [32]byte
	copy(ksHash32[:], ksHash)
	blockNumBig, err := das.seqInboxCaller.GetKeysetCreationBlock(&bind.CallOpts{Context: ctx}, ksHash32)
	if err != nil {
		return nil, err
	}
	if !blockNumBig.IsUint64() {
		return nil, errors.New("block number too large")
	}
	blockNum := blockNumBig.Uint64()
	blockNumPlus1 := blockNum + 1

	filterOpts := &bind.FilterOpts{
		Start:   blockNum,
		End:     &blockNumPlus1,
		Context: ctx,
	}
	iter, err := das.seqInboxFilterer.FilterSetValidKeyset(filterOpts, [][32]byte{ksHash32})
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		if bytes.Equal(iter.Event.KeysetHash[:], ksHash) {
			return iter.Event.KeysetBytes, nil
		}
	}
	if iter.Error() != nil {
		return nil, iter.Error()
	}

	return nil, errors.New("Keyset not found on chain")
}
