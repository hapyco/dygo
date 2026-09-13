--
-- PostgreSQL database dump
--

\restrict dygoschemasnapshot

-- Dumped from database version 18.6 (Postgres.app)
-- Dumped by pg_dump version 18.6 (Postgres.app)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: activity; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.activity (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    kind text DEFAULT 'record'::text NOT NULL,
    operation text NOT NULL,
    status text DEFAULT 'success'::text NOT NULL,
    entity_id bigint,
    record_id bigint,
    actor_id bigint,
    title text NOT NULL,
    message text,
    changes jsonb,
    snapshot jsonb,
    details jsonb,
    CONSTRAINT activity_kind_check CHECK ((kind = ANY (ARRAY['record'::text, 'comment'::text, 'workflow'::text, 'job'::text, 'email'::text, 'attachment'::text, 'auth'::text, 'system'::text, 'action'::text]))),
    CONSTRAINT activity_operation_check CHECK ((operation = ANY (ARRAY['create'::text, 'update'::text, 'delete'::text, 'restore'::text, 'comment'::text, 'workflow-transition'::text, 'job-completed'::text, 'email-sent'::text, 'attachment-added'::text, 'login'::text, 'logout'::text, 'system'::text, 'action'::text]))),
    CONSTRAINT activity_status_check CHECK ((status = ANY (ARRAY['success'::text, 'failed'::text])))
);


--
-- Name: activity_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.activity ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.activity_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: app; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.app (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    label text NOT NULL,
    version text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    CONSTRAINT app_status_check CHECK ((status = ANY (ARRAY['installed'::text, 'active'::text, 'disabled'::text, 'pending-install'::text, 'pending-upgrade'::text, 'failed'::text])))
);


--
-- Name: app_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.app ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.app_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: configuration; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.configuration (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    home text DEFAULT '/'::text NOT NULL,
    country_id bigint,
    language_id bigint,
    currency_id bigint,
    CONSTRAINT configuration_single_check CHECK ((name = 'configuration'::text))
);


--
-- Name: configuration_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.configuration ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.configuration_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: constraint; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."constraint" (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    entity_id bigint NOT NULL,
    constraint_name text NOT NULL,
    type text NOT NULL,
    field_names jsonb,
    field text,
    operator text,
    value jsonb,
    "position" integer,
    retired boolean DEFAULT false NOT NULL,
    CONSTRAINT constraint_operator_check CHECK ((operator = ANY (ARRAY['eq'::text, 'neq'::text, 'gt'::text, 'gte'::text, 'lt'::text, 'lte'::text, 'in'::text, 'not-in'::text]))),
    CONSTRAINT constraint_type_check CHECK ((type = ANY (ARRAY['unique'::text, 'check'::text, 'exactly-one'::text])))
);


--
-- Name: constraint_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public."constraint" ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.constraint_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: country; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.country (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    code text NOT NULL
);


--
-- Name: country_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.country ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.country_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: currency; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.currency (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    code text NOT NULL,
    numeric_code text,
    display_name text,
    symbol text,
    minor_unit_digits integer DEFAULT 2,
    cash_rounding_increment numeric,
    enabled boolean DEFAULT true
);


--
-- Name: currency_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.currency ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.currency_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: entity; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.entity (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    tree jsonb,
    app_id bigint NOT NULL,
    key text NOT NULL,
    slug text,
    label text NOT NULL,
    description text,
    icon text,
    is_single boolean DEFAULT false NOT NULL,
    is_system boolean DEFAULT false NOT NULL,
    is_collection boolean DEFAULT false NOT NULL,
    is_private boolean DEFAULT false NOT NULL,
    private_owner_field text,
    naming jsonb,
    retired boolean DEFAULT false NOT NULL
);


--
-- Name: entity_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.entity ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.entity_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: field; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.field (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    entity_id bigint NOT NULL,
    field_name text NOT NULL,
    label text NOT NULL,
    type text NOT NULL,
    required boolean DEFAULT false,
    "unique" boolean DEFAULT false,
    index boolean DEFAULT false,
    "default" jsonb,
    "check" jsonb,
    "fetch" jsonb,
    "position" integer,
    options jsonb,
    retired boolean DEFAULT false NOT NULL
);


