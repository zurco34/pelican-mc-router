CREATE TABLE pending_setup (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    pelican_url TEXT NOT NULL,
    pelican_secret_name TEXT NOT NULL,
    router_domain TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(pelican_url) > 0),
    CHECK (length(pelican_secret_name) > 0),
    CHECK (length(router_domain) > 0)
);
