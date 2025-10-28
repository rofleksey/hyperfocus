CREATE TABLE IF NOT EXISTS users
(
  username           VARCHAR(64) PRIMARY KEY,
  created            TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  password_hash      VARCHAR(255)   NOT NULL,
  roles              VARCHAR(255)[] NOT NULL,
  last_session_reset TIMESTAMP
);

CREATE TABLE IF NOT EXISTS streams
(
  id           VARCHAR(255) PRIMARY KEY,
  updated      TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  online       BOOLEAN        NOT NULL default TRUE,
  player_names VARCHAR(255)[] NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS settings
(
  id                INTEGER PRIMARY KEY,
  api_key           VARCHAR(256),
  notification_urls TEXT[],
  autodelete_days   INTEGER
);
INSERT INTO settings (id)
VALUES (1)
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS schema_version
(
  version INTEGER PRIMARY KEY DEFAULT 0
);
INSERT INTO schema_version (version)
VALUES (0)
ON CONFLICT DO NOTHING;
