--
-- PostgreSQL database dump
--


-- Dumped from database version 16.11 (Debian 16.11-1.pgdg13+1)
-- Dumped by pg_dump version 16.11 (Debian 16.11-1.pgdg12+1)


--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- Name: analysis_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.analysis_status AS ENUM (
    'pending',
    'running',
    'completed',
    'failed'
);


--
-- Name: github_account_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.github_account_type AS ENUM (
    'organization',
    'user'
);


--
-- Name: oauth_provider; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.oauth_provider AS ENUM (
    'github'
);


--
-- Name: plan_tier; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.plan_tier AS ENUM (
    'free',
    'pro',
    'pro_plus',
    'enterprise'
);


--
-- Name: river_job_state; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.river_job_state AS ENUM (
    'available',
    'cancelled',
    'completed',
    'discarded',
    'pending',
    'retryable',
    'running',
    'scheduled'
);


--
-- Name: subscription_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.subscription_status AS ENUM (
    'active',
    'canceled',
    'expired'
);


--
-- Name: test_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.test_status AS ENUM (
    'active',
    'skipped',
    'todo',
    'focused',
    'xfail'
);


--
-- Name: usage_event_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.usage_event_type AS ENUM (
    'specview',
    'analysis'
);


--
-- Name: river_job_state_in_bitmask(bit, public.river_job_state); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.river_job_state_in_bitmask(bitmask bit, state public.river_job_state) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT CASE state
        WHEN 'available' THEN get_bit(bitmask, 7)
        WHEN 'cancelled' THEN get_bit(bitmask, 6)
        WHEN 'completed' THEN get_bit(bitmask, 5)
        WHEN 'discarded' THEN get_bit(bitmask, 4)
        WHEN 'pending'   THEN get_bit(bitmask, 3)
        WHEN 'retryable' THEN get_bit(bitmask, 2)
        WHEN 'running'   THEN get_bit(bitmask, 1)
        WHEN 'scheduled' THEN get_bit(bitmask, 0)
        ELSE 0
    END = 1;
$$;




--
-- Name: analyses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.analyses (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    codebase_id uuid NOT NULL,
    commit_sha character varying(40) NOT NULL,
    branch_name character varying(255),
    status public.analysis_status DEFAULT 'pending'::public.analysis_status NOT NULL,
    error_message text,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    total_suites integer DEFAULT 0 NOT NULL,
    total_tests integer DEFAULT 0 NOT NULL,
    committed_at timestamp with time zone,
    parser_version character varying(100) DEFAULT 'legacy'::character varying NOT NULL
);


--
-- Name: atlas_schema_revisions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.atlas_schema_revisions (
    version character varying NOT NULL,
    description character varying NOT NULL,
    type bigint DEFAULT 2 NOT NULL,
    applied bigint DEFAULT 0 NOT NULL,
    total bigint DEFAULT 0 NOT NULL,
    executed_at timestamp with time zone NOT NULL,
    execution_time bigint NOT NULL,
    error text,
    error_stmt text,
    hash character varying NOT NULL,
    partial_hashes jsonb,
    operator_version character varying NOT NULL
);


--
-- Name: codebases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.codebases (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    host character varying(255) DEFAULT 'github.com'::character varying NOT NULL,
    owner character varying(255) NOT NULL,
    name character varying(255) NOT NULL,
    default_branch character varying(100),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    last_viewed_at timestamp with time zone,
    external_repo_id character varying(64) NOT NULL,
    is_stale boolean DEFAULT false NOT NULL,
    is_private boolean DEFAULT false NOT NULL
);


