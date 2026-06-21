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

func TestToastJSONIncludesDuration(t *testing.T) {
	duration := 4000
	data, err := json.Marshal(Toast{
		Title:    "Saved",
		Type:     ToastSuccess,
		Duration: &duration,
	})
	if err != nil {
		t.Fatalf("Marshal(Toast) error = %v", err)
	}

	want := `{"title":"Saved","type":"success","duration":4000}`
	if string(data) != want {
		t.Fatalf("Toast JSON = %s, want %s", data, want)
	}
}

func TestToastJSONIncludesExplicitZeroDuration(t *testing.T) {
	duration := 0
	data, err := json.Marshal(Toast{
		Title:    "Stay open",
		Duration: &duration,
	})
	if err != nil {
		t.Fatalf("Marshal(Toast) error = %v", err)
	}

	want := `{"title":"Stay open","duration":0}`
	if string(data) != want {
		t.Fatalf("Toast JSON = %s, want %s", data, want)
	}
}
