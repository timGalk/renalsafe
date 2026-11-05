package httpapi

import (
	"bytes"
	"encoding/json"
	"html/template"
	"mime"
	"net/http"
	"renalsafe/core/internal/store"
	"strconv"
	"time"
)

func pageQuery(r *http.Request) (string, int, error) {
	raw := r.URL.Query().Get("limit")
	n := 0
	var e error
	if raw != "" {
		n, e = strconv.Atoi(raw)
		if e != nil {
			return "", 0, store.ErrInvalid
		}
	}
	return r.URL.Query().Get("cursor"), n, nil
}
func historyQuery(r *http.Request) (store.HistoryFilter, error) {
	raw, n, e := pageQuery(r)
	f := store.HistoryFilter{Cursor: raw, Limit: n, PatientID: r.PathValue("id"), Product: r.URL.Query().Get("product"), Status: r.URL.Query().Get("status")}
	if e != nil {
		return f, e
	}
	u := r.URL.Query().Get("unlinked")
	if u != "" && u != "true" && u != "false" {
		return f, store.ErrInvalid
	}
	f.Unlinked = u == "true"
	for key, dest := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
		if value := r.URL.Query().Get(key); value != "" {
			t, e := time.Parse(time.RFC3339Nano, value)
			if e != nil {
				return f, store.ErrInvalid
			}
			*dest = &t
		}
	}
	return f, nil
}
func (s *Server) patientRoutes(api func(string, string, func(http.ResponseWriter, *http.Request, store.Actor) error)) {
	all := "physician pharmacist admin"
	api("GET /api/v1/staff", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.StaffDirectory(r.Context(), a)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("GET /api/v1/patients", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		cursor, n, e := pageQuery(r)
		if e != nil {
			return e
		}
		out, e := s.DB.Patients(r.Context(), a, cursor, n)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("POST /api/v1/patients", "physician", func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in store.PatientInput
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		out, e := s.DB.CreatePatient(r.Context(), a, in)
		if e != nil {
			return e
		}
		w.Header().Set("Location", "/api/v1/patients/"+out.ID)
		return respond(w, 201, out)
	})
	api("GET /api/v1/patients/{id}", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.Patient(r.Context(), a, r.PathValue("id"))
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("PATCH /api/v1/patients/{id}", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in store.PatientUpdate
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		out, e := s.DB.UpdatePatient(r.Context(), a, r.PathValue("id"), in)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("GET /api/v1/patients/{id}/access", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.PatientAccess(r.Context(), a, r.PathValue("id"))
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("PUT /api/v1/patients/{id}/access/{user}", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in struct {
			Granted *bool `json:"granted"`
		}
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		if in.Granted == nil {
			return store.ErrInvalid
		}
		if e := s.DB.SetPatientAccess(r.Context(), a, r.PathValue("id"), r.PathValue("user"), *in.Granted); e != nil {
			return e
		}
		return respond(w, 200, map[string]string{"status": "access_updated"})
	})
	api("GET /api/v1/patients/{id}/observations", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		cursor, n, e := pageQuery(r)
		if e != nil {
			return e
		}
		out, e := s.DB.Observations(r.Context(), a, r.PathValue("id"), cursor, n)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	addObservation := func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in store.ObservationInput
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		if correction := r.PathValue("observation"); correction != "" {
			if in.CorrectsID != "" && in.CorrectsID != correction {
				return store.ErrInvalid
			}
			in.CorrectsID = correction
		}
		out, e := s.DB.AddObservation(r.Context(), a, r.PathValue("id"), in)
		if e != nil {
			return e
		}
		return respond(w, 201, out)
	}
	api("POST /api/v1/patients/{id}/observations", "physician", addObservation)
	api("POST /api/v1/patients/{id}/observations/{observation}/corrections", "physician", addObservation)
	history := func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		f, e := historyQuery(r)
		if e != nil {
			return e
		}
		out, e := s.DB.AssessmentHistory(r.Context(), a, f)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	}
	api("GET /api/v1/assessments", all, history)
	api("GET /api/v1/patients/{id}/assessments", all, history)
	api("POST /api/v1/patients/{id}/assessments", "physician", func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in store.PatientAssessmentInput
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		out, e := s.DB.AssessPatient(r.Context(), a, r.PathValue("id"), in)
		if e != nil {
			return e
		}
		w.Header().Set("Location", "/api/v1/assessments/"+out.ID)
		return respond(w, 201, out)
	})
	api("GET /api/v1/assessments/{id}/export", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.ExportAssessment(r.Context(), a, r.PathValue("id"))
		if e != nil {
			return e
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "assessment-" + out.ID + ".json"}))
		return respond(w, 200, out)
	})
	api("GET /api/v1/assessments/{id}/report", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.ExportAssessment(r.Context(), a, r.PathValue("id"))
		if e != nil {
			return e
		}
		var buf bytes.Buffer
		if e = reportTemplate.Execute(&buf, out); e != nil {
			return e
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": "assessment-" + out.ID + ".html"}))
		_, e = w.Write(buf.Bytes())
		return e
	})
}

