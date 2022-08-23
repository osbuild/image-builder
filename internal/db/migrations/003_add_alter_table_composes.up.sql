ALTER TABLE IF EXISTS composes ADD CONSTRAINT account_number_constraint CHECK (account_number NOT SIMILAR TO '[ ]*');
