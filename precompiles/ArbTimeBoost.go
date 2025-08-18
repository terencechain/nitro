// Copyright 2021-2025, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE.md

package precompiles

// ArbTimeBoost provides access to time boost (express lane) transaction information
type ArbTimeBoost struct {
	Address addr
}

// IsTimeBoosted returns whether the current transaction was submitted via time boost / express lane
func (precompile *ArbTimeBoost) IsTimeBoosted(c ctx, evm mech) (bool, error) {
	// Access the timeboost flag from the transaction processor
	if c.txProcessor != nil {
		return c.txProcessor.IsTimeBoosted, nil
	}
	return false, nil
}