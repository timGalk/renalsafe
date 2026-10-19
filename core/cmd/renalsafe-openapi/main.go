// RenalSafe OpenAPI writes the current JSON API contract to stdout.
package main

import (
	"encoding/json"
	"os"
	"renalsafe/core/internal/httpapi"
)

func main() {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if e := enc.Encode(httpapi.OpenAPI()); e != nil {
		panic(e)
	}
}
