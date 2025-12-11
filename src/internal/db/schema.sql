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
    total_tests integer DEFAULT 0 NOT NULL
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
-- Name: test_suites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_suites (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    analysis_id uuid NOT NULL,
    parent_id uuid,
    name character varying(500) NOT NULL,
    file_path character varying(1000) NOT NULL,
    line_number integer,
    framework character varying(50),
    depth integer DEFAULT 0 NOT NULL,
    CONSTRAINT chk_no_self_reference CHECK ((id <> parent_id))
);


--
-- Name: analyses analyses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analyses
    ADD CONSTRAINT analyses_pkey PRIMARY KEY (id);


--
-- Name: codebases codebases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.codebases
    ADD CONSTRAINT codebases_pkey PRIMARY KEY (id);


--
-- Name: test_cases test_cases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_cases
    ADD CONSTRAINT test_cases_pkey PRIMARY KEY (id);


--
-- Name: test_suites test_suites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_suites
    ADD CONSTRAINT test_suites_pkey PRIMARY KEY (id);


--
-- Name: codebases uq_codebases_identity; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.codebases
    ADD CONSTRAINT uq_codebases_identity UNIQUE (host, owner, name);


--
-- Name: idx_analyses_codebase_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analyses_codebase_status ON public.analyses USING btree (codebase_id, status);


--
-- Name: idx_analyses_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analyses_created ON public.analyses USING btree (codebase_id, created_at);


--
-- Name: idx_codebases_owner_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_codebases_owner_name ON public.codebases USING btree (owner, name);


--
-- Name: idx_test_cases_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_cases_status ON public.test_cases USING btree (suite_id, status);


--
-- Name: idx_test_cases_suite; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_cases_suite ON public.test_cases USING btree (suite_id);


--
-- Name: idx_test_suites_analysis; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_suites_analysis ON public.test_suites USING btree (analysis_id);


--
-- Name: idx_test_suites_file; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_suites_file ON public.test_suites USING btree (analysis_id, file_path);


--
-- Name: idx_test_suites_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_suites_parent ON public.test_suites USING btree (parent_id) WHERE (parent_id IS NOT NULL);


--
-- Name: uq_analyses_completed_commit; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_analyses_completed_commit ON public.analyses USING btree (codebase_id, commit_sha) WHERE (status = 'completed'::public.analysis_status);


--
-- Name: analyses fk_analyses_codebase; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analyses
    ADD CONSTRAINT fk_analyses_codebase FOREIGN KEY (codebase_id) REFERENCES public.codebases(id) ON DELETE CASCADE;


--
-- Name: test_cases fk_test_cases_suite; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_cases
    ADD CONSTRAINT fk_test_cases_suite FOREIGN KEY (suite_id) REFERENCES public.test_suites(id) ON DELETE CASCADE;


--
-- Name: test_suites fk_test_suites_analysis; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_suites
    ADD CONSTRAINT fk_test_suites_analysis FOREIGN KEY (analysis_id) REFERENCES public.analyses(id) ON DELETE CASCADE;


--
-- Name: test_suites fk_test_suites_parent; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_suites
    ADD CONSTRAINT fk_test_suites_parent FOREIGN KEY (parent_id) REFERENCES public.test_suites(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


