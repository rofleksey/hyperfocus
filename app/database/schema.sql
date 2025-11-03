CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;

CREATE TABLE IF NOT EXISTS streams
(
  id           VARCHAR(255) PRIMARY KEY,
  updated      TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  url          TEXT,
  online       BOOLEAN        NOT NULL default TRUE,
  player_names VARCHAR(255)[] NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS schema_version
(
  version INTEGER PRIMARY KEY DEFAULT 0
);
INSERT INTO schema_version (version)
VALUES (0)
ON CONFLICT DO NOTHING;
