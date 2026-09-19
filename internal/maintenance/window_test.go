package maintenance

import (
	"testing"
	"time"
)

func at(day time.Weekday, hour, minute int) time.Time {
	base := time.Date(2026, time.September, 13, hour, minute, 0, 0, time.UTC)
	return base.AddDate(0, 0, int(day)-int(base.Weekday()))
}

func TestParse(t *testing.T) {
	w, err := Parse("Sat 02:00-05:00")
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Days) != 1 || w.Days[0] != time.Saturday || w.Start != 120 || w.End != 300 {
		t.Fatalf("window %+v", w)
	}
	w, err = Parse("Sat,Sun 22:00-23:30")
	if err != nil || len(w.Days) != 2 || w.End != 1410 {
		t.Fatalf("window %+v err %v", w, err)
	}
	for _, bad := range []string{"", "Sat", "Sat 02:00", "Sat 05:00-02:00", "Someday 02:00-05:00", "Sat 25:00-26:00", "Sat 02:00-02:00"} {
		if _, err := Parse(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestOpen(t *testing.T) {
	w, _ := Parse("Sat 02:00-05:00")
	if !w.Open(at(time.Saturday, 2, 0)) || !w.Open(at(time.Saturday, 4, 59)) {
		t.Fatal("inside window must be open")
	}
	if w.Open(at(time.Saturday, 5, 0)) || w.Open(at(time.Saturday, 1, 59)) || w.Open(at(time.Sunday, 3, 0)) {
		t.Fatal("outside window must be closed")
	}
	if (Window{}).Open(at(time.Saturday, 3, 0)) {
		t.Fatal("empty window is never open")
	}
}
