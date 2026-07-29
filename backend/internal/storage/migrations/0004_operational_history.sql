CREATE TABLE operational_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at DATETIME NOT NULL,
    kind TEXT NOT NULL,
    outcome TEXT NOT NULL,
    desired INTEGER NOT NULL DEFAULT 0 CHECK (desired >= 0),
    created INTEGER NOT NULL DEFAULT 0 CHECK (created >= 0),
    updated INTEGER NOT NULL DEFAULT 0 CHECK (updated >= 0),
    deleted INTEGER NOT NULL DEFAULT 0 CHECK (deleted >= 0),
    changed INTEGER NOT NULL DEFAULT 0 CHECK (changed IN (0, 1)),
    CHECK (kind IN ('reconciliation', 'setup', 'settings', 'manual_reconcile')),
    CHECK (outcome IN ('not_configured', 'success', 'failure'))
);

CREATE INDEX operational_history_occurred_at_idx
    ON operational_history (occurred_at DESC, id DESC);