--
-- Name: field_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.field ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.field_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: file; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.file (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    filename text NOT NULL,
    storage_key text NOT NULL,
    checksum text NOT NULL,
    content_type text NOT NULL,
    size bigint NOT NULL,
    private boolean DEFAULT true NOT NULL,
    actor_id bigint NOT NULL,
    retired boolean DEFAULT false NOT NULL,
    app text,
    entity text,
    record_id bigint,
    field text
);


--
-- Name: file_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.file ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.file_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: import; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.import (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    app text NOT NULL,
    entity text NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    actor_id bigint,
    headers jsonb NOT NULL,
    total_rows integer DEFAULT 0 NOT NULL,
    processed_rows integer DEFAULT 0 NOT NULL,
    succeeded_rows integer DEFAULT 0 NOT NULL,
    failed_rows integer DEFAULT 0 NOT NULL,
    error text,
    CONSTRAINT import_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text])))
);


--
-- Name: import_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.import ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.import_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: import_row; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.import_row (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    import_id bigint NOT NULL,
    row_number integer NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    data jsonb NOT NULL,
    error text,
    record_id bigint,
    CONSTRAINT import_row_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text])))
);


--
-- Name: import_row_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.import_row ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.import_row_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: index; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.index (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    entity_id bigint NOT NULL,
    index_name text NOT NULL,
    field_names jsonb NOT NULL,
    "position" integer,
    retired boolean DEFAULT false NOT NULL
);


--
-- Name: index_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.index ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.index_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: job; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.job (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    app_id bigint NOT NULL,
    key text NOT NULL,
    source text DEFAULT 'file'::text NOT NULL,
    label text NOT NULL,
    description text,
    queue text DEFAULT 'default'::text NOT NULL,
    timeout text NOT NULL,
    retry jsonb,
    enabled boolean DEFAULT true NOT NULL,
    retired boolean DEFAULT false NOT NULL,
    CONSTRAINT job_source_check CHECK ((source = ANY (ARRAY['file'::text, 'studio'::text, 'system'::text])))
);


--
-- Name: job_execution; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.job_execution (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    job_id bigint NOT NULL,
    app_name text NOT NULL,
    job_name text NOT NULL,
    queue text NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    priority integer DEFAULT 0,
    payload jsonb,
    result jsonb,
    error text,
    attempts integer DEFAULT 0 NOT NULL,
    max_attempts integer NOT NULL,
    retry jsonb,
    run_after timestamp with time zone NOT NULL,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    locked_by text,
    locked_until timestamp with time zone,
    idempotency_key text,
    actor_id bigint,
    CONSTRAINT job_execution_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text, 'cancelled'::text])))
);


--
-- Name: job_execution_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.job_execution ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.job_execution_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: job_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.job ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: language; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.language (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    code text NOT NULL,
    enabled boolean DEFAULT true
);


--
-- Name: language_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.language ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.language_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.log (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    type text NOT NULL,
    source text NOT NULL,
    app_id bigint,
    title text NOT NULL,
    message text,
    trace_id text,
    reference_entity_id bigint,
    reference_record_id bigint,
    reference_record_name text,
    actor_id bigint,
    metadata jsonb,
    CONSTRAINT log_source_check CHECK ((source = ANY (ARRAY['Framework'::text, 'SDK'::text, 'HTTP'::text, 'Job'::text, 'Hook'::text, 'CLI'::text, 'Studio'::text]))),
    CONSTRAINT log_type_check CHECK ((type = ANY (ARRAY['Debug'::text, 'Info'::text, 'Warning'::text, 'Error'::text, 'Panic'::text])))
);


--
-- Name: log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.log ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: naming_series; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.naming_series (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    entity_id bigint NOT NULL,
    key text NOT NULL,
    pattern text NOT NULL,
    current bigint DEFAULT 0 NOT NULL
);


