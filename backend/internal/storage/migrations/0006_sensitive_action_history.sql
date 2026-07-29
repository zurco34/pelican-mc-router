CREATE TABLE sensitive_action_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at DATETIME NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('bootstrap_authorization', 'setup', 'settings_update', 'manual_reconciliation', 'route_policy_mutation', 'rate_limit_rejection')),
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure', 'denied', 'canceled'))
);

CREATE INDEX sensitive_action_history_occurred_at_idx
    ON sensitive_action_history (occurred_at DESC, id DESC);
