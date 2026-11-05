package httpapi

import (
	"encoding/json"
	"reflect"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/store"
	"strings"
	"time"
)

// OpenAPI derives payload schemas from the actual exported JSON contracts. It is
// also emitted as api/openapi.json for clients without a running service.
func OpenAPI() map[string]any {
	schemas := map[string]any{}
	var schema func(reflect.Type) map[string]any
	schema = func(t reflect.Type) map[string]any {
		if t == reflect.TypeOf(time.Time{}) {
			return map[string]any{"type": "string", "format": "date-time"}
		}
		if t == reflect.TypeOf(json.RawMessage{}) {
			return map[string]any{"description": "JSON audit payload"}
		}
		if t.Kind() == reflect.Pointer {
			return map[string]any{"anyOf": []any{schema(t.Elem()), map[string]any{"type": "null"}}}
		}
		switch t.Kind() {
		case reflect.Struct:
			name := t.Name()
			if _, ok := schemas[name]; !ok {
				schemas[name] = map[string]any{}
				properties := map[string]any{}
				required := []string{}
				for i := 0; i < t.NumField(); i++ {
					f := t.Field(i)
					tag := strings.Split(f.Tag.Get("json"), ",")
					if tag[0] == "" || tag[0] == "-" {
						continue
					}
					properties[tag[0]] = schema(f.Type)
					if !strings.Contains(f.Tag.Get("json"), "omitempty") && f.Type.Kind() != reflect.Pointer {
						required = append(required, tag[0])
					}
				}
				schemas[name] = map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
			}
			return map[string]any{"$ref": "#/components/schemas/" + name}
		case reflect.Slice:
			return map[string]any{"type": "array", "items": schema(t.Elem())}
		case reflect.String:
			return map[string]any{"type": "string"}
		case reflect.Bool:
			return map[string]any{"type": "boolean"}
		case reflect.Int, reflect.Int64:
			return map[string]any{"type": "integer"}
		case reflect.Float64:
			return map[string]any{"type": "number"}
		default:
			return map[string]any{}
		}
	}
	ref := func(v any) map[string]any { return schema(reflect.TypeOf(v)) }
	array := func(v any) map[string]any { return map[string]any{"type": "array", "items": ref(v)} }
	paths := map[string]any{}
	operation := func(path, method, description string, body, response map[string]any, status string) {
		errors := map[string]any{}
		for _, code := range []string{"401", "403", "404", "409", "413", "415", "422", "503"} {
			errors[code] = map[string]any{"description": "Request rejected or operation not committed", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{"error": map[string]any{"type": "string"}}}}}}
		}
		errors[status] = map[string]any{"description": "Success after required persistence", "content": map[string]any{"application/json": map[string]any{"schema": response}}}
		op := map[string]any{"summary": description, "responses": errors}
		if strings.Contains(path, "{id}") {
			op["parameters"] = []any{map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}}}
		}
		if body != nil {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": body}}}
		}
		item, ok := paths[path].(map[string]any)
		if !ok {
			item = map[string]any{}
			paths[path] = item
		}
		item[method] = op
	}
	object := func(properties map[string]any) map[string]any {
		return map[string]any{"type": "object", "properties": properties}
	}

	textField := map[string]any{"type": "string"}
	statusResponse := object(map[string]any{"status": textField})
	operation("/api/v1/auth/login", "post", "Create cookie session with durable login throttling", object(map[string]any{"username": textField, "password": textField}), ref(store.Session{}), "200")
	operation("/api/v1/auth/activate", "post", "Consume single-use activation secret", object(map[string]any{"username": textField, "activation_secret": textField, "password": textField}), statusResponse, "200")
	operation("/api/v1/auth/session", "get", "Restore session identity and CSRF token", nil, ref(store.Session{}), "200")
	operation("/api/v1/auth/logout", "post", "Revoke server session; requires X-CSRF-Token", nil, statusResponse, "200")
	operation("/api/v1/auth/password", "post", "Change password and revoke sessions and legacy key", object(map[string]any{"current_password": textField, "new_password": textField}), statusResponse, "200")
	operation("/api/v1/admin/users", "get", "Administrator: list staff accounts", nil, array(store.User{}), "200")
	userInput := object(map[string]any{"username": textField, "role": map[string]any{"type": "string", "enum": []string{"physician", "pharmacist", "admin"}}, "active": map[string]any{"type": "boolean"}})
	operation("/api/v1/admin/users", "post", "Administrator: create staff account", userInput, ref(store.User{}), "200")
	operation("/api/v1/admin/users/{id}", "patch", "Administrator: manage staff; protects last active administrator", userInput, ref(store.User{}), "200")
	operation("/api/v1/admin/users/{id}/reset", "post", "Administrator: reset credentials and issue activation secret", nil, object(map[string]any{"activation_secret": textField, "expires_in": textField}), "200")
	for _, p := range []string{"/api/v1/auth/login", "/api/v1/auth/activate"} {
		paths[p].(map[string]any)["post"].(map[string]any)["security"] = []any{}
	}

	operation("/api/v1/staff", "get", "Active physician/pharmacist directory for case sharing", nil, array(store.StaffMember{}), "200")
	operation("/api/v1/patients", "get", "Cursor-paginated authorised synthetic patients; admin reads audited", nil, ref(store.PatientPage{}), "200")
	operation("/api/v1/patients", "post", "Physician: create a confirmed synthetic case", ref(store.PatientInput{}), ref(store.SyntheticPatient{}), "201")
	operation("/api/v1/patients/{id}", "get", "Read an authorised synthetic case", nil, ref(store.SyntheticPatient{}), "200")
	operation("/api/v1/patients/{id}", "patch", "Owner/admin: update alias or archive with expected version", ref(store.PatientUpdate{}), ref(store.SyntheticPatient{}), "200")
	operation("/api/v1/patients/{id}/access", "get", "Owner/admin: inspect explicit case access", nil, array(store.PatientAccess{}), "200")
	operation("/api/v1/patients/{id}/access/{user}", "put", "Owner/admin: grant or revoke complete case history access", object(map[string]any{"granted": map[string]any{"type": "boolean"}}), statusResponse, "200")
	paths["/api/v1/patients/{id}/access/{user}"].(map[string]any)["put"].(map[string]any)["parameters"] = []any{map[string]any{"name": "id", "in": "path", "required": true, "schema": textField}, map[string]any{"name": "user", "in": "path", "required": true, "schema": textField}}
	operation("/api/v1/patients/{id}/observations", "get", "Cursor-paginated immutable observations, including supersession", nil, ref(store.ObservationPage{}), "200")
	operation("/api/v1/patients/{id}/observations", "post", "Physician: append observation or correction, never overwrite", ref(store.ObservationInput{}), ref(store.Observation{}), "201")
	operation("/api/v1/patients/{id}/observations/{observation}/corrections", "post", "Physician: append a correction to the current same-case observation", ref(store.ObservationInput{}), ref(store.Observation{}), "201")
	paths["/api/v1/patients/{id}/observations/{observation}/corrections"].(map[string]any)["post"].(map[string]any)["parameters"] = []any{map[string]any{"name": "id", "in": "path", "required": true, "schema": textField}, map[string]any{"name": "observation", "in": "path", "required": true, "schema": textField}}
	operation("/api/v1/assessments", "get", "Authorised cursor history; legacy creator/admin access; optional unlinked/product/status/from/to", nil, ref(store.AssessmentPage{}), "200")
	operation("/api/v1/patients/{id}/assessments", "get", "Authorised same-case saved result history", nil, ref(store.AssessmentPage{}), "200")
	operation("/api/v1/patients/{id}/assessments", "post", "Physician: atomic lab-linked assessment, audit and idempotency; selected stored lab overrides submitted lab fields", ref(store.PatientAssessmentInput{}), ref(store.AssessmentRecord{}), "201")
	operation("/api/v1/assessments/{id}/export", "get", "Authorised audited JSON report of one saved assessment, at most 4 MiB", nil, ref(store.AssessmentExport{}), "200")
	operation("/api/v1/assessments/{id}/report", "get", "Authorised audited escaped HTML report, use browser Print", nil, map[string]any{"type": "string"}, "200")
	paths["/api/v1/assessments/{id}/report"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"] = map[string]any{"description": "Printable saved result", "content": map[string]any{"text/html": map[string]any{"schema": textField}}}
	for _, path := range []string{"/api/v1/patients", "/api/v1/patients/{id}/observations", "/api/v1/patients/{id}/assessments", "/api/v1/assessments"} {
		op := paths[path].(map[string]any)["get"].(map[string]any)
		params, _ := op["parameters"].([]any)
		params = append(params, map[string]any{"name": "cursor", "in": "query", "schema": textField}, map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "default": 20}})
		if path == "/api/v1/assessments" || path == "/api/v1/patients/{id}/assessments" {
			for _, name := range []string{"product", "status", "from", "to"} {
				params = append(params, map[string]any{"name": name, "in": "query", "schema": textField})
			}
			params = append(params, map[string]any{"name": "unlinked", "in": "query", "schema": map[string]any{"type": "boolean"}})
		}
		op["parameters"] = params
	}

	operation("/api/v1/me", "get", "Authenticated identity and server-bound role", nil, ref(store.Actor{}), "200")
	operation("/api/v1/catalog", "get", "Exact active product/form/route/indication contexts", nil, array(domain.Context{}), "200")
	operation("/api/v1/assessments", "post", "Persist deterministic assessment and audit atomically", ref(domain.Request{}), ref(store.AssessmentRecord{}), "201")
	operation("/api/v1/assessments/{id}", "get", "Read authorised case result; unlinked results use creator/admin access", nil, ref(store.AssessmentRecord{}), "200")
	operation("/api/v1/assessments/{id}/replay", "post", "Replay immutable snapshot with compatible engine version", nil, object(map[string]any{"reproduced": map[string]any{"type": "boolean"}}), "200")
	operation("/api/v1/documents", "get", "List immutable documents and lifecycle status", nil, array(store.Document{}), "200")
	operation("/api/v1/documents/{id}/chunks", "get", "Source text with physical pages and byte offsets", nil, array(store.Chunk{}), "200")
	operation("/api/v1/documents/{id}/pdf", "get", "Download exact pinned source PDF", nil, map[string]any{"type": "string", "format": "binary"}, "200")
	pdfop := paths["/api/v1/documents/{id}/pdf"].(map[string]any)["get"].(map[string]any)
	pdfop["responses"].(map[string]any)["200"] = map[string]any{"description": "Original source PDF", "content": map[string]any{"application/pdf": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}}
	operation("/api/v1/documents", "post", "Pharmacist/admin: upload PDF and declare exact product page scope", nil, ref(store.Document{}), "201")
	paths["/api/v1/documents"].(map[string]any)["post"].(map[string]any)["requestBody"] = map[string]any{"required": true, "content": map[string]any{"multipart/form-data": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"metadata", "file"}, "additionalProperties": false, "properties": map[string]any{"metadata": map[string]any{"type": "string", "contentMediaType": "application/json", "contentSchema": ref(store.DocumentInput{})}, "file": map[string]any{"type": "string", "format": "binary", "description": "PDF, maximum 12 MiB"}}}}}}
	operation("/api/v1/documents/{id}/supersede", "post", "Pharmacist/admin: supersede with same-context active replacement", object(map[string]any{"replacement_id": map[string]any{"type": "string"}}), object(map[string]any{"status": map[string]any{"type": "string"}}), "200")
	operation("/api/v1/documents/{id}/embeddings", "post", "Pharmacist/admin: index with configured local model", nil, object(map[string]any{"indexed_chunks": map[string]any{"type": "integer"}}), "200")
	operation("/api/v1/rules", "get", "List candidates and immutable reviewed rules", nil, array(store.StoredRule{}), "200")
	paths["/api/v1/rules"].(map[string]any)["get"].(map[string]any)["parameters"] = []any{map[string]any{"name": "status", "in": "query", "schema": map[string]any{"type": "string", "enum": []string{"", "PENDING", "APPROVED", "REJECTED", "SUPERSEDED"}}}}
	operation("/api/v1/rules", "post", "Pharmacist/admin: prepare grounded PENDING candidate; server assigns ID/source", ref(store.CandidateInput{}), ref(store.StoredRule{}), "201")
	operation("/api/v1/rules/{id}", "patch", "Edit only PENDING candidate using expected content digest", object(map[string]any{"expected_digest": map[string]any{"type": "string"}, "rule": ref(domain.Rule{})}), ref(store.StoredRule{}), "200")
	for _, action := range []string{"approve", "reject"} {
		operation("/api/v1/rules/{id}/"+action, "post", "Pharmacist only: "+action+" with digest, review note and explicit confirmation", ref(store.ReviewInput{}), ref(store.StoredRule{}), "200")
	}
	operation("/api/v1/evidence/search", "post", "Evidence-only retrieval; never produces personalized action or dose", ref(store.SearchInput{}), ref(store.EvidenceResult{}), "200")
	operation("/api/v1/audit", "get", "Admin only: verify and read append-only audit chain", nil, array(store.AuditEvent{}), "200")
	for _, path := range []string{"/healthz", "/readyz"} {
		operation(path, "get", "Service liveness / database readiness", nil, object(map[string]any{"status": map[string]any{"type": "string"}}), "200")
		paths[path].(map[string]any)["get"].(map[string]any)["security"] = []any{}
	}
	// Identity/version fields in candidates may be omitted; the server sets them.
	ruleSchema := schemas["Rule"].(map[string]any)
	required := []string{}
	for _, v := range ruleSchema["required"].([]string) {
		if v != "id" && v != "version" {
			required = append(required, v)
		}
	}
	ruleSchema["required"] = required
	for _, name := range []string{"SearchInput", "ReviewInput"} {
		obj := schemas[name].(map[string]any)
		if name == "SearchInput" {
			obj["required"] = []string{"context", "query"}
		}
	}
	return map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "RenalSafe local decision and evidence API", "version": "0.2.0", "description": "Academic prototype. Synthetic patients only. Clinical candidates require human pharmacist validation. Extractive evidence cannot choose a dose."}, "servers": []any{map[string]any{"url": "/"}}, "security": []any{map[string]any{"bearerAuth": []any{}}, map[string]any{"sessionAuth": []any{}}}, "paths": paths, "components": map[string]any{"securitySchemes": map[string]any{"sessionAuth": map[string]any{"type": "apiKey", "in": "cookie", "name": sessionCookie}, "bearerAuth": map[string]any{"type": "http", "scheme": "bearer"}}, "schemas": schemas}}
}
