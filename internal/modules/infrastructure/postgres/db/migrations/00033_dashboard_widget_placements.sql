-- +goose Up
CREATE TABLE plugins.dashboard_widget_placement (
    placement_id uuid PRIMARY KEY,
    plugin_id text NOT NULL,
    plugin_version text NOT NULL,
    artifact_digest text NOT NULL CHECK (artifact_digest ~ '^[0-9a-f]{64}$'),
    widget_id text NOT NULL,
    configuration jsonb NOT NULL CHECK (jsonb_typeof(configuration) = 'object'),
    position integer NOT NULL CHECK (position >= 0),
    enabled boolean NOT NULL DEFAULT true,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (position),
    FOREIGN KEY (plugin_id, plugin_version, artifact_digest)
      REFERENCES plugins.installed_release(plugin_id, version, artifact_digest) ON DELETE RESTRICT
);

CREATE INDEX dashboard_widget_placement_order_idx ON plugins.dashboard_widget_placement (position, placement_id);

-- +goose Down
DROP TABLE plugins.dashboard_widget_placement;
