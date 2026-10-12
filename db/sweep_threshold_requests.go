package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethpandaops/dora/dbtypes"
	"github.com/jmoiron/sqlx"
)

// InsertSweepThresholdRequests inserts or updates EIP-8148 set sweep threshold requests.
func InsertSweepThresholdRequests(ctx context.Context, tx *sqlx.Tx, requests []*dbtypes.SweepThresholdRequest) error {
	var sql strings.Builder
	fmt.Fprint(&sql,
		EngineQuery(map[dbtypes.DBEngineType]string{
			dbtypes.DBEnginePgsql:  "INSERT INTO sweep_threshold_requests ",
			dbtypes.DBEngineSqlite: "INSERT OR REPLACE INTO sweep_threshold_requests ",
		}),
		"(slot_number, slot_root, slot_index, orphaned, fork_id, source_address, validator_index, validator_pubkey, threshold, block_number)",
		" VALUES ",
	)
	argIdx := 0
	fieldCount := 10

	args := make([]any, len(requests)*fieldCount)
	for i, request := range requests {
		if i > 0 {
			fmt.Fprintf(&sql, ", ")
		}
		fmt.Fprintf(&sql, "(")
		for f := 0; f < fieldCount; f++ {
			if f > 0 {
				fmt.Fprintf(&sql, ", ")
			}
			fmt.Fprintf(&sql, "$%v", argIdx+f+1)
		}
		fmt.Fprintf(&sql, ")")

		args[argIdx+0] = request.SlotNumber
		args[argIdx+1] = request.SlotRoot
		args[argIdx+2] = request.SlotIndex
		args[argIdx+3] = request.Orphaned
		args[argIdx+4] = request.ForkId
		args[argIdx+5] = request.SourceAddress
		args[argIdx+6] = request.ValidatorIndex
		args[argIdx+7] = request.ValidatorPubkey
		args[argIdx+8] = request.Threshold
		args[argIdx+9] = request.BlockNumber
		argIdx += fieldCount
	}
	fmt.Fprint(&sql, EngineQuery(map[dbtypes.DBEngineType]string{
		dbtypes.DBEnginePgsql:  " ON CONFLICT (slot_root, slot_index) DO UPDATE SET orphaned = excluded.orphaned, fork_id = excluded.fork_id, validator_index = excluded.validator_index",
		dbtypes.DBEngineSqlite: "",
	}))

	_, err := tx.ExecContext(ctx, sql.String(), args...)
	if err != nil {
		return err
	}
	return nil
}

// GetSweepThresholdRequestsFiltered returns a page of EIP-8148 set sweep threshold requests matching the
// filter, newest first, and the total number of matching requests.
func GetSweepThresholdRequestsFiltered(ctx context.Context, offset uint64, limit uint32, canonicalForkIds []uint64, filter *dbtypes.SweepThresholdRequestFilter) ([]*dbtypes.SweepThresholdRequest, uint64, error) {
	var sql strings.Builder
	args := []interface{}{}
	fmt.Fprint(&sql, `
	WITH cte AS (
		SELECT
			slot_number, slot_root, slot_index, orphaned, fork_id, source_address, validator_index, validator_pubkey, CAST(threshold AS BIGINT), block_number
		FROM sweep_threshold_requests
	`)

	filterOp := "WHERE"
	if filter.MinSlot > 0 {
		args = append(args, filter.MinSlot)
		fmt.Fprintf(&sql, " %v slot_number >= $%v", filterOp, len(args))
		filterOp = "AND"
	}
	if filter.MaxSlot > 0 {
		args = append(args, filter.MaxSlot)
		fmt.Fprintf(&sql, " %v slot_number <= $%v", filterOp, len(args))
		filterOp = "AND"
	}
	if len(filter.PublicKey) > 0 {
		args = append(args, filter.PublicKey)
		fmt.Fprintf(&sql, " %v validator_pubkey = $%v", filterOp, len(args))
		filterOp = "AND"
	}
	if len(filter.SourceAddress) > 0 {
		args = append(args, filter.SourceAddress)
		fmt.Fprintf(&sql, " %v source_address = $%v", filterOp, len(args))
		filterOp = "AND"
	}

	appendWithOrphanedFilter(&sql, &args, &filterOp, filter.WithOrphaned, canonicalForkIds, "fork_id")

	args = append(args, limit)
	fmt.Fprintf(&sql, `)
	SELECT
		count(*) AS slot_number,
		null AS slot_root,
		0 AS slot_index,
		false AS orphaned,
		0 AS fork_id,
		null AS source_address,
		0 AS validator_index,
		null AS validator_pubkey,
		CAST(0 AS BIGINT) AS threshold,
		0 AS block_number
	FROM cte
	UNION ALL SELECT * FROM (
	SELECT * FROM cte
	ORDER BY slot_number DESC, slot_index DESC
	LIMIT $%v
	`, len(args))

	if offset > 0 {
		args = append(args, offset)
		fmt.Fprintf(&sql, " OFFSET $%v ", len(args))
	}
	fmt.Fprintf(&sql, ") AS t1")

	requests := []*dbtypes.SweepThresholdRequest{}
	err := ReaderDb.SelectContext(ctx, &requests, sql.String(), args...)
	if err != nil {
		logger.Errorf("Error while fetching filtered sweep threshold requests: %v", err)
		return nil, 0, err
	}

	return requests[1:], requests[0].SlotNumber, nil
}
