package dygo

import (
	"encoding/json"
	"testing"
)

func TestDialogJSONIncludesExplicitFalseDismissible(t *testing.T) {
	dismissible := false
	data, err := json.Marshal(Dialog{
		Title:       "Delete invoice?",
		Dismissible: &dismissible,
		Actions: []DialogAction{{
			Key:     "confirm",
			Label:   "Delete",
			Variant: DialogActionDanger,
		}},
	})
	if err != nil {
		t.Fatalf("Marshal(Dialog) error = %v", err)
	}

	want := `{"title":"Delete invoice?","actions":[{"key":"confirm","label":"Delete","variant":"danger"}],"dismissible":false}`
	if string(data) != want {
		t.Fatalf("Dialog JSON = %s, want %s", data, want)
	}
}
