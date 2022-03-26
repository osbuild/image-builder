ALTER TABLE IF EXISTS composes ADD CONSTRAINT org_id_constraint CHECK (org_id NOT SIMILAR TO '[ ]*');
