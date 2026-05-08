--
-- PostgreSQL database dump
--

\restrict dygoschemasnapshot

-- Dumped from database version 18.3 (Postgres.app)
-- Dumped by pg_dump version 18.3 (Postgres.app)

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
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
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
    CONSTRAINT activity_kind_check CHECK ((kind = ANY (ARRAY['record'::text, 'comment'::text, 'workflow'::text, 'job'::text, 'email'::text, 'attachment'::text, 'auth'::text, 'system'::text]))),
    CONSTRAINT activity_operation_check CHECK ((operation = ANY (ARRAY['create'::text, 'update'::text, 'delete'::text, 'restore'::text, 'comment'::text, 'workflow-transition'::text, 'job-completed'::text, 'email-sent'::text, 'attachment-added'::text, 'login'::text, 'logout'::text, 'system'::text]))),
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
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    name text NOT NULL,
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
-- Name: constraint; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."constraint" (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    entity_id bigint NOT NULL,
    name text NOT NULL,
    type text NOT NULL,
    fields jsonb,
    field text,
    operator text,
    value jsonb,
    "position" integer,
    CONSTRAINT constraint_operator_check CHECK ((operator = ANY (ARRAY['eq'::text, 'neq'::text, 'gt'::text, 'gte'::text, 'lt'::text, 'lte'::text, 'in'::text, 'not-in'::text]))),
    CONSTRAINT constraint_type_check CHECK ((type = ANY (ARRAY['unique'::text, 'check'::text])))
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
-- Name: entity; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.entity (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    app_id bigint NOT NULL,
    name text NOT NULL,
    label text NOT NULL,
    description text
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
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    entity_id bigint NOT NULL,
    name text NOT NULL,
    label text NOT NULL,
    type text NOT NULL,
    required boolean DEFAULT false,
    "unique" boolean DEFAULT false,
    index boolean DEFAULT false,
    "default" jsonb,
    "position" integer,
    options jsonb
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
-- Name: index; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.index (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    entity_id bigint NOT NULL,
    name text NOT NULL,
    fields jsonb NOT NULL,
    "position" integer
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
-- Name: permission; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permission (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    entity_id bigint NOT NULL,
    role_id bigint NOT NULL,
    read boolean DEFAULT false,
    "create" boolean DEFAULT false,
    update boolean DEFAULT false,
    delete boolean DEFAULT false,
    export boolean DEFAULT false,
    print boolean DEFAULT false
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
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    name text NOT NULL,
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
-- Name: session; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.session (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
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
-- Name: user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."user" (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
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
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
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
-- Name: constraint constraint_entity_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."constraint"
    ADD CONSTRAINT constraint_entity_name_key UNIQUE (entity_id, name);


--
-- Name: constraint constraint_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."constraint"
    ADD CONSTRAINT constraint_pkey PRIMARY KEY (id);


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
-- Name: field field_entity_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.field
    ADD CONSTRAINT field_entity_name_key UNIQUE (entity_id, name);


--
-- Name: field field_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.field
    ADD CONSTRAINT field_pkey PRIMARY KEY (id);


--
-- Name: index index_entity_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.index
    ADD CONSTRAINT index_entity_name_key UNIQUE (entity_id, name);


--
-- Name: index index_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.index
    ADD CONSTRAINT index_pkey PRIMARY KEY (id);


--
-- Name: permission permission_entity_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission
    ADD CONSTRAINT permission_entity_role_key UNIQUE (entity_id, role_id);


--
-- Name: permission permission_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission
    ADD CONSTRAINT permission_pkey PRIMARY KEY (id);


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
-- Name: user user_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT user_email_key UNIQUE (email);


--
-- Name: user user_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT user_pkey PRIMARY KEY (id);


--
-- Name: user_role user_role_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_role
    ADD CONSTRAINT user_role_pkey PRIMARY KEY (id);


--
-- Name: user_role user_role_user_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_role
    ADD CONSTRAINT user_role_user_role_key UNIQUE (user_id, role_id);


--
-- Name: activity_actor_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX activity_actor_id_idx ON public.activity USING btree (actor_id);


--
-- Name: activity_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX activity_entity_id_idx ON public.activity USING btree (entity_id);


--
-- Name: activity_kind_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX activity_kind_idx ON public.activity USING btree (kind);


--
-- Name: activity_operation_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX activity_operation_idx ON public.activity USING btree (operation);


--
-- Name: activity_record_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX activity_record_id_idx ON public.activity USING btree (record_id);


--
-- Name: activity_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX activity_status_idx ON public.activity USING btree (status);


--
-- Name: by_kind_operation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX by_kind_operation ON public.activity USING btree (kind, operation);


--
-- Name: by_record; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX by_record ON public.activity USING btree (entity_id, record_id);


--
-- Name: constraint_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX constraint_entity_id_idx ON public."constraint" USING btree (entity_id);


--
-- Name: constraint_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX constraint_name_idx ON public."constraint" USING btree (name);


--
-- Name: constraint_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX constraint_type_idx ON public."constraint" USING btree (type);


--
-- Name: entity_app_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX entity_app_id_idx ON public.entity USING btree (app_id);


--
-- Name: entity_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX entity_name_idx ON public.entity USING btree (name);


--
-- Name: field_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX field_entity_id_idx ON public.field USING btree (entity_id);


--
-- Name: field_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX field_name_idx ON public.field USING btree (name);


--
-- Name: field_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX field_type_idx ON public.field USING btree (type);


--
-- Name: index_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_entity_id_idx ON public.index USING btree (entity_id);


--
-- Name: index_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_name_idx ON public.index USING btree (name);


--
-- Name: permission_entity_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX permission_entity_id_idx ON public.permission USING btree (entity_id);


--
-- Name: permission_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX permission_role_id_idx ON public.permission USING btree (role_id);


--
-- Name: role_enabled_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX role_enabled_idx ON public.role USING btree (enabled);


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
-- PostgreSQL database dump complete
--

\unrestrict dygoschemasnapshot

