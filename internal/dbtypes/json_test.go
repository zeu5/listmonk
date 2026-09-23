package dbtypes

import "testing"

func TestRawJSONScan(t *testing.T) {
	for _, value := range []any{`{"ok":true}`, []byte(`{"ok":true}`)} {
		var got RawJSON
		if err := got.Scan(value); err != nil {
			t.Fatal(err)
		}
		if string(got) != `{"ok":true}` {
			t.Fatalf("got %s", got)
		}
	}
}
