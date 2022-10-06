CREATE TABLE IF NOT EXISTS composes(
       job_id uuid PRIMARY KEY,
       request jsonb NOT NULL,
       created_at timestamp NOT NULL,
       account_number varchar NOT NULL,
       org_id varchar NOT NULL,
       image_name varchar,

       CONSTRAINT account_number_constraint CHECK (account_number NOT SIMILAR TO '[ ]*'),
       CONSTRAINT org_id_constraint CHECK (org_id NOT SIMILAR TO '[ ]*'),
       CONSTRAINT check_name_length CHECK (length(image_name) <= 100) NOT VALID
);
