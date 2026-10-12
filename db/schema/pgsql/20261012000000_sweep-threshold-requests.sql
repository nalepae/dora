-- +goose Up
-- +goose StatementBegin

-- EIP-8148 set sweep threshold requests (CL view, attributed to the processing slot)
CREATE TABLE IF NOT EXISTS public."sweep_threshold_requests" (
    slot_number BIGINT NOT NULL,
    slot_root bytea NOT NULL,
    slot_index INT NOT NULL,
    orphaned bool NOT NULL DEFAULT FALSE,
    fork_id BIGINT NOT NULL DEFAULT 0,
    source_address bytea NOT NULL,
    validator_index BIGINT NULL,
    validator_pubkey bytea NOT NULL,
    threshold BIGINT NOT NULL,
    block_number BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT sweep_threshold_requests_pkey PRIMARY KEY (slot_root, slot_index)
);

CREATE INDEX IF NOT EXISTS "sweep_threshold_requests_slot_number_idx"
    ON public."sweep_threshold_requests" ("slot_number" ASC NULLS FIRST);
CREATE INDEX IF NOT EXISTS "sweep_threshold_requests_source_addr_idx"
    ON public."sweep_threshold_requests" ("source_address" ASC NULLS FIRST);
CREATE INDEX IF NOT EXISTS "sweep_threshold_requests_validator_index_idx"
    ON public."sweep_threshold_requests" ("validator_index" ASC NULLS FIRST);
CREATE INDEX IF NOT EXISTS "sweep_threshold_requests_validator_pubkey_idx"
    ON public."sweep_threshold_requests" ("validator_pubkey" ASC NULLS FIRST);
CREATE INDEX IF NOT EXISTS "sweep_threshold_requests_fork_idx"
    ON public."sweep_threshold_requests" ("fork_id" ASC NULLS FIRST);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SELECT 'NOT SUPPORTED';
-- +goose StatementEnd
