package extraction

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
)

// Export the exact public-document request body for an independent HTTP-client
// comparison. No real credential, header or network connection is used.
func TestExportNVIDIADiagnosticRequest(t *testing.T) {
	sourcePath, targetPath := os.Getenv("RENALSAFE_EXPORT_SOURCE"), os.Getenv("RENALSAFE_EXPORT_REQUEST")
	if sourcePath == "" || targetPath == "" {
		t.Skip("explicit request-export diagnostic only")
	}
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	var input SourceInput
	if err = json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	p, err := NewNVIDIA("diagnostic-placeholder", os.Getenv("RENALSAFE_NVIDIA_MODEL"))
	if err != nil {
		t.Fatal(err)
	}
	written := false
	p.client.Transport = extractionTransport(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(targetPath, body, 0600); err != nil {
			t.Fatal(err)
		}
		written = true
		return nil, errors.New("diagnostic export; no network request")
	})
	_, _ = p.Extract(t.Context(), input)
	if !written {
		t.Fatal("request was not generated")
	}
}
