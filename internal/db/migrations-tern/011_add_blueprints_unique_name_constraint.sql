ALTER TABLE blueprints
ADD CONSTRAINT blueprints_name_unique UNIQUE (name, org_id);
