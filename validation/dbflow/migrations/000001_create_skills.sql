-- +goose Up
CREATE TABLE IF NOT EXISTS skills (
    id VARCHAR(128) PRIMARY KEY,
    tenant_id VARCHAR(128) NOT NULL,
    owner_id VARCHAR(128) NOT NULL,
    name VARCHAR(128) NOT NULL,
    display_name VARCHAR(256) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE UNIQUE INDEX uk_skills_tenant_name ON skills (tenant_id, name);
CREATE INDEX idx_skills_tenant_owner ON skills (tenant_id, owner_id);

-- +goose Down
DROP TABLE IF EXISTS skills;
