-- Anonymous daily player counts: one counter per (day, length, guesses).
-- guesses is 1..length+1 for a solve and 0 for a loss. No row identifies a
-- player. The cron in src/index.ts deletes rows older than 90 days.
CREATE TABLE daily_counts (
  day     TEXT    NOT NULL,
  length  INTEGER NOT NULL,
  guesses INTEGER NOT NULL,
  n       INTEGER NOT NULL,
  PRIMARY KEY (day, length, guesses)
);
