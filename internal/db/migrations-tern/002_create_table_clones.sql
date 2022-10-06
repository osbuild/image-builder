CREATE TABLE IF NOT EXISTS clones(
       id uuid PRIMARY KEY,
       compose_id uuid NOT NULL REFERENCES composes(job_id) ON DELETE CASCADE,
       request jsonb NOT NULL,
       created_at timestamp NOT NULL,

       CONSTRAINT clone_id_differs_from_compose_id CHECK (id != compose_id)
);