--
-- Name: github_app_installations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.github_app_installations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    installation_id bigint NOT NULL,
    account_type public.github_account_type NOT NULL,
    account_id bigint NOT NULL,
    account_login character varying(255) NOT NULL,
    account_avatar_url text,
    installer_user_id uuid,
    suspended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: github_organizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.github_organizations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    github_org_id bigint NOT NULL,
    login character varying(255) NOT NULL,
    avatar_url text,
    html_url text,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: oauth_accounts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.oauth_accounts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    provider public.oauth_provider NOT NULL,
    provider_user_id character varying(255) NOT NULL,
    provider_username character varying(255),
    access_token text,
    scope character varying(500),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: refresh_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.refresh_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    token_hash text NOT NULL,
    family_id uuid NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    replaces uuid
);


--
-- Name: river_client; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.river_client (
    id text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    paused_at timestamp with time zone,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT name_length CHECK (((char_length(id) > 0) AND (char_length(id) < 128)))
);


--
-- Name: river_client_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.river_client_queue (
    river_client_id text NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    max_workers bigint DEFAULT 0 NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    num_jobs_completed bigint DEFAULT 0 NOT NULL,
    num_jobs_running bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT name_length CHECK (((char_length(name) > 0) AND (char_length(name) < 128))),
    CONSTRAINT num_jobs_completed_zero_or_positive CHECK ((num_jobs_completed >= 0)),
    CONSTRAINT num_jobs_running_zero_or_positive CHECK ((num_jobs_running >= 0))
);


--
-- Name: river_job; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_job (
    id bigint NOT NULL,
    state public.river_job_state DEFAULT 'available'::public.river_job_state NOT NULL,
    attempt smallint DEFAULT 0 NOT NULL,
    max_attempts smallint NOT NULL,
    attempted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    finalized_at timestamp with time zone,
    scheduled_at timestamp with time zone DEFAULT now() NOT NULL,
    priority smallint DEFAULT 1 NOT NULL,
    args jsonb NOT NULL,
    attempted_by text[],
    errors jsonb[],
    kind text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    queue text DEFAULT 'default'::text NOT NULL,
    tags character varying(255)[] DEFAULT '{}'::character varying[] NOT NULL,
    unique_key bytea,
    unique_states bit(8),
    CONSTRAINT finalized_or_finalized_at_null CHECK ((((finalized_at IS NULL) AND (state <> ALL (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))) OR ((finalized_at IS NOT NULL) AND (state = ANY (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))))),
    CONSTRAINT kind_length CHECK (((char_length(kind) > 0) AND (char_length(kind) < 128))),
    CONSTRAINT max_attempts_is_positive CHECK ((max_attempts > 0)),
    CONSTRAINT priority_in_range CHECK (((priority >= 1) AND (priority <= 4))),
    CONSTRAINT queue_length CHECK (((char_length(queue) > 0) AND (char_length(queue) < 128)))
);


--
-- Name: river_job_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.river_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: river_job_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.river_job_id_seq OWNED BY public.river_job.id;


--
-- Name: river_leader; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.river_leader (
    elected_at timestamp with time zone NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    leader_id text NOT NULL,
    name text DEFAULT 'default'::text NOT NULL,
    CONSTRAINT leader_id_length CHECK (((char_length(leader_id) > 0) AND (char_length(leader_id) < 128))),
    CONSTRAINT name_length CHECK ((name = 'default'::text))
);


--
-- Name: river_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_queue (
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    paused_at timestamp with time zone,
    updated_at timestamp with time zone NOT NULL
);


--
-- Name: spec_behaviors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.spec_behaviors (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    feature_id uuid NOT NULL,
    source_test_case_id uuid,
    original_name character varying(2000) NOT NULL,
    converted_description text NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: spec_documents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.spec_documents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    analysis_id uuid NOT NULL,
    content_hash bytea NOT NULL,
    language character varying(10) DEFAULT 'en'::character varying NOT NULL,
    executive_summary text,
    model_id character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    user_id uuid NOT NULL
);


