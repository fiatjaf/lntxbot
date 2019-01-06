CREATE TABLE account (
  id int PRIMARY KEY, -- telegram id
  username text NOT NULL,
  lndhub_credentials text -- encrypted with secret_key + : + telegram id
);

CREATE INDEX ON account (username);
