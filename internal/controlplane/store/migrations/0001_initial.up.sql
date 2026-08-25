-- Node registry. credential_hash is SHA-256 of the bearer token issued
-- to the agent at registration; the plaintext token is never stored
-- (see docs/ARCHITECTURE.md's node-identity design). Resource columns
-- hold the most recent heartbeat's snapshot only — history/trends are
-- Phase 8, not this table.
CREATE TABLE nodes (
    id                UUID PRIMARY KEY,
    name              TEXT NOT NULL UNIQUE,
    credential_hash   TEXT NOT NULL,

    cpu_cores         INTEGER NOT NULL DEFAULT 0,
    cpu_used_percent  DOUBLE PRECISION NOT NULL DEFAULT 0,
    mem_total_bytes   BIGINT NOT NULL DEFAULT 0,
    mem_used_bytes    BIGINT NOT NULL DEFAULT 0,
    disk_total_bytes  BIGINT NOT NULL DEFAULT 0,
    disk_used_bytes   BIGINT NOT NULL DEFAULT 0,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_heartbeat_at TIMESTAMPTZ
);

-- One-time join tokens an operator generates and hands to a new node's
-- agent. Like node credentials, only the hash is stored. A token is
-- consumed (used_at set) the moment a node successfully registers with
-- it — see internal/controlplane/store's RegisterNode.
CREATE TABLE join_tokens (
    token_hash    TEXT PRIMARY KEY,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at       TIMESTAMPTZ,
    used_by_node  UUID REFERENCES nodes(id)
);

-- Desired + last-observed state for one workload. There is no
-- scheduler yet (Phase 5) — node_id is set at creation time by
-- whoever/whatever calls CreateWorkload (today: the API handler,
-- trivially, since Phase 3 has at most one node to choose from).
-- state/pid are last-observed-by-agent, not desired — they're
-- overwritten on every heartbeat that reports this workload.
CREATE TABLE workloads (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    image       TEXT NOT NULL,
    node_id     UUID NOT NULL REFERENCES nodes(id),

    state       TEXT NOT NULL DEFAULT 'pending',
    pid         INTEGER NOT NULL DEFAULT 0,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX workloads_node_id_idx ON workloads (node_id);