--
-- Name: naming_series_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.naming_series ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.naming_series_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: notification; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    recipient_id bigint NOT NULL,
    title text NOT NULL,
    message text NOT NULL,
    deep_link text DEFAULT '/'::text NOT NULL,
    read_at timestamp with time zone,
    send_email boolean DEFAULT false NOT NULL,
    emailed_at timestamp with time zone,
    idempotency_key text NOT NULL
);


--
-- Name: notification_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.notification ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.notification_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: page; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.page (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    app_id bigint NOT NULL,
    key text NOT NULL,
    source text DEFAULT 'file'::text NOT NULL,
    label text NOT NULL,
    description text,
    icon text,
    path text NOT NULL,
    renderer text DEFAULT 'entity-index'::text NOT NULL,
    options jsonb,
    retired boolean DEFAULT false NOT NULL,
    CONSTRAINT page_renderer_check CHECK ((renderer = 'entity-index'::text)),
    CONSTRAINT page_source_check CHECK ((source = ANY (ARRAY['file'::text, 'studio'::text, 'system'::text])))
);


--
-- Name: page_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.page ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.page_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: patch_run; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.patch_run (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    app_id bigint NOT NULL,
    patch_id text NOT NULL,
    path text NOT NULL,
    phase text NOT NULL,
    checksum text NOT NULL,
    applied_at timestamp with time zone NOT NULL,
    dygo_version text,
    CONSTRAINT patch_run_phase_check CHECK ((phase = ANY (ARRAY['pre-sync'::text, 'post-sync'::text])))
);


--
-- Name: patch_run_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.patch_run ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.patch_run_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: permission; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permission (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    entity_id bigint,
    page_id bigint,
    role_id bigint NOT NULL,
    read boolean DEFAULT false,
    "create" boolean DEFAULT false,
    update boolean DEFAULT false,
    delete boolean DEFAULT false,
    export boolean DEFAULT false,
    print boolean DEFAULT false,
    actions jsonb,
    "when" jsonb,
    field_rules jsonb,
    retired boolean DEFAULT false,
    CONSTRAINT entity_or_page_target CHECK ((num_nonnulls(entity_id, page_id) = 1))
);


--
-- Name: permission_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.permission ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.permission_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: role; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.role (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    label text NOT NULL,
    description text,
    enabled boolean DEFAULT true
);


--
-- Name: role_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.role ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: schedule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schedule (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    app_id bigint NOT NULL,
    key text NOT NULL,
    source text DEFAULT 'file'::text NOT NULL,
    label text NOT NULL,
    description text,
    cron text NOT NULL,
    timezone text NOT NULL,
    job_id bigint NOT NULL,
    job_app_name text NOT NULL,
    job_name text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    retired boolean DEFAULT false NOT NULL,
    next_run_at timestamp with time zone,
    last_run_at timestamp with time zone,
    last_error text,
    actor_id bigint,
    CONSTRAINT schedule_source_check CHECK ((source = ANY (ARRAY['file'::text, 'studio'::text, 'system'::text])))
);


--
-- Name: schedule_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.schedule ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.schedule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: session; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.session (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    user_id bigint NOT NULL,
    token_digest text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    started_at timestamp with time zone NOT NULL,
    expires_at timestamp with time zone,
    last_seen_at timestamp with time zone,
    CONSTRAINT session_status_check CHECK ((status = ANY (ARRAY['active'::text, 'expired'::text, 'revoked'::text])))
);


--
-- Name: session_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.session ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.session_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: studio_preference; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.studio_preference (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    user_id bigint NOT NULL,
    key text NOT NULL,
    value jsonb
);


--
-- Name: studio_preference_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.studio_preference ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.studio_preference_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: studio_saved_filter; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.studio_saved_filter (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    user_id bigint NOT NULL,
    entity text NOT NULL,
    label text NOT NULL,
    filters jsonb NOT NULL
);


