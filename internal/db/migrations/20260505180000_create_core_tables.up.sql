CREATE TABLE apps (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	name text NOT NULL UNIQUE,
	label text NOT NULL,
	version text NOT NULL,
	status text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT apps_status_check CHECK (status IN (
		'installed',
		'active',
		'disabled',
		'pending-install',
		'pending-upgrade',
		'failed'
	))
);

CREATE TABLE entities (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	app_id bigint NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
	name text NOT NULL,
	label text NOT NULL,
	plural_name text NOT NULL,
	plural_label text NOT NULL,
	description text,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (app_id, name),
	UNIQUE (app_id, plural_name)
);

CREATE TABLE fields (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	entity_id bigint NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
	name text NOT NULL,
	label text NOT NULL,
	type text NOT NULL,
	required boolean NOT NULL DEFAULT false,
	is_unique boolean NOT NULL DEFAULT false,
	position integer NOT NULL DEFAULT 0,
	options jsonb NOT NULL DEFAULT '{}'::jsonb,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (entity_id, name)
);

CREATE TABLE users (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	email text NOT NULL UNIQUE,
	full_name text NOT NULL,
	enabled boolean NOT NULL DEFAULT true,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE roles (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	name text NOT NULL UNIQUE,
	label text NOT NULL,
	description text,
	enabled boolean NOT NULL DEFAULT true,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_roles (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	role_id bigint NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
	created_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (user_id, role_id)
);

CREATE TABLE permissions (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	entity_id bigint NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
	role_id bigint NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
	can_read boolean NOT NULL DEFAULT false,
	can_create boolean NOT NULL DEFAULT false,
	can_update boolean NOT NULL DEFAULT false,
	can_delete boolean NOT NULL DEFAULT false,
	can_export boolean NOT NULL DEFAULT false,
	can_print boolean NOT NULL DEFAULT false,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (entity_id, role_id)
);

CREATE TABLE sessions (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status text NOT NULL,
	started_at timestamptz NOT NULL,
	expires_at timestamptz,
	last_seen_at timestamptz,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT sessions_status_check CHECK (status IN ('active', 'expired', 'revoked'))
);
