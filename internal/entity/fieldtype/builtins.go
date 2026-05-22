package fieldtype

func builtIns() []Definition {
	return []Definition{
		// Scalar and simple fields.
		scalar("text", "Text", true, true, true, true),
		scalar("email", "Email", true, true, true, true),
		scalar("phone", "Phone", true, true, true, true),
		scalar("long-text", "Long Text", true, false, true, false),
		scalar("int", "Integer", true, true, true, true),
		scalar("bigint", "Big Integer", true, true, true, true),
		scalar("decimal", "Decimal", true, true, true, true),
		scalar("currency", "Currency", true, true, true, true),
		scalar("boolean", "Boolean", true, true, true, true),
		scalar("date", "Date", true, true, true, true),
		scalar("datetime", "Datetime", true, true, true, true),
		scalar("time", "Time", true, true, true, true),

		// Sensitive fields.
		scalar("password", "Password", true, false, false, false),

		// Option-backed fields.
		{
			Name:          "select",
			Label:         "Select",
			AllowRequired: true,
			AllowUnique:   true,
			AllowDefault:  true,
			AllowIndex:    true,
			Validate:      SelectOptions,
		},

		// Relationship fields.
		{
			Name:          "link",
			Label:         "Link",
			AllowRequired: true,
			AllowUnique:   true,
			AllowDefault:  true,
			AllowIndex:    true,
			Validate:      EntityOptions,
		},
		{
			Name:          "collection",
			Label:         "Record Collection",
			AllowRequired: true,
			AllowUnique:   false,
			AllowDefault:  false,
			AllowIndex:    false,
			Validate:      EntityOptions,
		},

		// Structured and blob-like fields.
		scalar("attachment", "Attachment", true, false, false, false),
		scalar("json", "JSON", true, false, false, false),
	}
}

func scalar(name string, label string, allowRequired bool, allowUnique bool, allowDefault bool, allowIndex bool) Definition {
	return Definition{
		Name:          name,
		Label:         label,
		AllowRequired: allowRequired,
		AllowUnique:   allowUnique,
		AllowDefault:  allowDefault,
		AllowIndex:    allowIndex,
		Validate:      NoOptions,
	}
}
