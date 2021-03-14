CREATE SCHEMA lightning;

CREATE TABLE account (
  id serial PRIMARY KEY,

  telegram_id int UNIQUE,
  telegram_username text UNIQUE,
  telegram_chat_id int, -- telegram private chat id

  discord_id text UNIQUE,
  discord_username text UNIQUE,
  discord_channel_id text, -- telegram private chat id

  password text NOT NULL DEFAULT md5(random()::text) || md5(random()::text), -- used in lndhub interface
  locale text NOT NULL DEFAULT 'en', -- default language for messages
  manual_locale boolean NOT NULL DEFAULT false,
  appdata jsonb NOT NULL DEFAULT '{}' -- data for all apps this user have, as a map of {"appname": {anything}}
);

CREATE TABLE balance_check (
  service text NOT NULL, -- a domain name
  account int REFERENCES account (id),
  url text NOT NULL,

  PRIMARY KEY(service, account)
);

CREATE TABLE groupchat (
  telegram_id bigint UNIQUE,
  discord_guild_id TEXT UNIQUE,
  locale text NOT NULL DEFAULT 'en',
  spammy boolean NOT NULL DEFAULT false,
  ticket int NOT NULL DEFAULT 0,
  renamable int NOT NULL DEFAULT 0,
  coinflips bool NOT NULL DEFAULT true
);

CREATE TABLE lightning.transaction (
  time timestamp NOT NULL DEFAULT now(),
  from_id int REFERENCES account (id),
  to_id int REFERENCES account (id),
  amount numeric(13) NOT NULL, -- in msatoshis
  fees int NOT NULL DEFAULT 0, -- in msatoshis
  description text,
  payment_hash text UNIQUE NOT NULL DEFAULT md5(random()::text) || md5(random()::text),
  label text, -- null on internal sends/tips
  preimage text,
  pending boolean NOT NULL DEFAULT false,
  trigger_message int NOT NULL DEFAULT 0,
  remote_node text,
  anonymous boolean NOT NULL DEFAULT false,
  tag text,
  proxied_with text -- the transaction related to this if used the proxy account
);

CREATE INDEX ON lightning.transaction (from_id);
CREATE INDEX ON lightning.transaction (to_id);
CREATE INDEX ON lightning.transaction (label);
CREATE INDEX ON lightning.transaction (payment_hash);
CREATE INDEX ON lightning.transaction (pending);
CREATE INDEX ON lightning.transaction (proxied_with);

CREATE VIEW lightning.account_txn AS
  SELECT
    time, account_id, anonymous, trigger_message, amount, pending,
    CASE WHEN t.username != '@'
      THEN coalesce(t.username, t.telegram_id::text)
      ELSE NULL
    END AS telegram_peer,
    status, fees, payment_hash, description, tag, preimage, payee_node
  FROM (
      SELECT time,
        from_id AS account_id,
        anonymous,
        trigger_message,
        CASE WHEN pending THEN 'PENDING' ELSE 'SENT' END AS status,
        to_id AS peer,
        -amount AS amount, fees,
        payment_hash, description, tag, preimage,
        remote_node AS payee_node,
        pending
      FROM lightning.transaction
      WHERE from_id IS NOT NULL
    UNION ALL
      SELECT time,
        to_id AS account_id,
        anonymous,
        CASE WHEN from_id IS NULL THEN trigger_message ELSE 0 END AS trigger_message,
        'RECEIVED' AS status,
        from_id AS peer,
        amount, 0 AS fees,
        payment_hash, description, tag, preimage,
        NULL as payee_node,
        pending
      FROM lightning.transaction
      WHERE to_id IS NOT NULL
  ) AS x
  LEFT OUTER JOIN account AS t ON x.peer = t.id;

CREATE VIEW lightning.balance AS
    SELECT
      account.id AS account_id,
      (
        coalesce(sum(amount), 0) -
        coalesce(sum(fees), 0)
      )::numeric(13) AS balance
    FROM lightning.account_txn
    RIGHT OUTER JOIN account AS account ON account_id = account.id
    WHERE amount <= 0 OR (amount > 0 AND pending = false)
    GROUP BY account.id;

CREATE OR REPLACE FUNCTION is_unclaimed(tx lightning.transaction) RETURNS boolean AS $$
  -- a user is potentially inactive if it doesn't have an active chat or has called /stop
  -- a user is only _truly_ inactive if it haven't made any outgoing transactions.
  WITH potentially_inactive_user AS (
    SELECT *
    FROM account AS acct
    WHERE acct.id = tx.to_id
  )
  SELECT CASE
    WHEN id IS NOT NULL AND telegram_chat_id IS NULL AND discord_channel_id IS NULL THEN CASE
      WHEN (
        SELECT count(*) AS total FROM lightning.transaction
        WHERE from_id = (SELECT id FROM potentially_inactive_user)
      ) = 0 THEN true
      ELSE false
    END
    ELSE false
  END FROM potentially_inactive_user
$$ LANGUAGE SQL;
