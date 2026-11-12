-- 0071 Guard jobs at a base (docs/base-building-plan.md, Defence: "Wardens+ can post guard jobs on the base on the JOBS
-- board, so freelancers can be hired to defend it").
--
-- A guard job guards a node (node_id) or one of the poster's faction's standing bases (base_id), never both. The guard
-- spot is the base's core; the job's radius_m is the base's zone or JobGuardRadiusM, whichever is larger. A base that
-- collapses or is destroyed ends its open guard jobs at once, and the unpaid slots go back to the poster.
ALTER TABLE td.jobs ADD COLUMN base_id bigint REFERENCES td.bases (id) ON DELETE SET NULL;
ALTER TABLE td.jobs ADD CONSTRAINT jobs_guard_one_target
    CHECK (base_id IS NULL OR (kind = 'guard' AND node_id IS NULL));
CREATE INDEX jobs_base_open_idx ON td.jobs (base_id) WHERE status = 'open' AND base_id IS NOT NULL;
