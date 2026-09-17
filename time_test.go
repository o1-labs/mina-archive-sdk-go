package archive

import (
	"encoding/json"
	"testing"
	"time"
)

// BlockInfo.Timestamp is Unix epoch milliseconds as a decimal string, while
// Block.DateTime a few fields away is ISO-8601 (#12). Pin both, and pin the
// fact that the obvious call on the first one fails.
func TestBlockInfoTime(t *testing.T) {
	bi := BlockInfo{Timestamp: "1692054601000"}
	got, err := bi.Time()
	if err != nil {
		t.Fatalf("Time() error: %v", err)
	}
	// NB: #12's acceptance criterion says 2023-08-15T00:30:01Z. That is wrong —
	// it disagrees with new Date(1692054601000).toISOString() and with the
	// audit brief's own table, both of which give 2023-08-14T23:10:01Z.
	if want := "2023-08-14T23:10:01Z"; got.Format(time.RFC3339) != want {
		t.Errorf("Time() = %s, want %s", got.Format(time.RFC3339), want)
	}

	// The wrong call the doc comment warns about.
	if _, err := time.Parse(time.RFC3339, bi.Timestamp); err == nil {
		t.Error("expected RFC3339 parsing of a millisecond timestamp to fail")
	}
	// Reading it as seconds lands in the far future, not 2023.
	if time.Unix(1692054601000, 0).UTC().Year() == 2023 {
		t.Error("expected seconds-interpretation to be wrong")
	}
}

func TestBlockInfoTimeRejectsISO8601(t *testing.T) {
	bi := BlockInfo{Timestamp: "2023-08-14T23:10:01.000Z"}
	if _, err := bi.Time(); err == nil {
		t.Error("expected an error for an ISO-8601 value in Timestamp")
	}
}

func TestBlockTime(t *testing.T) {
	b := Block{DateTime: "2023-08-14T23:10:01.000Z"}
	got, err := b.Time()
	if err != nil {
		t.Fatalf("Time() error: %v", err)
	}
	if got.UnixMilli() != 1692054601000 {
		t.Errorf("UnixMilli() = %d, want 1692054601000", got.UnixMilli())
	}
}

// The point of DateTimeFilter: the server runs new Date(v).getTime(), and a
// value it cannot parse becomes NaN and silently matches nothing. The emitted
// string must be one that survives that round trip.
func TestDateTimeFilterRoundTrips(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{1691971200000, "2023-08-14T00:00:00.000Z"},
		{1692054601000, "2023-08-14T23:10:01.000Z"},
		{0, "1970-01-01T00:00:00.000Z"},
	}
	for _, c := range cases {
		got := DateTimeFilter(time.UnixMilli(c.ms))
		if got != c.want {
			t.Errorf("DateTimeFilter(%d) = %q, want %q", c.ms, got, c.want)
		}
		// Parseable as RFC3339 means new Date() gets a finite number.
		back, err := time.Parse(time.RFC3339, got)
		if err != nil {
			t.Errorf("DateTimeFilter(%d) produced an unparseable value %q: %v", c.ms, got, err)
		} else if back.UnixMilli() != c.ms {
			t.Errorf("round trip: %d -> %q -> %d", c.ms, got, back.UnixMilli())
		}
	}
}

// A non-UTC input must still serialise to a UTC instant.
func TestDateTimeFilterNormalisesToUTC(t *testing.T) {
	loc := time.FixedZone("UTC+5", 5*60*60)
	got := DateTimeFilter(time.UnixMilli(1691971200000).In(loc))
	if want := "2023-08-14T00:00:00.000Z"; got != want {
		t.Errorf("DateTimeFilter = %q, want %q", got, want)
	}
}

func TestSetDateTimeRange(t *testing.T) {
	var in BlockQueryInput
	in.SetDateTimeRange(time.UnixMilli(1691971200000), time.UnixMilli(1692054601000))

	m, err := in.toMap("TestSetDateTimeRange")
	if err != nil {
		t.Fatalf("toMap: %v", err)
	}
	if got := m["dateTime_gte"]; got != "2023-08-14T00:00:00.000Z" {
		t.Errorf("dateTime_gte = %v", got)
	}
	if got := m["dateTime_lt"]; got != "2023-08-14T23:10:01.000Z" {
		t.Errorf("dateTime_lt = %v", got)
	}

	// A zero time leaves that bound alone, so it stays out of the payload.
	var only BlockQueryInput
	only.SetDateTimeRange(time.UnixMilli(1691971200000), time.Time{})
	m2, err := only.toMap("TestSetDateTimeRange")
	if err != nil {
		t.Fatalf("toMap: %v", err)
	}
	if _, ok := m2["dateTime_lt"]; ok {
		t.Error("a zero upper bound should not be sent")
	}

	// Sanity: the map really is what goes on the wire.
	if _, err := json.Marshal(m); err != nil {
		t.Fatal(err)
	}
}
