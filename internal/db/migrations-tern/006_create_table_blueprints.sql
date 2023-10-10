CREATE TABLE IF NOT EXISTS blueprints
(
    id uuid PRIMARY KEY,
    org_id varchar NOT NULL CHECK (coalesce(trim(org_id), '') != ''),
    account_number varchar NOT NULL CHECK (coalesce(trim(account_number), '') != ''),
    name VARCHAR(100) NOT NULL,
    description VARCHAR(250),
    body_version INTEGER NOT NULL,
    body JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT current_timestamp
);
