-- Host-port mappings for a workload's container — see
-- docs/ROADMAP.md's Phase 6. A normalized table (not a JSONB column on
-- workloads) specifically so Postgres can enforce the unique index
-- below: two workloads on the same node cannot claim the same host
-- port. node_id is duplicated from workloads.node_id (rather than
-- joined through workload_id at constraint-check time) because a
-- unique index can only see columns physically present on the table
-- it's defined on.
CREATE TABLE workload_ports (
    id              UUID PRIMARY KEY,
    workload_id     UUID NOT NULL REFERENCES workloads(id) ON DELETE CASCADE,
    node_id         UUID NOT NULL REFERENCES nodes(id),
    container_port  INTEGER NOT NULL,
    host_port       INTEGER NOT NULL,
    protocol        TEXT NOT NULL DEFAULT 'tcp',

    UNIQUE (node_id, host_port, protocol)
);

CREATE INDEX workload_ports_workload_id_idx ON workload_ports (workload_id);

-- The address a node last reported from, captured server-side from the
-- request's remote address at registration/heartbeat time rather than
-- self-reported by the agent, so ambudctl/the web UI can show a
-- reachable "nodeIP:hostPort" for a deployed workload without Ambud
-- managing DNS (see docs/ROADMAP.md's Phase 6 "reachable address" note).
ALTER TABLE nodes ADD COLUMN address TEXT NOT NULL DEFAULT '';
