ALTER TABLE blueprints
    DROP COLUMN body_version,
    DROP COLUMN body;

CREATE INDEX ON blueprints(org_id);

CREATE TABLE IF NOT EXISTS blueprint_versions
(
    id uuid PRIMARY KEY,
    blueprint_id uuid NOT NULL REFERENCES blueprints(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    body JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT current_timestamp
);

CREATE UNIQUE INDEX ON blueprint_versions(blueprint_id, version);

ALTER TABLE composes ADD COLUMN blueprint_version_id uuid NULL REFERENCES blueprint_versions (id) ON DELETE SET NULL;
CREATE INDEX ON composes(blueprint_version_id);
