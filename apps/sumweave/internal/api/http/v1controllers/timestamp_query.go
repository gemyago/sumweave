package v1controllers

import (
	"net/http"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/app"
)

func parseOptionalTimestampQuery(req *http.Request, name, value string) (time.Time, bool, error) {
	if _, supplied := req.URL.Query()[name]; !supplied {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, true, app.NewErrInvalidInput(name, "must be an RFC3339 timestamp")
	}
	return parsed, true, nil
}