--
-- Name: studio_saved_filter_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.studio_saved_filter ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.studio_saved_filter_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."user" (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    email text NOT NULL,
    full_name text NOT NULL,
    password_hash text,
    enabled boolean DEFAULT true,
    administrator boolean DEFAULT false
);


--
-- Name: user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public."user" ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_role; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_role (
    id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id bigint,
    user_id bigint NOT NULL,
    role_id bigint NOT NULL
);


--
-- Name: user_role_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.user_role ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.user_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: activity activity_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity
    ADD CONSTRAINT activity_name_key UNIQUE (name);


--
-- Name: activity activity_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity
    ADD CONSTRAINT activity_pkey PRIMARY KEY (id);


--
-- Name: app app_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.app
    ADD CONSTRAINT app_name_key UNIQUE (name);


--
-- Name: app app_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.app
    ADD CONSTRAINT app_pkey PRIMARY KEY (id);


--
-- Name: configuration configuration_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.configuration
    ADD CONSTRAINT configuration_name_key UNIQUE (name);


--
-- Name: configuration configuration_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.configuration
    ADD CONSTRAINT configuration_pkey PRIMARY KEY (id);


--
-- Name: constraint constraint_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."constraint"
    ADD CONSTRAINT constraint_name_key UNIQUE (name);


--
-- Name: constraint constraint_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."constraint"
    ADD CONSTRAINT constraint_pkey PRIMARY KEY (id);


--
-- Name: country country_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.country
    ADD CONSTRAINT country_code_key UNIQUE (code);


--
-- Name: country country_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.country
    ADD CONSTRAINT country_name_key UNIQUE (name);


--
-- Name: country country_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.country
    ADD CONSTRAINT country_pkey PRIMARY KEY (id);


--
-- Name: currency currency_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currency
    ADD CONSTRAINT currency_code_key UNIQUE (code);


--
-- Name: currency currency_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currency
    ADD CONSTRAINT currency_name_key UNIQUE (name);


--
-- Name: currency currency_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currency
    ADD CONSTRAINT currency_pkey PRIMARY KEY (id);


--
-- Name: entity entity_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entity
    ADD CONSTRAINT entity_name_key UNIQUE (name);


--
-- Name: entity entity_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entity
    ADD CONSTRAINT entity_pkey PRIMARY KEY (id);


--
-- Name: entity entity_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entity
    ADD CONSTRAINT entity_slug_key UNIQUE (slug);


--
-- Name: field field_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.field
    ADD CONSTRAINT field_name_key UNIQUE (name);


--
-- Name: field field_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.field
    ADD CONSTRAINT field_pkey PRIMARY KEY (id);


--
-- Name: file file_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file
    ADD CONSTRAINT file_name_key UNIQUE (name);


--
-- Name: file file_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file
    ADD CONSTRAINT file_pkey PRIMARY KEY (id);


--
-- Name: file file_storage_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file
    ADD CONSTRAINT file_storage_key_key UNIQUE (storage_key);


--
-- Name: import import_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import
    ADD CONSTRAINT import_name_key UNIQUE (name);


--
-- Name: import import_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import
    ADD CONSTRAINT import_pkey PRIMARY KEY (id);


--
-- Name: import_row import_row_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_row
    ADD CONSTRAINT import_row_name_key UNIQUE (name);


--
-- Name: import_row import_row_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_row
    ADD CONSTRAINT import_row_pkey PRIMARY KEY (id);


--
-- Name: index index_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.index
    ADD CONSTRAINT index_name_key UNIQUE (name);


--
-- Name: index index_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.index
    ADD CONSTRAINT index_pkey PRIMARY KEY (id);


--
-- Name: job job_app_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job
    ADD CONSTRAINT job_app_key_key UNIQUE (app_id, key);


--
-- Name: job_execution job_execution_job_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job_execution
    ADD CONSTRAINT job_execution_job_idempotency_key_key UNIQUE (job_id, idempotency_key);


--
-- Name: job_execution job_execution_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job_execution
    ADD CONSTRAINT job_execution_name_key UNIQUE (name);


--
-- Name: job_execution job_execution_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job_execution
    ADD CONSTRAINT job_execution_pkey PRIMARY KEY (id);


