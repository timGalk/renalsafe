CREATE TABLE synthetic_patients (
 id text PRIMARY KEY, code text UNIQUE NOT NULL, alias text NOT NULL,
 owner_id text NOT NULL REFERENCES app_users(id), archived boolean NOT NULL DEFAULT false,
 version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX patients_owner_date_idx ON synthetic_patients(owner_id,created_at DESC,id DESC);
CREATE TABLE patient_access (
 patient_id text NOT NULL REFERENCES synthetic_patients(id), user_id text NOT NULL REFERENCES app_users(id),
 PRIMARY KEY(patient_id,user_id)
);
CREATE TABLE renal_observations (
 id text PRIMARY KEY, patient_id text NOT NULL REFERENCES synthetic_patients(id),
 measured_at timestamptz NOT NULL, corrects_id text UNIQUE REFERENCES renal_observations(id),
 payload jsonb NOT NULL
);
CREATE INDEX observations_patient_date_idx ON renal_observations(patient_id,measured_at DESC,id DESC);
CREATE TABLE patient_assessments (
 assessment_id text PRIMARY KEY REFERENCES assessments(id), patient_id text NOT NULL REFERENCES synthetic_patients(id),
 observation_id text NOT NULL REFERENCES renal_observations(id), previous_id text REFERENCES assessments(id),
 actor_id text NOT NULL REFERENCES app_users(id), retry_key text NOT NULL, request_digest text NOT NULL,
 UNIQUE(patient_id,actor_id,retry_key)
);
CREATE INDEX patient_assessments_patient_idx ON patient_assessments(patient_id,assessment_id);
CREATE INDEX assessments_actor_date_idx ON assessments(actor_id,created_at DESC,id DESC);
CREATE TRIGGER preserve_patient BEFORE DELETE ON synthetic_patients FOR EACH ROW EXECUTE FUNCTION immutable_audit_record();
CREATE TRIGGER preserve_observation BEFORE UPDATE OR DELETE ON renal_observations FOR EACH ROW EXECUTE FUNCTION immutable_audit_record();
CREATE TRIGGER preserve_patient_assessment BEFORE UPDATE OR DELETE ON patient_assessments FOR EACH ROW EXECUTE FUNCTION immutable_audit_record();
CREATE FUNCTION validate_patient_assessment() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM renal_observations WHERE id=NEW.observation_id AND patient_id=NEW.patient_id) THEN RAISE EXCEPTION 'observation belongs to another case'; END IF;
 IF NEW.previous_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM patient_assessments WHERE assessment_id=NEW.previous_id AND patient_id=NEW.patient_id) THEN RAISE EXCEPTION 'predecessor belongs to another case'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER validate_patient_assessment BEFORE INSERT ON patient_assessments FOR EACH ROW EXECUTE FUNCTION validate_patient_assessment();
CREATE FUNCTION validate_observation_correction() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.corrects_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM renal_observations WHERE id=NEW.corrects_id AND patient_id=NEW.patient_id) THEN RAISE EXCEPTION 'correction belongs to another case'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER validate_observation_correction BEFORE INSERT ON renal_observations FOR EACH ROW EXECUTE FUNCTION validate_observation_correction();
CREATE FUNCTION protect_patient_identity() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (NEW.id,NEW.code,NEW.owner_id,NEW.created_at) IS DISTINCT FROM (OLD.id,OLD.code,OLD.owner_id,OLD.created_at) THEN RAISE EXCEPTION 'immutable patient identity'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER protect_patient_identity BEFORE UPDATE ON synthetic_patients FOR EACH ROW EXECUTE FUNCTION protect_patient_identity();
