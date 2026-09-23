package models

import "testing"

func TestJSONScannersAcceptTextAndBytes(t *testing.T) {
	for _, source := range []any{`{"name":"sqlite"}`, []byte(`{"name":"postgres"}`)} {
		value := JSON{}
		if err := value.Scan(source); err != nil {
			t.Fatal(err)
		}
		if value["name"] == nil {
			t.Fatalf("decoded value=%#v", value)
		}
	}
	for _, source := range []any{`{"confirmed":2}`, []byte(`{"confirmed":3}`)} {
		value := StringIntMap{}
		if err := value.Scan(source); err != nil {
			t.Fatal(err)
		}
		if value["confirmed"] == 0 {
			t.Fatalf("decoded value=%#v", value)
		}
	}
}
