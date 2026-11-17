CREATE TABLE extraction_jobs (
 id text PRIMARY KEY, document_id text NOT NULL REFERENCES documents(id),
 actor_id text NOT NULL, request_key text NOT NULL, model text NOT NULL,
 status text NOT NULL CHECK(status IN ('QUEUED','RUNNING','COMPLETED','FAILED','CANCELLED','DISMISSED')),
 payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(actor_id,request_key), CHECK(payload->>'id'=id AND payload->>'status'=status)
);
CREATE INDEX extraction_jobs_queue ON extraction_jobs(created_at) WHERE status IN ('QUEUED','RUNNING');
CREATE TABLE extraction_candidates (
 job_id text NOT NULL REFERENCES extraction_jobs(id), finding_index integer NOT NULL CHECK(finding_index>=0),
 rule_id text NOT NULL UNIQUE REFERENCES renal_rules(id),
 PRIMARY KEY(job_id,finding_index)
);
