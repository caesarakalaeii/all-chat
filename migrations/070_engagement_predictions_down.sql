-- 070_engagement_predictions_down.sql
DROP TABLE IF EXISTS prediction_entries;
-- Drop the circular FK before the tables so the DROPs don't depend on order.
ALTER TABLE IF EXISTS predictions DROP CONSTRAINT IF EXISTS fk_predictions_winning_outcome;
DROP TABLE IF EXISTS prediction_outcomes;
DROP TABLE IF EXISTS predictions;