var reportTemplate = template.Must(template.New("report").Funcs(template.FuncMap{"json": func(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }}).Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>RenalSafe assessment report</title></head><body><main>
<h1>RenalSafe assessment report</h1><p>Academic synthetic case. This report addresses renal criteria only. Manual-review outcomes contain no dose recommendation. Use the browser Print command to print or save this report.</p>
<dl><dt>Synthetic patient</dt><dd>{{if .PatientCode}}{{.PatientCode}}{{else}}Unlinked assessment{{end}}</dd><dt>Assessment ID</dt><dd>{{.ID}}</dd><dt>Saved at</dt><dd>{{.CreatedAt}}</dd><dt>Author ID</dt><dd>{{.Actor.ID}}</dd><dt>Observation ID</dt><dd>{{.ObservationID}}</dd><dt>Previous assessment</dt><dd>{{.PreviousID}}</dd><dt>Engine / policy / knowledge</dt><dd>{{.Assessment.EngineVersion}} / {{.Assessment.PolicyVersion}} / {{.Assessment.KnowledgeVersion}}</dd><dt>Knowledge digest</dt><dd>{{.Assessment.KnowledgeDigest}}</dd><dt>Audit hash</dt><dd>{{.AuditHash}}</dd></dl>
<h2>Renal estimates</h2>{{range .Assessment.Renal.Estimates}}<p>{{.Metric}}: {{.Value}} {{.Unit}}</p>{{end}}{{range .Assessment.Renal.Warnings}}<p>{{.}}</p>{{end}}
<h2>Medicine results and citations</h2>{{range .Assessment.Results}}<article><h3>{{.Context.Product}}</h3><p>{{.Context.Form}} · {{.Context.Route}} · {{.Context.Indication}}</p><p>{{.Status}} {{.Action}}</p><p>{{.DoseText}}</p><p>{{.Explanation}}</p>{{range .Reasons}}<p>{{.}}</p>{{end}}{{if .Rule}}<blockquote>{{.Rule.Citation.Quote}}</blockquote><p>Section {{.Rule.Citation.Section}}, page {{.Rule.Citation.Page}}, source {{.Rule.Citation.SourceID}}, version {{.Rule.Citation.SourceVersion}}, SHA-256 {{.Rule.Citation.DocumentSHA256}}</p>{{end}}{{if .Source}}<p>Source: {{.Source.URL}}</p>{{end}}{{if .PrescriptionCheck}}<p>Prescription check: {{.PrescriptionCheck.Status}}</p>{{range .PrescriptionCheck.Reasons}}<p>{{.}}</p>{{end}}{{end}}</article>{{end}}
<h2>Exact saved assessment inputs</h2><pre>{{json .Assessment.Request}}</pre></main></body></html>`))
