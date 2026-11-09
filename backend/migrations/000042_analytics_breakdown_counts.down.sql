-- The rebuilt aggregates stay; only the helper goes (queries fall back to `||` in older code).
DROP FUNCTION IF EXISTS jsonb_sum_counts(jsonb, jsonb);
