ALTER TABLE assessments ADD COLUMN audit_hash text GENERATED ALWAYS AS (payload->>'audit_hash') STORED;
ALTER TABLE assessments ALTER COLUMN audit_hash SET NOT NULL;
ALTER TABLE assessments ADD CONSTRAINT assessment_audit_fk FOREIGN KEY (audit_hash) REFERENCES audit_events(hash);
ALTER TABLE assessments ADD CONSTRAINT assessment_audit_unique UNIQUE (audit_hash);

CREATE OR REPLACE FUNCTION protect_chunk() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (to_jsonb(NEW)-'embedding'-'embedding_model'-'search_text') IS DISTINCT FROM (to_jsonb(OLD)-'embedding'-'embedding_model'-'search_text') THEN RAISE EXCEPTION 'immutable source passage'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER protect_chunk BEFORE UPDATE ON chunks FOR EACH ROW EXECUTE FUNCTION protect_chunk();
CREATE TRIGGER preserve_document BEFORE DELETE ON documents FOR EACH ROW EXECUTE FUNCTION immutable_audit_record();
CREATE TRIGGER preserve_chunk BEFORE DELETE ON chunks FOR EACH ROW EXECUTE FUNCTION immutable_audit_record();
CREATE TRIGGER preserve_rule BEFORE DELETE ON renal_rules FOR EACH ROW EXECUTE FUNCTION immutable_audit_record();
