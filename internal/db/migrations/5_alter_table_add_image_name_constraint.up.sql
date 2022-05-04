ALTER TABLE composes
  ADD CONSTRAINT check_name_length CHECK (length(image_name) <= 100) NOT VALID;
