CREATE TABLE IF NOT EXISTS composes(
       job_id uuid PRIMARY KEY,
       request jsonb NOT NULL,
       created_at timestamp NOT NULL,
       account_number varchar NOT NULL,
       org_id varchar NOT NULL,
       image_name varchar
);
