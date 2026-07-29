CREATE TABLE route_policies (
    server_uuid TEXT PRIMARY KEY NOT NULL,
    primary_hostname TEXT,
    excluded INTEGER NOT NULL DEFAULT 0 CHECK (excluded IN (0, 1)),
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(server_uuid) > 0)
);

CREATE TABLE route_policy_aliases (
    server_uuid TEXT NOT NULL,
    hostname TEXT NOT NULL,
    PRIMARY KEY (server_uuid, hostname),
    FOREIGN KEY (server_uuid) REFERENCES route_policies(server_uuid)
        ON DELETE CASCADE,
    CHECK (length(hostname) > 0)
);
