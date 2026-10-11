-- Async rooms ("any time"). Live rooms keep nothing in D1.
-- A row lives until expires_at (30 days after creation); the daily cron in
-- src/index.ts deletes expired rooms and their results.
CREATE TABLE async_rooms (
  code       TEXT    PRIMARY KEY,
  length     INTEGER NOT NULL,
  answer     TEXT    NOT NULL,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX async_rooms_expires_at ON async_rooms (expires_at);

-- One row per finished seat. key_hash is the SHA-256 of the seat key, so
-- the results endpoint can check a reader without storing the key itself.
-- rows is the JSON array of that seat's marks, colours only.
CREATE TABLE async_results (
  code        TEXT    NOT NULL,
  seat        INTEGER NOT NULL,
  key_hash    TEXT    NOT NULL,
  name        TEXT    NOT NULL,
  solved      INTEGER NOT NULL,
  guesses     INTEGER NOT NULL,
  elapsed_ms  INTEGER NOT NULL,
  rows        TEXT    NOT NULL,
  finished_at INTEGER NOT NULL,
  PRIMARY KEY (code, seat)
);