--
-- Name: job job_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job
    ADD CONSTRAINT job_name_key UNIQUE (name);


--
-- Name: job job_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job
    ADD CONSTRAINT job_pkey PRIMARY KEY (id);


--
-- Name: language language_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.language
    ADD CONSTRAINT language_code_key UNIQUE (code);


--
-- Name: language language_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.language
    ADD CONSTRAINT language_name_key UNIQUE (name);


--
-- Name: language language_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.language
    ADD CONSTRAINT language_pkey PRIMARY KEY (id);


--
-- Name: log log_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.log
    ADD CONSTRAINT log_name_key UNIQUE (name);


--
-- Name: log log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.log
    ADD CONSTRAINT log_pkey PRIMARY KEY (id);


--
-- Name: naming_series naming_series_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.naming_series
    ADD CONSTRAINT naming_series_key_key UNIQUE (key);


--
-- Name: naming_series naming_series_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.naming_series
    ADD CONSTRAINT naming_series_name_key UNIQUE (name);


--
-- Name: naming_series naming_series_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.naming_series
    ADD CONSTRAINT naming_series_pkey PRIMARY KEY (id);


--
-- Name: notification notification_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification
    ADD CONSTRAINT notification_name_key UNIQUE (name);


--
-- Name: notification notification_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification
    ADD CONSTRAINT notification_pkey PRIMARY KEY (id);


--
-- Name: notification notification_recipient_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification
    ADD CONSTRAINT notification_recipient_idempotency_key_key UNIQUE (recipient_id, idempotency_key);


--
-- Name: page page_app_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.page
    ADD CONSTRAINT page_app_key_key UNIQUE (app_id, key);


--
-- Name: page page_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.page
    ADD CONSTRAINT page_name_key UNIQUE (name);


--
-- Name: page page_path_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.page
    ADD CONSTRAINT page_path_key UNIQUE (path);


--
-- Name: page page_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.page
    ADD CONSTRAINT page_pkey PRIMARY KEY (id);


--
-- Name: patch_run patch_run_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.patch_run
    ADD CONSTRAINT patch_run_name_key UNIQUE (name);


--
-- Name: patch_run patch_run_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.patch_run
    ADD CONSTRAINT patch_run_pkey PRIMARY KEY (id);


--
-- Name: permission permission_entity_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission
    ADD CONSTRAINT permission_entity_role_key UNIQUE (entity_id, role_id);


--
-- Name: permission permission_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission
    ADD CONSTRAINT permission_name_key UNIQUE (name);


--
-- Name: permission permission_page_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission
    ADD CONSTRAINT permission_page_role_key UNIQUE (page_id, role_id);


--
-- Name: permission permission_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission
    ADD CONSTRAINT permission_pkey PRIMARY KEY (id);


--
-- Name: studio_preference preference_user_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.studio_preference
    ADD CONSTRAINT preference_user_key_key UNIQUE (user_id, key);


--
-- Name: role role_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role
    ADD CONSTRAINT role_name_key UNIQUE (name);


--
-- Name: role role_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role
    ADD CONSTRAINT role_pkey PRIMARY KEY (id);


--
-- Name: studio_saved_filter saved_filter_user_entity_label_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.studio_saved_filter
    ADD CONSTRAINT saved_filter_user_entity_label_key UNIQUE (user_id, entity, label);


--
-- Name: schedule schedule_app_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schedule
    ADD CONSTRAINT schedule_app_key_key UNIQUE (app_id, key);


--
-- Name: schedule schedule_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schedule
    ADD CONSTRAINT schedule_name_key UNIQUE (name);


--
-- Name: schedule schedule_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schedule
    ADD CONSTRAINT schedule_pkey PRIMARY KEY (id);


--
-- Name: session session_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.session
    ADD CONSTRAINT session_name_key UNIQUE (name);


--
-- Name: session session_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.session
    ADD CONSTRAINT session_pkey PRIMARY KEY (id);