--
-- Name: spec_domains; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.spec_domains (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    document_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    sort_order integer DEFAULT 0 NOT NULL,
    classification_confidence numeric(3,2),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: spec_features; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.spec_features (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    domain_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: subscription_plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.subscription_plans (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tier public.plan_tier NOT NULL,
    specview_monthly_limit integer,
    analysis_monthly_limit integer,
    retention_days integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    monthly_price integer
);


--
-- Name: system_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_config (
    key character varying(100) NOT NULL,
    value text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: test_cases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_cases (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    suite_id uuid NOT NULL,
    name character varying(2000) NOT NULL,
    line_number integer,
    status public.test_status DEFAULT 'active'::public.test_status NOT NULL,
    tags jsonb DEFAULT '[]'::jsonb NOT NULL,
    modifier character varying(50)
);


--
-- Name: test_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_files (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    analysis_id uuid NOT NULL,
    file_path character varying(1000) NOT NULL,
    framework character varying(50),
    domain_hints jsonb
);


--
-- Name: test_suites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_suites (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    parent_id uuid,
    name character varying(500) NOT NULL,
    line_number integer,
    depth integer DEFAULT 0 NOT NULL,
    file_id uuid NOT NULL,
    CONSTRAINT chk_no_self_reference CHECK ((id <> parent_id))
);


--
-- Name: usage_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.usage_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    event_type public.usage_event_type NOT NULL,
    analysis_id uuid,
    document_id uuid,
    quota_amount integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_usage_events_resource CHECK (((((analysis_id IS NOT NULL))::integer + ((document_id IS NOT NULL))::integer) = 1))
);


--
-- Name: user_analysis_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_analysis_history (
    user_id uuid NOT NULL,
    analysis_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    id uuid DEFAULT gen_random_uuid() NOT NULL
);


--
-- Name: user_bookmarks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_bookmarks (
    user_id uuid NOT NULL,
    codebase_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    id uuid DEFAULT gen_random_uuid() NOT NULL
);


--
-- Name: user_github_org_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_github_org_memberships (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    org_id uuid NOT NULL,
    role character varying(50),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_github_repositories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_github_repositories (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    github_repo_id bigint NOT NULL,
    name character varying(255) NOT NULL,
    full_name character varying(500) NOT NULL,
    html_url text NOT NULL,
    description text,
    default_branch character varying(100),
    language character varying(50),
    visibility character varying(20) DEFAULT 'public'::character varying NOT NULL,
    is_private boolean DEFAULT false NOT NULL,
    archived boolean DEFAULT false NOT NULL,
    disabled boolean DEFAULT false NOT NULL,
    fork boolean DEFAULT false NOT NULL,
    stargazers_count integer DEFAULT 0 NOT NULL,
    pushed_at timestamp with time zone,
    source_type character varying(20) DEFAULT 'personal'::character varying NOT NULL,
    org_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_specview_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_specview_history (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    document_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_subscriptions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    status public.subscription_status DEFAULT 'active'::public.subscription_status NOT NULL,
    current_period_start timestamp with time zone NOT NULL,
    current_period_end timestamp with time zone NOT NULL,
    canceled_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_canceled_at_status CHECK (((status = 'canceled'::public.subscription_status) = (canceled_at IS NOT NULL)))
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email character varying(255),
    username character varying(255) NOT NULL,
    avatar_url text,
    last_login_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    token_version integer DEFAULT 1 NOT NULL
);


--
-- Name: river_job id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job ALTER COLUMN id SET DEFAULT nextval('public.river_job_id_seq'::regclass);


--
-- Name: analyses analyses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analyses
    ADD CONSTRAINT analyses_pkey PRIMARY KEY (id);


--
-- Name: atlas_schema_revisions atlas_schema_revisions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atlas_schema_revisions
    ADD CONSTRAINT atlas_schema_revisions_pkey PRIMARY KEY (version);


--
-- Name: codebases codebases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.codebases
    ADD CONSTRAINT codebases_pkey PRIMARY KEY (id);


--
-- Name: github_app_installations github_app_installations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_app_installations
    ADD CONSTRAINT github_app_installations_pkey PRIMARY KEY (id);


--
-- Name: github_organizations github_organizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_organizations
    ADD CONSTRAINT github_organizations_pkey PRIMARY KEY (id);


--
-- Name: oauth_accounts oauth_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_accounts
    ADD CONSTRAINT oauth_accounts_pkey PRIMARY KEY (id);


--
-- Name: refresh_tokens refresh_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_pkey PRIMARY KEY (id);


--
-- Name: river_client river_client_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_client
    ADD CONSTRAINT river_client_pkey PRIMARY KEY (id);


--
-- Name: river_client_queue river_client_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_client_queue
    ADD CONSTRAINT river_client_queue_pkey PRIMARY KEY (river_client_id, name);


--
-- Name: river_job river_job_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job
    ADD CONSTRAINT river_job_pkey PRIMARY KEY (id);


--
-- Name: river_leader river_leader_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_leader
    ADD CONSTRAINT river_leader_pkey PRIMARY KEY (name);


--
-- Name: river_queue river_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_queue
    ADD CONSTRAINT river_queue_pkey PRIMARY KEY (name);


--
-- Name: spec_behaviors spec_behaviors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_behaviors
    ADD CONSTRAINT spec_behaviors_pkey PRIMARY KEY (id);


--
-- Name: spec_documents spec_documents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_documents
    ADD CONSTRAINT spec_documents_pkey PRIMARY KEY (id);


--
-- Name: spec_domains spec_domains_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_domains
    ADD CONSTRAINT spec_domains_pkey PRIMARY KEY (id);


--
-- Name: spec_features spec_features_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_features
    ADD CONSTRAINT spec_features_pkey PRIMARY KEY (id);


--
-- Name: subscription_plans subscription_plans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_plans
    ADD CONSTRAINT subscription_plans_pkey PRIMARY KEY (id);


--
-- Name: system_config system_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_config
    ADD CONSTRAINT system_config_pkey PRIMARY KEY (key);


--
-- Name: test_cases test_cases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_cases
    ADD CONSTRAINT test_cases_pkey PRIMARY KEY (id);


--
-- Name: test_files test_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_files
    ADD CONSTRAINT test_files_pkey PRIMARY KEY (id);


--
-- Name: test_suites test_suites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_suites
    ADD CONSTRAINT test_suites_pkey PRIMARY KEY (id);


--
-- Name: github_app_installations uq_github_app_installations_account; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_app_installations
    ADD CONSTRAINT uq_github_app_installations_account UNIQUE (account_type, account_id);


--
-- Name: github_app_installations uq_github_app_installations_installation_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_app_installations
    ADD CONSTRAINT uq_github_app_installations_installation_id UNIQUE (installation_id);


--
-- Name: github_organizations uq_github_organizations_github_org_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_organizations
    ADD CONSTRAINT uq_github_organizations_github_org_id UNIQUE (github_org_id);


--
-- Name: oauth_accounts uq_oauth_provider_user; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_accounts
    ADD CONSTRAINT uq_oauth_provider_user UNIQUE (provider, provider_user_id);


--
-- Name: refresh_tokens uq_refresh_tokens_hash; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT uq_refresh_tokens_hash UNIQUE (token_hash);


--
-- Name: spec_documents uq_spec_documents_user_analysis_lang_version; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_documents
    ADD CONSTRAINT uq_spec_documents_user_analysis_lang_version UNIQUE (user_id, analysis_id, language, version);


--
-- Name: spec_documents uq_spec_documents_user_hash_lang_model_version; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_documents
    ADD CONSTRAINT uq_spec_documents_user_hash_lang_model_version UNIQUE (user_id, content_hash, language, model_id, version);


--
-- Name: subscription_plans uq_subscription_plans_tier; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_plans
    ADD CONSTRAINT uq_subscription_plans_tier UNIQUE (tier);


--
-- Name: test_files uq_test_files_analysis_path; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_files
    ADD CONSTRAINT uq_test_files_analysis_path UNIQUE (analysis_id, file_path);


--
-- Name: user_analysis_history uq_user_analysis_history_user_analysis; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_analysis_history
    ADD CONSTRAINT uq_user_analysis_history_user_analysis UNIQUE (user_id, analysis_id);


--
-- Name: user_bookmarks uq_user_bookmarks_user_codebase; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_bookmarks
    ADD CONSTRAINT uq_user_bookmarks_user_codebase UNIQUE (user_id, codebase_id);


--
-- Name: user_github_org_memberships uq_user_github_org_memberships_user_org; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_github_org_memberships
    ADD CONSTRAINT uq_user_github_org_memberships_user_org UNIQUE (user_id, org_id);


--
-- Name: user_github_repositories uq_user_github_repositories_user_repo; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_github_repositories
    ADD CONSTRAINT uq_user_github_repositories_user_repo UNIQUE (user_id, github_repo_id);


--
-- Name: user_specview_history uq_user_specview_history_user_document; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_specview_history
    ADD CONSTRAINT uq_user_specview_history_user_document UNIQUE (user_id, document_id);


--
-- Name: usage_events usage_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usage_events
    ADD CONSTRAINT usage_events_pkey PRIMARY KEY (id);


--
-- Name: user_analysis_history user_analysis_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_analysis_history
    ADD CONSTRAINT user_analysis_history_pkey PRIMARY KEY (id);


--
-- Name: user_bookmarks user_bookmarks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_bookmarks
    ADD CONSTRAINT user_bookmarks_pkey PRIMARY KEY (id);


--
-- Name: user_github_org_memberships user_github_org_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_github_org_memberships
    ADD CONSTRAINT user_github_org_memberships_pkey PRIMARY KEY (id);


--
-- Name: user_github_repositories user_github_repositories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_github_repositories
    ADD CONSTRAINT user_github_repositories_pkey PRIMARY KEY (id);


--
-- Name: user_specview_history user_specview_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_specview_history
    ADD CONSTRAINT user_specview_history_pkey PRIMARY KEY (id);


--
-- Name: user_subscriptions user_subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_subscriptions
    ADD CONSTRAINT user_subscriptions_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: idx_analyses_codebase_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analyses_codebase_status ON public.analyses USING btree (codebase_id, status);


--
-- Name: idx_analyses_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analyses_created ON public.analyses USING btree (codebase_id, created_at);


--
-- Name: idx_codebases_external_repo_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_codebases_external_repo_id ON public.codebases USING btree (host, external_repo_id);


--
-- Name: idx_codebases_identity; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_codebases_identity ON public.codebases USING btree (host, owner, name) WHERE (is_stale = false);


--
-- Name: idx_codebases_last_viewed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_codebases_last_viewed ON public.codebases USING btree (last_viewed_at) WHERE (last_viewed_at IS NOT NULL);


--
-- Name: idx_codebases_owner_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_codebases_owner_name ON public.codebases USING btree (owner, name);


--
-- Name: idx_codebases_public; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_codebases_public ON public.codebases USING btree (is_private) WHERE (is_private = false);


--
-- Name: idx_github_app_installations_installer; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_github_app_installations_installer ON public.github_app_installations USING btree (installer_user_id) WHERE (installer_user_id IS NOT NULL);


--
-- Name: idx_github_organizations_login; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_github_organizations_login ON public.github_organizations USING btree (login);


--
-- Name: idx_oauth_accounts_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_oauth_accounts_user_id ON public.oauth_accounts USING btree (user_id);


--
-- Name: idx_oauth_accounts_user_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_oauth_accounts_user_provider ON public.oauth_accounts USING btree (user_id, provider);


--
-- Name: idx_refresh_tokens_expires; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_expires ON public.refresh_tokens USING btree (expires_at) WHERE (revoked_at IS NULL);


--
-- Name: idx_refresh_tokens_family_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_family_active ON public.refresh_tokens USING btree (family_id, created_at) WHERE (revoked_at IS NULL);


--
-- Name: idx_refresh_tokens_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_user ON public.refresh_tokens USING btree (user_id);


--
-- Name: idx_spec_behaviors_feature_sort; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_spec_behaviors_feature_sort ON public.spec_behaviors USING btree (feature_id, sort_order);


--
-- Name: idx_spec_behaviors_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_spec_behaviors_source ON public.spec_behaviors USING btree (source_test_case_id) WHERE (source_test_case_id IS NOT NULL);


--
-- Name: idx_spec_documents_analysis; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_spec_documents_analysis ON public.spec_documents USING btree (analysis_id);


--
-- Name: idx_spec_documents_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_spec_documents_user_created ON public.spec_documents USING btree (user_id, created_at);


--
-- Name: idx_spec_domains_document_sort; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_spec_domains_document_sort ON public.spec_domains USING btree (document_id, sort_order);


--
-- Name: idx_spec_features_domain_sort; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_spec_features_domain_sort ON public.spec_features USING btree (domain_id, sort_order);


--
-- Name: idx_test_cases_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_cases_status ON public.test_cases USING btree (suite_id, status);


--
-- Name: idx_test_cases_suite; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_cases_suite ON public.test_cases USING btree (suite_id);


--
-- Name: idx_test_files_analysis; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_files_analysis ON public.test_files USING btree (analysis_id);


--
-- Name: idx_test_suites_file; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_suites_file ON public.test_suites USING btree (file_id);


--
-- Name: idx_test_suites_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_suites_parent ON public.test_suites USING btree (parent_id) WHERE (parent_id IS NOT NULL);


--
-- Name: idx_usage_events_analysis; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_usage_events_analysis ON public.usage_events USING btree (analysis_id) WHERE (analysis_id IS NOT NULL);


--
-- Name: idx_usage_events_document; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_usage_events_document ON public.usage_events USING btree (document_id) WHERE (document_id IS NOT NULL);


--
-- Name: idx_usage_events_quota_lookup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_usage_events_quota_lookup ON public.usage_events USING btree (user_id, event_type, created_at);


--
-- Name: idx_user_analysis_history_analysis; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_analysis_history_analysis ON public.user_analysis_history USING btree (analysis_id);


--
-- Name: idx_user_analysis_history_cursor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_analysis_history_cursor ON public.user_analysis_history USING btree (user_id, updated_at, id);


--
-- Name: idx_user_bookmarks_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_bookmarks_user ON public.user_bookmarks USING btree (user_id, created_at);


--
-- Name: idx_user_github_org_memberships_org; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_github_org_memberships_org ON public.user_github_org_memberships USING btree (org_id);


--
-- Name: idx_user_github_org_memberships_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_github_org_memberships_user ON public.user_github_org_memberships USING btree (user_id);


--
-- Name: idx_user_github_repositories_language; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_github_repositories_language ON public.user_github_repositories USING btree (user_id, language) WHERE (language IS NOT NULL);


--
-- Name: idx_user_github_repositories_org; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_github_repositories_org ON public.user_github_repositories USING btree (user_id, org_id) WHERE (org_id IS NOT NULL);


--
-- Name: idx_user_github_repositories_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_github_repositories_source ON public.user_github_repositories USING btree (user_id, source_type);


--
-- Name: idx_user_github_repositories_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_github_repositories_user ON public.user_github_repositories USING btree (user_id, updated_at);


--
-- Name: idx_user_specview_history_cursor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_specview_history_cursor ON public.user_specview_history USING btree (user_id, updated_at, id);


--
-- Name: idx_user_specview_history_document; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_specview_history_document ON public.user_specview_history USING btree (document_id);


--
-- Name: idx_user_subscriptions_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_subscriptions_active ON public.user_subscriptions USING btree (user_id) WHERE (status = 'active'::public.subscription_status);


--
-- Name: idx_user_subscriptions_period_end; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_subscriptions_period_end ON public.user_subscriptions USING btree (current_period_end) WHERE (status = 'active'::public.subscription_status);


--
-- Name: idx_user_subscriptions_plan; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_subscriptions_plan ON public.user_subscriptions USING btree (plan_id);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_users_email ON public.users USING btree (email) WHERE (email IS NOT NULL);


--
-- Name: idx_users_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_username ON public.users USING btree (username);


--
-- Name: river_job_args_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_args_index ON public.river_job USING gin (args);


--
-- Name: river_job_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_kind ON public.river_job USING btree (kind);


--
-- Name: river_job_metadata_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_metadata_index ON public.river_job USING gin (metadata);


--
-- Name: river_job_prioritized_fetching_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_prioritized_fetching_index ON public.river_job USING btree (state, queue, priority, scheduled_at, id);


--
-- Name: river_job_state_and_finalized_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_state_and_finalized_at_index ON public.river_job USING btree (state, finalized_at) WHERE (finalized_at IS NOT NULL);


--
-- Name: river_job_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX river_job_unique_idx ON public.river_job USING btree (unique_key) WHERE ((unique_key IS NOT NULL) AND (unique_states IS NOT NULL) AND public.river_job_state_in_bitmask(unique_states, state));


--
-- Name: uq_analyses_completed_commit_version; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_analyses_completed_commit_version ON public.analyses USING btree (codebase_id, commit_sha, parser_version) WHERE (status = 'completed'::public.analysis_status);


--
-- Name: analyses fk_analyses_codebase; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analyses
    ADD CONSTRAINT fk_analyses_codebase FOREIGN KEY (codebase_id) REFERENCES public.codebases(id) ON DELETE CASCADE;


--
-- Name: github_app_installations fk_github_app_installations_installer; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_app_installations
    ADD CONSTRAINT fk_github_app_installations_installer FOREIGN KEY (installer_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: oauth_accounts fk_oauth_accounts_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_accounts
    ADD CONSTRAINT fk_oauth_accounts_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: refresh_tokens fk_refresh_tokens_replaces; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT fk_refresh_tokens_replaces FOREIGN KEY (replaces) REFERENCES public.refresh_tokens(id) ON DELETE SET NULL;


--
-- Name: refresh_tokens fk_refresh_tokens_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: spec_behaviors fk_spec_behaviors_feature; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_behaviors
    ADD CONSTRAINT fk_spec_behaviors_feature FOREIGN KEY (feature_id) REFERENCES public.spec_features(id) ON DELETE CASCADE;


--
-- Name: spec_behaviors fk_spec_behaviors_test_case; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_behaviors
    ADD CONSTRAINT fk_spec_behaviors_test_case FOREIGN KEY (source_test_case_id) REFERENCES public.test_cases(id) ON DELETE SET NULL;


--
-- Name: spec_documents fk_spec_documents_analysis; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_documents
    ADD CONSTRAINT fk_spec_documents_analysis FOREIGN KEY (analysis_id) REFERENCES public.analyses(id) ON DELETE CASCADE;


--
-- Name: spec_documents fk_spec_documents_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_documents
    ADD CONSTRAINT fk_spec_documents_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: spec_domains fk_spec_domains_document; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_domains
    ADD CONSTRAINT fk_spec_domains_document FOREIGN KEY (document_id) REFERENCES public.spec_documents(id) ON DELETE CASCADE;


--
-- Name: spec_features fk_spec_features_domain; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spec_features
    ADD CONSTRAINT fk_spec_features_domain FOREIGN KEY (domain_id) REFERENCES public.spec_domains(id) ON DELETE CASCADE;


--
-- Name: test_cases fk_test_cases_suite; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_cases
    ADD CONSTRAINT fk_test_cases_suite FOREIGN KEY (suite_id) REFERENCES public.test_suites(id) ON DELETE CASCADE;


--
-- Name: test_files fk_test_files_analysis; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_files
    ADD CONSTRAINT fk_test_files_analysis FOREIGN KEY (analysis_id) REFERENCES public.analyses(id) ON DELETE CASCADE;


--
-- Name: test_suites fk_test_suites_file; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_suites
    ADD CONSTRAINT fk_test_suites_file FOREIGN KEY (file_id) REFERENCES public.test_files(id) ON DELETE CASCADE;


--
-- Name: test_suites fk_test_suites_parent; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_suites
    ADD CONSTRAINT fk_test_suites_parent FOREIGN KEY (parent_id) REFERENCES public.test_suites(id) ON DELETE CASCADE;


--
-- Name: usage_events fk_usage_events_analysis; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usage_events
    ADD CONSTRAINT fk_usage_events_analysis FOREIGN KEY (analysis_id) REFERENCES public.analyses(id) ON DELETE SET NULL;


--
-- Name: usage_events fk_usage_events_document; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usage_events
    ADD CONSTRAINT fk_usage_events_document FOREIGN KEY (document_id) REFERENCES public.spec_documents(id) ON DELETE SET NULL;


--
-- Name: usage_events fk_usage_events_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usage_events
    ADD CONSTRAINT fk_usage_events_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_analysis_history fk_user_analysis_history_analysis; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_analysis_history
    ADD CONSTRAINT fk_user_analysis_history_analysis FOREIGN KEY (analysis_id) REFERENCES public.analyses(id) ON DELETE CASCADE;


--
-- Name: user_analysis_history fk_user_analysis_history_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_analysis_history
    ADD CONSTRAINT fk_user_analysis_history_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_bookmarks fk_user_bookmarks_codebase; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_bookmarks
    ADD CONSTRAINT fk_user_bookmarks_codebase FOREIGN KEY (codebase_id) REFERENCES public.codebases(id) ON DELETE CASCADE;


--
-- Name: user_bookmarks fk_user_bookmarks_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_bookmarks
    ADD CONSTRAINT fk_user_bookmarks_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_github_org_memberships fk_user_github_org_memberships_org; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_github_org_memberships
    ADD CONSTRAINT fk_user_github_org_memberships_org FOREIGN KEY (org_id) REFERENCES public.github_organizations(id) ON DELETE CASCADE;


--
-- Name: user_github_org_memberships fk_user_github_org_memberships_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_github_org_memberships
    ADD CONSTRAINT fk_user_github_org_memberships_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_github_repositories fk_user_github_repositories_org; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_github_repositories
    ADD CONSTRAINT fk_user_github_repositories_org FOREIGN KEY (org_id) REFERENCES public.github_organizations(id) ON DELETE CASCADE;


--
-- Name: user_github_repositories fk_user_github_repositories_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_github_repositories
    ADD CONSTRAINT fk_user_github_repositories_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_specview_history fk_user_specview_history_document; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_specview_history
    ADD CONSTRAINT fk_user_specview_history_document FOREIGN KEY (document_id) REFERENCES public.spec_documents(id) ON DELETE CASCADE;


--
-- Name: user_specview_history fk_user_specview_history_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_specview_history
    ADD CONSTRAINT fk_user_specview_history_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_subscriptions fk_user_subscriptions_plan; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_subscriptions
    ADD CONSTRAINT fk_user_subscriptions_plan FOREIGN KEY (plan_id) REFERENCES public.subscription_plans(id) ON DELETE RESTRICT;


--
-- Name: user_subscriptions fk_user_subscriptions_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_subscriptions
    ADD CONSTRAINT fk_user_subscriptions_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: river_client_queue river_client_queue_river_client_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_client_queue
    ADD CONSTRAINT river_client_queue_river_client_id_fkey FOREIGN KEY (river_client_id) REFERENCES public.river_client(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


