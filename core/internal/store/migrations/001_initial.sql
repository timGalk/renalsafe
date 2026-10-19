CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;
CREATE TABLE IF NOT EXISTS knowledge_revision (singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton), revision bigint NOT NULL);
INSERT INTO knowledge_revision VALUES(true,0) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS documents (
 id text PRIMARY KEY, context jsonb NOT NULL, source jsonb NOT NULL,
 status text NOT NULL CHECK(status IN ('ACTIVE','SUPERSEDED')),
 page_start integer NOT NULL CHECK(page_start>0), page_end integer NOT NULL CHECK(page_end>=page_start),
 pdf bytea NOT NULL, text_content text NOT NULL, synthetic boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS chunks (
 id text PRIMARY KEY, document_id text NOT NULL REFERENCES documents(id), section text NOT NULL,
 page_start integer NOT NULL, page_end integer NOT NULL,
 start_byte integer NOT NULL CHECK(start_byte>=0), end_byte integer NOT NULL CHECK(end_byte>start_byte),
 content text NOT NULL, is_table boolean NOT NULL,
 search_text tsvector GENERATED ALWAYS AS (to_tsvector('simple',content)) STORED,
 embedding vector(1024), embedding_model text,
 CHECK ((embedding IS NULL)=(embedding_model IS NULL))
);
CREATE INDEX IF NOT EXISTS chunks_search_idx ON chunks USING gin(search_text);
CREATE INDEX IF NOT EXISTS chunks_document_idx ON chunks(document_id);
CREATE TABLE IF NOT EXISTS renal_rules (
 id text PRIMARY KEY, document_id text NOT NULL REFERENCES documents(id), chunk_id text NOT NULL REFERENCES chunks(id),
 product text NOT NULL, route text NOT NULL, form text NOT NULL, indication text NOT NULL,
 metric text NOT NULL, kidney_range numrange NOT NULL CHECK(NOT isempty(kidney_range)),
 status text NOT NULL CHECK(status IN ('PENDING','APPROVED','REJECTED','SUPERSEDED')),
 payload jsonb NOT NULL, digest text NOT NULL,
 CHECK(payload->>'status'=status AND payload->>'id'=id),
 EXCLUDE USING gist(product WITH =,route WITH =,form WITH =,indication WITH =,metric WITH =,kidney_range WITH &&) WHERE(status='APPROVED')
);
CREATE TABLE IF NOT EXISTS assessments (
 id text PRIMARY KEY, actor_id text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), payload jsonb NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_events (
 sequence bigint PRIMARY KEY, hash text UNIQUE NOT NULL, previous_hash text NOT NULL, payload jsonb NOT NULL
);
CREATE OR REPLACE FUNCTION immutable_audit_record() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'immutable history'; END $$;
DROP TRIGGER IF EXISTS immutable_audit ON audit_events;
CREATE TRIGGER immutable_audit BEFORE UPDATE OR DELETE ON audit_events FOR EACH ROW EXECUTE FUNCTION immutable_audit_record();
DROP TRIGGER IF EXISTS immutable_assessment ON assessments;
CREATE TRIGGER immutable_assessment BEFORE UPDATE OR DELETE ON assessments FOR EACH ROW EXECUTE FUNCTION immutable_audit_record();
CREATE OR REPLACE FUNCTION protect_document() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (to_jsonb(NEW)-'status') IS DISTINCT FROM (to_jsonb(OLD)-'status') THEN RAISE EXCEPTION 'immutable document bytes and scope'; END IF;
 IF OLD.status='SUPERSEDED' AND NEW.status<>'SUPERSEDED' THEN RAISE EXCEPTION 'superseded document cannot reactivate'; END IF;
 RETURN NEW; END $$;
DROP TRIGGER IF EXISTS protect_document ON documents;
CREATE TRIGGER protect_document BEFORE UPDATE ON documents FOR EACH ROW EXECUTE FUNCTION protect_document();
CREATE OR REPLACE FUNCTION protect_rule() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF OLD.status<>'PENDING' AND NOT (OLD.status='APPROVED' AND NEW.status='SUPERSEDED' AND (to_jsonb(NEW)-'status'-'payload')=(to_jsonb(OLD)-'status'-'payload') AND (NEW.payload-'status')=(OLD.payload-'status')) THEN RAISE EXCEPTION 'immutable reviewed rule'; END IF;
 RETURN NEW; END $$;
DROP TRIGGER IF EXISTS protect_rule ON renal_rules;
CREATE TRIGGER protect_rule BEFORE UPDATE ON renal_rules FOR EACH ROW EXECUTE FUNCTION protect_rule();