--
-- Name: session session_token_digest_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.session
    ADD CONSTRAINT session_token_digest_key UNIQUE (token_digest);


--
-- Name: studio_preference studio_preference_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.studio_preference
    ADD CONSTRAINT studio_preference_name_key UNIQUE (name);


--
-- Name: studio_preference studio_preference_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.studio_preference
    ADD CONSTRAINT studio_preference_pkey PRIMARY KEY (id);


--
-- Name: studio_saved_filter studio_saved_filter_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.studio_saved_filter
    ADD CONSTRAINT studio_saved_filter_name_key UNIQUE (name);


--
-- Name: studio_saved_filter studio_saved_filter_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.studio_saved_filter
    ADD CONSTRAINT studio_saved_filter_pkey PRIMARY KEY (id);


--
-- Name: user user_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT user_email_key UNIQUE (email);


--
-- Name: user user_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT user_name_key UNIQUE (name);


--
-- Name: user user_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT user_pkey PRIMARY KEY (id);


--
-- Name: user_role user_role_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_role
    ADD CONSTRAINT user_role_name_key UNIQUE (name);


--
-- Name: user_role user_role_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_role
    ADD CONSTRAINT user_role_pkey PRIMARY KEY (id);


--
-- Name: by_import; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX by_import ON public.import_row USING btree (import_id, row_number);


--
-- Name: by_recipient_read_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX by_recipient_read_at ON public.notification USING btree (recipient_id, read_at);


--
-- Name: by_record; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX by_record ON public.activity USING btree (entity_id, record_id);


--
-- Name: by_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX by_target ON public.file USING btree (app, entity, record_id, field);


--
-- Name: constraint_constraint_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX constraint_constraint_name_idx ON public."constraint" USING btree (constraint_name);


--
-- Name: constraint_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX constraint_entity_id_idx ON public."constraint" USING btree (entity_id);


--
-- Name: constraint_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX constraint_type_idx ON public."constraint" USING btree (type);


--
-- Name: currency_enabled_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX currency_enabled_idx ON public.currency USING btree (enabled);


--
-- Name: entity_app_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX entity_app_id_idx ON public.entity USING btree (app_id);


--
-- Name: entity_is_collection_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX entity_is_collection_idx ON public.entity USING btree (is_collection);


--
-- Name: entity_is_private_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX entity_is_private_idx ON public.entity USING btree (is_private);


--
-- Name: entity_is_single_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX entity_is_single_idx ON public.entity USING btree (is_single);


--
-- Name: entity_is_system_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX entity_is_system_idx ON public.entity USING btree (is_system);


--
-- Name: entity_key_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX entity_key_idx ON public.entity USING btree (key);


--
-- Name: entity_slug_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX entity_slug_idx ON public.entity USING btree (slug);


--
-- Name: field_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX field_entity_id_idx ON public.field USING btree (entity_id);


--
-- Name: field_field_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX field_field_name_idx ON public.field USING btree (field_name);


--
-- Name: field_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX field_type_idx ON public.field USING btree (type);


--
-- Name: import_row_import_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX import_row_import_id_idx ON public.import_row USING btree (import_id);


--
-- Name: index_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_entity_id_idx ON public.index USING btree (entity_id);


--
-- Name: index_index_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_index_name_idx ON public.index USING btree (index_name);


--
-- Name: job_app_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_app_id_idx ON public.job USING btree (app_id);


--
-- Name: job_enabled_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_enabled_idx ON public.job USING btree (enabled);


--
-- Name: job_execution_actor_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_actor_id_idx ON public.job_execution USING btree (actor_id);


--
-- Name: job_execution_app_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_app_name_idx ON public.job_execution USING btree (app_name);


--
-- Name: job_execution_claim; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_claim ON public.job_execution USING btree (status, queue, run_after, priority);


--
-- Name: job_execution_finished_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_finished_at_idx ON public.job_execution USING btree (finished_at);


--
-- Name: job_execution_job_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_job_id_idx ON public.job_execution USING btree (job_id);


