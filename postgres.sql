CREATE EXTENSION pgcrypto;
CREATE SCHEMA telegram;
CREATE SCHEMA lightning;

CREATE TABLE telegram.account (
  id serial PRIMARY KEY,
  telegram_id int UNIQUE, -- telegram id
  username text UNIQUE, -- telegram name
  chat_id int, -- telegram private chat id
  password text NOT NULL DEFAULT encode(digest(random()::text, 'sha256'), 'hex'), -- used in lndhub interface
  locale text NOT NULL DEFAULT 'en', -- default language for messages
  appdata jsonb NOT NULL DEFAULT '{}' -- data for all apps this user have, as a map of {"appname": {anything}}
);

CREATE INDEX ON telegram.account (username);
CREATE INDEX ON telegram.account (telegram_id);

CREATE TABLE telegram.chat (
  telegram_id bigint PRIMARY KEY,
  locale text NOT NULL DEFAULT 'en',
  spammy boolean NOT NULL DEFAULT false,
  ticket int NOT NULL DEFAULT 0
);

CREATE TABLE lightning.transaction (
  time timestamp NOT NULL DEFAULT now(),
  from_id int REFERENCES telegram.account (id),
  to_id int REFERENCES telegram.account (id),
  amount numeric(13) NOT NULL, -- in msatoshis
  fees int NOT NULL DEFAULT 0, -- in msatoshis
  description text,
  payment_hash text UNIQUE NOT NULL DEFAULT md5(random()::text) || md5(random()::text),
  label text, -- null on internal sends/tips
  preimage text,
  pending boolean NOT NULL DEFAULT false,
  trigger_message int NOT NULL DEFAULT 0,
  remote_node text,
  anonymous boolean NOT NULL DEFAULT false
);

CREATE INDEX ON lightning.transaction (from_id);
CREATE INDEX ON lightning.transaction (to_id);
CREATE INDEX ON lightning.transaction (label);
CREATE INDEX ON lightning.transaction (payment_hash);

CREATE VIEW lightning.account_txn AS
  SELECT
    time, account_id, anonymous, trigger_message, amount,
    CASE
      WHEN label IS NULL THEN coalesce(t.username, t.telegram_id::text)
      ELSE NULL
    END AS telegram_peer,
    status, fees, payment_hash, label, description, preimage, payee_node
  FROM (
      SELECT time,
        from_id AS account_id,
        anonymous,
        trigger_message,
        CASE WHEN pending THEN 'PENDING' ELSE 'SENT' END AS status,
        to_id AS peer,
        -amount AS amount, fees,
        payment_hash, label, description, preimage,
        remote_node AS payee_node
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
        payment_hash, label, description, preimage,
        NULL as payee_node
      FROM lightning.transaction
      WHERE to_id IS NOT NULL
  ) AS x
  LEFT OUTER JOIN telegram.account AS t ON x.peer = t.id;

CREATE VIEW lightning.balance AS
    SELECT
      account.id AS account_id,
      (coalesce(sum(amount), 0) - coalesce(sum(fees), 0))::float AS balance
    FROM lightning.account_txn
    RIGHT OUTER JOIN telegram.account AS account ON account_id = account.id
    GROUP BY account.id;

CREATE FUNCTION is_unclaimed(tx lightning.transaction) RETURNS boolean AS $$
  -- a user is potentially inactive if it doesn't have an active chat or has called /stop
  -- a user is only _truly_ inactive if it haven't made any outgoing transactions.
  WITH potentially_inactive_user AS (
    SELECT *
    FROM telegram.account AS acct
    WHERE acct.id = tx.to_id
  )
  SELECT CASE
    WHEN id IS NOT NULL AND chat_id IS NULL THEN CASE
      WHEN (
        SELECT count(*) AS total FROM lightning.transaction
        WHERE from_id = (SELECT id FROM potentially_inactive_user)
      ) = 0 THEN true
      ELSE false
    END
    ELSE false
  END FROM potentially_inactive_user
$$ LANGUAGE SQL;

table telegram.account;
table telegram.chat;
table lightning.transaction;
table lightning.account_txn;
table lightning.balance;
select * from lightning.transaction where pending;
select * from telegram.account inner join lightning.balance on id = account_id where chat_id is not null order by id;
select count(*) as active_users from telegram.account inner join lightning.balance as b on account_id = id where chat_id is not null and b.balance > 1000000;
select sum(balance) from lightning.balance;
