package services

import (
	"bytes"
	"context"
	"slices"

	"github.com/ethpandaops/go-eth2-client/spec/phase0"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/dora/db"
	"github.com/ethpandaops/dora/dbtypes"
)

// GetSweepThresholdRequestsByFilter returns a page of EIP-8148 set sweep threshold requests processed by
// the beacon chain, newest first, and the total number of matching requests. Recent requests come from
// the indexer cache, older ones from the database.
func (bs *ChainService) GetSweepThresholdRequestsByFilter(ctx context.Context, filter *dbtypes.SweepThresholdRequestFilter, pageOffset uint64, pageSize uint32) ([]*dbtypes.SweepThresholdRequest, uint64) {
	chainState := bs.consensusPool.GetChainState()
	_, prunedEpoch := bs.beaconIndexer.GetBlockCacheState()
	idxMinSlot := chainState.EpochToSlot(prunedEpoch)
	currentSlot := chainState.CurrentSlot()

	canonicalForkIds := bs.GetCanonicalForkIds()

	// load most recent objects from indexer cache
	cachedMatches := make([]*dbtypes.SweepThresholdRequest, 0)
	for slotIdx := int64(currentSlot); slotIdx >= int64(idxMinSlot); slotIdx-- {
		slot := uint64(slotIdx)
		if filter.MinSlot > 0 && slot < filter.MinSlot {
			continue
		}
		if filter.MaxSlot > 0 && slot > filter.MaxSlot {
			continue
		}

		for _, block := range bs.beaconIndexer.GetBlocksBySlot(phase0.Slot(slot)) {
			isCanonical := bs.isCanonicalForkId(uint64(block.GetForkId()), canonicalForkIds)
			if filter.WithOrphaned == 0 && !isCanonical {
				continue
			}
			if filter.WithOrphaned == 2 && isCanonical {
				continue
			}

			requests := block.GetDbSweepThresholdRequests(bs.beaconIndexer, isCanonical)
			slices.Reverse(requests) // reverse as the database results are ordered by descending block index too
			for _, request := range requests {
				if len(filter.SourceAddress) > 0 && !bytes.Equal(request.SourceAddress, filter.SourceAddress) {
					continue
				}
				if len(filter.PublicKey) > 0 && !bytes.Equal(request.ValidatorPubkey, filter.PublicKey) {
					continue
				}

				cachedMatches = append(cachedMatches, request)
			}
		}
	}

	resObjs := make([]*dbtypes.SweepThresholdRequest, 0)

	cachedStart := pageOffset
	cachedEnd := cachedStart + uint64(pageSize)
	cachedMatchesLen := uint64(len(cachedMatches))

	if cachedEnd <= cachedMatchesLen {
		resObjs = append(resObjs, cachedMatches[cachedStart:cachedEnd]...)
	} else if cachedStart < cachedMatchesLen {
		resObjs = append(resObjs, cachedMatches[cachedStart:]...)
	}

	// load older objects from db
	var dbObjects []*dbtypes.SweepThresholdRequest
	var dbCount uint64
	var err error

	if cachedEnd <= cachedMatchesLen {
		// all results from cache, just get result count from db
		_, dbCount, err = db.GetSweepThresholdRequestsFiltered(ctx, 0, 1, canonicalForkIds, filter)
	} else {
		dbSliceStart := uint64(0)
		if cachedStart > cachedMatchesLen {
			dbSliceStart = cachedStart - cachedMatchesLen
		}

		dbSliceLimit := pageSize - uint32(len(resObjs))
		dbObjects, dbCount, err = db.GetSweepThresholdRequestsFiltered(ctx, dbSliceStart, dbSliceLimit, canonicalForkIds, filter)
	}

	if err != nil {
		logrus.Warnf("ChainService.GetSweepThresholdRequestsByFilter error: %v", err)
		return resObjs, cachedMatchesLen
	}

	for _, dbObject := range dbObjects {
		dbObject.Orphaned = !bs.isCanonicalForkId(dbObject.ForkId, canonicalForkIds)
		if filter.WithOrphaned == 0 && dbObject.Orphaned {
			continue
		}
		if filter.WithOrphaned == 2 && !dbObject.Orphaned {
			continue
		}

		resObjs = append(resObjs, dbObject)
	}

	return resObjs, cachedMatchesLen + dbCount
}