--
-- Name: job_execution_job_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_job_name_idx ON public.job_execution USING btree (job_name);


--
-- Name: job_execution_locked_until_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_locked_until_idx ON public.job_execution USING btree (locked_until);


--
-- Name: job_execution_priority_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_priority_idx ON public.job_execution USING btree (priority);


--
-- Name: job_execution_queue_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_queue_idx ON public.job_execution USING btree (queue);


--
-- Name: job_execution_recovery; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_recovery ON public.job_execution USING btree (status, locked_until);


--
-- Name: job_execution_run_after_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_run_after_idx ON public.job_execution USING btree (run_after);


--
-- Name: job_execution_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_execution_status_idx ON public.job_execution USING btree (status);


--
-- Name: job_key_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_key_idx ON public.job USING btree (key);


--
-- Name: job_queue_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_queue_idx ON public.job USING btree (queue);


--
-- Name: job_retired_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_retired_idx ON public.job USING btree (retired);


--
-- Name: job_source_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX job_source_idx ON public.job USING btree (source);


--
-- Name: language_enabled_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX language_enabled_idx ON public.language USING btree (enabled);


--
-- Name: naming_series_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX naming_series_entity_id_idx ON public.naming_series USING btree (entity_id);


--
-- Name: notification_read_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notification_read_at_idx ON public.notification USING btree (read_at);


--
-- Name: notification_recipient_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notification_recipient_id_idx ON public.notification USING btree (recipient_id);


--
-- Name: page_app_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX page_app_id_idx ON public.page USING btree (app_id);


--
-- Name: page_key_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX page_key_idx ON public.page USING btree (key);


--
-- Name: page_path_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX page_path_idx ON public.page USING btree (path);


--
-- Name: page_renderer_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX page_renderer_idx ON public.page USING btree (renderer);


--
-- Name: page_retired_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX page_retired_idx ON public.page USING btree (retired);


--
-- Name: page_source_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX page_source_idx ON public.page USING btree (source);


--
-- Name: patch_run_app_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX patch_run_app_id_idx ON public.patch_run USING btree (app_id);


--
-- Name: patch_run_patch_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX patch_run_patch_id_idx ON public.patch_run USING btree (patch_id);


--
-- Name: patch_run_phase_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX patch_run_phase_idx ON public.patch_run USING btree (phase);


--
-- Name: permission_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX permission_entity_id_idx ON public.permission USING btree (entity_id);


--
-- Name: permission_page_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX permission_page_id_idx ON public.permission USING btree (page_id);


--
-- Name: permission_retired_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX permission_retired_idx ON public.permission USING btree (retired);


--
-- Name: permission_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX permission_role_id_idx ON public.permission USING btree (role_id);


--
-- Name: role_enabled_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX role_enabled_idx ON public.role USING btree (enabled);


--
-- Name: schedule_actor_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_actor_id_idx ON public.schedule USING btree (actor_id);


--
-- Name: schedule_app_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_app_id_idx ON public.schedule USING btree (app_id);


--
-- Name: schedule_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_due ON public.schedule USING btree (enabled, retired, next_run_at);


--
-- Name: schedule_enabled_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_enabled_idx ON public.schedule USING btree (enabled);


--
-- Name: schedule_job_app_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_job_app_name_idx ON public.schedule USING btree (job_app_name);


--
-- Name: schedule_job_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_job_id_idx ON public.schedule USING btree (job_id);


--
-- Name: schedule_job_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_job_name_idx ON public.schedule USING btree (job_name);


--
-- Name: schedule_key_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_key_idx ON public.schedule USING btree (key);


--
-- Name: schedule_next_run_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_next_run_at_idx ON public.schedule USING btree (next_run_at);


--
-- Name: schedule_retired_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_retired_idx ON public.schedule USING btree (retired);


--
-- Name: schedule_source_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX schedule_source_idx ON public.schedule USING btree (source);


--
-- Name: session_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX session_status_idx ON public.session USING btree (status);


--
-- Name: session_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX session_user_id_idx ON public.session USING btree (user_id);


