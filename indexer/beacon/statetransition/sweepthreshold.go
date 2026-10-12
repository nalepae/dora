package statetransition

import (
	"bytes"
	"encoding/binary"

	"github.com/ethpandaops/go-eth2-client/spec"
	"github.com/ethpandaops/go-eth2-client/spec/gloas"
	"github.com/ethpandaops/go-eth2-client/spec/phase0"
)

// EIP-8148 (custom sweep threshold) gives each validator a sweep threshold, stored in
// state.validator_sweep_thresholds (Gloas+), indexed like the validator registry.
//
// https://github.com/ethereum/consensus-specs/blob/master/specs/_features/eip8148/beacon-chain.md

// sweepThresholdCredentialOffset locates, inside 0x02 compounding withdrawal credentials,
// the 2 bytes holding the initial sweep threshold, in EFFECTIVE_BALANCE_INCREMENT units,
// little-endian.
const sweepThresholdCredentialOffset = 10

// validatorSweepThreshold returns state.validator_sweep_thresholds[index], or 0 before Gloas.
func (s *stateAccessor) validatorSweepThreshold(index phase0.ValidatorIndex) phase0.Gwei {
	if s.Version < spec.DataVersionGloas || uint64(index) >= uint64(len(s.ValidatorSweepThresholds)) {
		return 0
	}
	return s.ValidatorSweepThresholds[index]
}

// getEffectiveSweepThreshold implements get_effective_sweep_threshold: the validator's sweep
// threshold, or get_max_effective_balance when none is set. Before Gloas the threshold is
// always 0, so this is exactly get_max_effective_balance.
//
// https://github.com/ethereum/consensus-specs/blob/master/specs/_features/eip8148/beacon-chain.md#new-get_effective_sweep_threshold
func (s *stateAccessor) getEffectiveSweepThreshold(index phase0.ValidatorIndex) phase0.Gwei {
	if threshold := s.validatorSweepThreshold(index); threshold != 0 {
		return threshold
	}
	return s.getMaxEffectiveBalance(s.Validators[index])
}

// getInitialSweepThreshold implements get_initial_sweep_threshold: the threshold requested
// at deposit time through compounding withdrawal credentials. A threshold out of
// [MIN_ACTIVATION_BALANCE, MAX_EFFECTIVE_BALANCE_ELECTRA] falls back to the maximum, and
// non-compounding credentials carry none.
//
// https://github.com/ethereum/consensus-specs/blob/master/specs/_features/eip8148/beacon-chain.md#new-get_initial_sweep_threshold
func (s *stateAccessor) getInitialSweepThreshold(withdrawalCredentials []byte) phase0.Gwei {
	if len(withdrawalCredentials) != 32 || withdrawalCredentials[0] != 0x02 {
		return 0
	}

	increments := binary.LittleEndian.Uint16(withdrawalCredentials[sweepThresholdCredentialOffset:])
	threshold := uint64(increments) * s.specs.EffectiveBalanceIncrement
	if threshold < s.specs.MinActivationBalance || threshold > s.specs.MaxEffectiveBalanceElectra {
		return phase0.Gwei(s.specs.MaxEffectiveBalanceElectra)
	}
	return phase0.Gwei(threshold)
}

// initializeSweepThresholds builds state.validator_sweep_thresholds at the Gloas upgrade:
// compounding validators start at MAX_EFFECTIVE_BALANCE_ELECTRA, the others at 0.
func initializeSweepThresholds(s *stateAccessor) {
	thresholds := make([]phase0.Gwei, len(s.Validators))
	for i, v := range s.Validators {
		if hasCompoundingWithdrawalCredential(v) {
			thresholds[i] = phase0.Gwei(s.specs.MaxEffectiveBalanceElectra)
		}
	}
	s.ValidatorSweepThresholds = thresholds
}

// processSetSweepThresholdRequest implements process_set_sweep_threshold_request. Invalid
// requests are silently dropped.
//
// https://github.com/ethereum/consensus-specs/blob/master/specs/_features/eip8148/beacon-chain.md#new-process_set_sweep_threshold_request
func processSetSweepThresholdRequest(s *stateAccessor, request *gloas.SetSweepThresholdRequest) {
	if request == nil {
		return
	}

	index := findValidatorByPubkeyInRegistry(s, request.ValidatorPubkey)
	if index == nil || uint64(*index) >= uint64(len(s.ValidatorSweepThresholds)) {
		return
	}

	validator := s.Validators[*index]
	if !hasCompoundingWithdrawalCredential(validator) {
		return
	}
	if !bytes.Equal(validator.WithdrawalCredentials[12:], request.SourceAddress[:]) {
		return
	}
	if validator.ExitEpoch != FarFutureEpoch {
		return
	}

	threshold := request.Threshold
	if s.ValidatorSweepThresholds[*index] == threshold {
		return
	}
	// A threshold below the current balance would let the validator sweep out immediately,
	// bypassing the partial withdrawal queue.
	if threshold < s.Balances[*index] {
		return
	}
	if uint64(threshold)%s.specs.EffectiveBalanceIncrement != 0 {
		return
	}
	if uint64(threshold) < s.specs.MinActivationBalance || uint64(threshold) > s.specs.MaxEffectiveBalanceElectra {
		return
	}

	s.ValidatorSweepThresholds[*index] = threshold
}
