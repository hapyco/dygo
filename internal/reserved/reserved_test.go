package reserved

import (
	"reflect"
	"sort"
	"testing"
)

func TestReservedWords(t *testing.T) {
	if !IsApp("CORE") || !IsApp(" studio ") || !IsApp("Localization") {
		t.Fatal("IsApp() did not normalize reserved app names")
	}
	if IsApp("sales") {
		t.Fatal("IsApp(sales) = true, want false")
	}
	if !IsSlug("API") || !IsSlug(" setup ") {
		t.Fatal("IsSlug() did not normalize reserved root slugs")
	}
	if IsSlug("settings") {
		t.Fatal("IsSlug(settings) = true, want false")
	}
	if !IsField("created-at") || !IsField("created-by") || !IsField("modified-by") || !IsField("owner") || !IsField("fields") || !IsField("Q") {
		t.Fatal("IsField() did not include system/query reserved field names")
	}
	if !IsQuery("limit") || !IsQuery("fields") || !IsQuery("cursor") {
		t.Fatal("IsQuery() did not include reserved query params")
	}
}

func TestReservedListsAreSortedCopies(t *testing.T) {
	nonEmptyTests := [][]string{
		Apps(),
		Slugs(),
		Fields(),
		Queries(),
	}
	for _, values := range nonEmptyTests {
		sorted := append([]string(nil), values...)
		sort.Strings(sorted)
		if !reflect.DeepEqual(values, sorted) {
			t.Fatalf("reserved list = %#v, want sorted", values)
		}
		if len(values) == 0 {
			t.Fatal("reserved list is empty")
		}
		values[0] = "mutated"
	}
	if IsApp("mutated") || IsSlug("mutated") || IsField("mutated") || IsQuery("mutated") {
		t.Fatal("reserved list mutation changed package state")
	}
}