--
-- Name: user_administrator_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_administrator_idx ON public."user" USING btree (administrator);


--
-- Name: user_enabled_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_enabled_idx ON public."user" USING btree (enabled);


--
-- Name: user_role_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_role_role_id_idx ON public.user_role USING btree (role_id);


--
-- Name: user_role_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_role_user_id_idx ON public.user_role USING btree (user_id);


--
-- Name: configuration configuration_country_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.configuration
    ADD CONSTRAINT configuration_country_id_fkey FOREIGN KEY (country_id) REFERENCES public.country(id);


--
-- Name: configuration configuration_currency_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.configuration
    ADD CONSTRAINT configuration_currency_id_fkey FOREIGN KEY (currency_id) REFERENCES public.currency(id);


--
-- Name: configuration configuration_language_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.configuration
    ADD CONSTRAINT configuration_language_id_fkey FOREIGN KEY (language_id) REFERENCES public.language(id);


--
-- Name: constraint constraint_entity_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."constraint"
    ADD CONSTRAINT constraint_entity_id_fkey FOREIGN KEY (entity_id) REFERENCES public.entity(id);


--
-- Name: entity entity_app_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entity
    ADD CONSTRAINT entity_app_id_fkey FOREIGN KEY (app_id) REFERENCES public.app(id);


--
-- Name: field field_entity_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.field
    ADD CONSTRAINT field_entity_id_fkey FOREIGN KEY (entity_id) REFERENCES public.entity(id);


--
-- Name: index index_entity_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.index
    ADD CONSTRAINT index_entity_id_fkey FOREIGN KEY (entity_id) REFERENCES public.entity(id);


--
-- Name: job job_app_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job
    ADD CONSTRAINT job_app_id_fkey FOREIGN KEY (app_id) REFERENCES public.app(id);


--
-- Name: job_execution job_execution_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job_execution
    ADD CONSTRAINT job_execution_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.job(id);


--
-- Name: naming_series naming_series_entity_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.naming_series
    ADD CONSTRAINT naming_series_entity_id_fkey FOREIGN KEY (entity_id) REFERENCES public.entity(id);


--
-- Name: notification notification_recipient_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification
    ADD CONSTRAINT notification_recipient_id_fkey FOREIGN KEY (recipient_id) REFERENCES public."user"(id);


--
-- Name: page page_app_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.page
    ADD CONSTRAINT page_app_id_fkey FOREIGN KEY (app_id) REFERENCES public.app(id);


--
-- Name: patch_run patch_run_app_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.patch_run
    ADD CONSTRAINT patch_run_app_id_fkey FOREIGN KEY (app_id) REFERENCES public.app(id);


--
-- Name: permission permission_entity_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission
    ADD CONSTRAINT permission_entity_id_fkey FOREIGN KEY (entity_id) REFERENCES public.entity(id);


--
-- Name: permission permission_page_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission
    ADD CONSTRAINT permission_page_id_fkey FOREIGN KEY (page_id) REFERENCES public.page(id);


--
-- Name: permission permission_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission
    ADD CONSTRAINT permission_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.role(id);


--
-- Name: schedule schedule_app_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schedule
    ADD CONSTRAINT schedule_app_id_fkey FOREIGN KEY (app_id) REFERENCES public.app(id);


--
-- Name: schedule schedule_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schedule
    ADD CONSTRAINT schedule_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.job(id);


--
-- Name: session session_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.session
    ADD CONSTRAINT session_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id);


--
-- Name: studio_preference studio_preference_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.studio_preference
    ADD CONSTRAINT studio_preference_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id);


--
-- Name: studio_saved_filter studio_saved_filter_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.studio_saved_filter
    ADD CONSTRAINT studio_saved_filter_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id);


--
-- Name: user_role user_role_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_role
    ADD CONSTRAINT user_role_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.role(id);


--
-- Name: user_role user_role_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_role
    ADD CONSTRAINT user_role_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id);


--
-- PostgreSQL database dump complete
--

\unrestrict dygoschemasnapshot
