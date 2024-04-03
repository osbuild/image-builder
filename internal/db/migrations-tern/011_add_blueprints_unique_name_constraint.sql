ALTER TABLE blueprints
ALTER COLUMN name TYPE VARCHAR(200);

-- Add " N" suffix to all blueprint names which row number is higher
-- than 2. For two blueprints named "test" it will leave one record as
-- "test" and the other will be updated to "test 2 (numbered for uniqueness)" and so on. 
-- Numbering is ordered by created first, so latest Blueprint will have highest number.
UPDATE blueprints SET name = CONCAT(names.name, ' ', names.rn::text, ' (renamed at ', TO_CHAR(CURRENT_TIMESTAMP, 'YYYY/MM/DD HH24:MM:SS'), ')')
    FROM (SELECT id, name, row_number() over (partition by name, org_id order by created_at) as rn from blueprints) AS names
    WHERE names.rn > 1 AND blueprints.id = names.id;

ALTER TABLE blueprints
ADD CONSTRAINT blueprints_name_unique UNIQUE (name, org_id);
