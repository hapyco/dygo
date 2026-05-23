package fieldtype

func builtIns() []Definition {
	return []Definition{
		// Scalar and simple fields.
		scalar("text", "Text", "text", "", ValueString, "text", "text", true, true, true, true, true, true, true),
		scalar("email", "Email", "text", "", ValueString, "email", "email", true, true, true, true, true, true, true),
		scalar("phone", "Phone", "text", "", ValueString, "text", "phone", true, true, true, true, true, true, true),
		scalar("long-text", "Long Text", "text", "", ValueString, "textarea", "text", true, false, true, false, false, false, true),
		scalar("int", "Integer", "integer", "integer", ValueInteger, "number", "number", true, true, true, true, true, false, true),
		scalar("bigint", "Big Integer", "bigint", "bigint", ValueInteger, "number", "number", true, true, true, true, true, false, true),
		scalar("decimal", "Decimal", "numeric", "numeric", ValueNumber, "number", "number", true, true, true, true, true, false, true),
		scalar("currency", "Currency", "numeric", "numeric", ValueNumber, "number", "currency", true, true, true, true, true, false, true),
		scalar("boolean", "Boolean", "boolean", "boolean", ValueBoolean, "switch", "boolean", true, true, true, true, true, false, true),
		scalar("date", "Date", "date", "date", ValueDate, "date", "date", true, true, true, true, true, false, true),
		scalar("datetime", "Datetime", "timestamptz", "timestamptz", ValueDatetime, "datetime", "datetime", true, true, true, true, true, false, true),
		scalar("time", "Time", "time", "time", ValueTime, "time", "time", true, true, true, true, true, false, true),

		// Sensitive fields.
		{
			Name:          "password",
			Label:         "Password",
			AllowRequired: true,
			Behavior: Behavior{
				Stored:        true,
				ColumnSuffix:  "_hash",
				SQLType:       "text",
				ValueKind:     ValuePassword,
				WriteOnly:     true,
				StudioEditor:  "password",
				StudioDisplay: "hidden",
			},
			Validate: NoOptions,
		},

		// Option-backed fields.
		{
			Name:          "select",
			Label:         "Select",
			AllowRequired: true,
			AllowUnique:   true,
			AllowDefault:  true,
			AllowIndex:    true,
			Behavior:      storedBehavior("text", "", ValueString, "select", "text", true, true, true),
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
			Behavior: Behavior{
				Stored:          true,
				ColumnSuffix:    "_id",
				SQLType:         "bigint",
				PlaceholderCast: "bigint",
				ValueKind:       ValueInteger,
				Listable:        true,
				NameRenderable:  true,
				StudioEditor:    "link",
				StudioDisplay:   "link",
			},
			Validate: EntityOptions,
		},
		{
			Name:          "collection",
			Label:         "Record Collection",
			AllowRequired: true,
			AllowUnique:   false,
			AllowDefault:  false,
			AllowIndex:    false,
			Behavior: Behavior{
				StudioEditor:  "collection",
				StudioDisplay: "collection",
			},
			Validate: EntityOptions,
		},

		// Structured and blob-like fields.
		scalar("attachment", "Attachment", "text", "", ValueString, "attachment", "attachment", true, false, false, false, false, false, false),
		scalar("json", "JSON", "jsonb", "jsonb", ValueJSON, "json", "json", true, false, false, false, false, false, false),
	}
}

func scalar(name string, label string, sqlType string, placeholderCast string, valueKind string, studioEditor string, studioDisplay string, allowRequired bool, allowUnique bool, allowDefault bool, allowIndex bool, nameRenderable bool, systemName bool, checkable bool) Definition {
	return Definition{
		Name:          name,
		Label:         label,
		AllowRequired: allowRequired,
		AllowUnique:   allowUnique,
		AllowDefault:  allowDefault,
		AllowIndex:    allowIndex,
		Behavior:      storedBehavior(sqlType, placeholderCast, valueKind, studioEditor, studioDisplay, nameRenderable, systemName, checkable),
		Validate:      NoOptions,
	}
}

func storedBehavior(sqlType string, placeholderCast string, valueKind string, studioEditor string, studioDisplay string, nameRenderable bool, systemName bool, checkable bool) Behavior {
	return Behavior{
		Stored:          true,
		SQLType:         sqlType,
		PlaceholderCast: placeholderCast,
		ValueKind:       valueKind,
		Listable:        true,
		NameRenderable:  nameRenderable,
		SystemName:      systemName,
		Checkable:       checkable,
		StudioEditor:    studioEditor,
		StudioDisplay:   studioDisplay,
	}
}
