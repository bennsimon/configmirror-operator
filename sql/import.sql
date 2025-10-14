create schema IF NOT EXISTS configmirror;

CREATE TABLE IF NOT EXISTS configmirror.configmaps
(
    name                  TEXT,
    source_namespace      TEXT,
    destination_namespace TEXT,
    configmirror          TEXT,
    json_data             JSONB       DEFAULT '{}'::jsonb,
    created_at            TIMESTAMPTZ DEFAULT NOW(),
    updated_at            TIMESTAMPTZ,
    UNIQUE (name, destination_namespace)
);
