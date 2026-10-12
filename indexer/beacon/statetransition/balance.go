package statetransition

import (
	"github.com/ethpandaops/go-eth2-client/spec/phase0"
)

// processEffectiveBalanceUpdates implements the Electra+ version of
// process_effective_balance_updates.
// Modified in Electra: https://github.com/ethereum/consensus-specs/blob/master/specs/electra/beacon-chain.md#modified-process_effective_balance_updates
// Modified in EIP-8148: the cap is the validator's effective sweep threshold.
// https://github.com/ethereum/consensus-specs/blob/master/specs/_features/eip8148/beacon-chain.md#modified-process_effective_balance_updates
func processEffectiveBalanceUpdates(s *stateAccessor) {
	hysteresisIncrement := s.specs.EffectiveBalanceIncrement / s.specs.HysteresisQuotient
	downwardThreshold := hysteresisIncrement * s.specs.HysteresisDownwardMultiplier
	upwardThreshold := hysteresisIncrement * s.specs.HysteresisUpwardMultiplier

	for i, v := range s.Validators {
		balance := uint64(s.Balances[i])
		maxEB := uint64(s.getEffectiveSweepThreshold(phase0.ValidatorIndex(i)))
		eb := uint64(v.EffectiveBalance)

		if balance+downwardThreshold < eb || eb+upwardThreshold < balance {
			newEB := balance - balance%s.specs.EffectiveBalanceIncrement
			if newEB > maxEB {
				newEB = maxEB
			}
			v.EffectiveBalance = phase0.Gwei(newEB)
		}
	}
}
