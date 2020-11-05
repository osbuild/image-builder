CREATE TABLE IF NOT EXISTS composes(
       job_id uuid PRIMARY KEY,
       request json NOT NULL,
       created_at timestamp NOT NULL,
       account_id varchar NOT NULL,
       org_id varchar NOT NULL
);
