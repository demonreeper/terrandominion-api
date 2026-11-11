package httpapi

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadFixKeepsTheBody(t *testing.T) {
	body := `{"weapon":"em_pulse","lat":4.17,"lon":73.5,"accuracyM":8,"mock":true,"fixAt":"2026-11-11T10:00:00Z"}`
	r := httptest.NewRequest("POST", "/v1/tethers/1/attack", strings.NewReader(body))
	fix, err := readFix(r)
	if err != nil || fix.Lat != 4.17 || fix.Lon != 73.5 || fix.AccuracyM != 8 || !fix.Mock || fix.FixAt == nil {
		t.Fatalf("fix not read: %+v %v", fix, err)
	}
	rest, _ := io.ReadAll(r.Body)
	if string(rest) != body {
		t.Error("the handler must still read the whole body")
	}
}

func TestReadFixFromQuery(t *testing.T) {
	r := httptest.NewRequest("GET", "/v1/nodes/nearby?lat=4.1&lon=73.4&accuracy=12&mock=1", nil)
	fix, _ := readFix(r)
	if fix.Lat != 4.1 || fix.AccuracyM != 12 || !fix.Mock {
		t.Errorf("query fix not read: %+v", fix)
	}
	take := httptest.NewRequest("POST", "/v1/jobs/3/take", nil)
	if fix, _ := readFix(take); fix.HasPosition() {
		t.Error("a job take names no place")
	}
}
