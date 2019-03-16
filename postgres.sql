CREATE SCHEMA telegram;
CREATE SCHEMA lightning;

CREATE TABLE telegram.account (
  id serial PRIMARY KEY,
  telegram_id int UNIQUE, -- telegram id
  username text UNIQUE, -- telegram name
  chat_id int -- telegram private chat id
);

CREATE INDEX ON telegram.account (username);
CREATE INDEX ON telegram.account (telegram_id);

CREATE TABLE telegram.chat (
  telegram_id bigint PRIMARY KEY,
  spammy boolean NOT NULL DEFAULT false
);

CREATE TABLE lightning.transaction (
  time timestamp NOT NULL DEFAULT now(),
  from_id int REFERENCES telegram.account (id),
  to_id int REFERENCES telegram.account (id),
  amount int NOT NULL, -- in msatoshis
  fees int NOT NULL DEFAULT 0, -- in msatoshis
  description text,
  payment_hash text UNIQUE NOT NULL DEFAULT md5(random()::text) || md5(random()::text),
  label text, -- null on internal sends/tips
  preimage text,
  pending_bolt11 text,
  trigger_message int NOT NULL DEFAULT 0
);

CREATE INDEX ON lightning.transaction (from_id);
CREATE INDEX ON lightning.transaction (to_id);
CREATE INDEX ON lightning.transaction (label);
CREATE INDEX ON lightning.transaction (payment_hash);

CREATE VIEW lightning.account_txn AS
  SELECT
    time, account_id, trigger_message, amount,
    CASE
      WHEN label IS NULL THEN coalesce(t.username, t.telegram_id::text)
      ELSE NULL
    END AS telegram_peer,
    status, fees, payment_hash, label, description, preimage, pending_bolt11
  FROM (
    SELECT time,
      from_id AS account_id,
      trigger_message,
      CASE WHEN pending_bolt11 IS NOT NULL THEN 'PENDING' ELSE 'SENT' END AS status,
      to_id AS peer,
      -amount AS amount, fees,
      payment_hash, label, description, preimage, pending_bolt11
    FROM lightning.transaction
    WHERE from_id IS NOT NULL
  UNION ALL
    SELECT time,
      to_id AS account_id,
      CASE WHEN from_id IS NULL THEN trigger_message ELSE 0 END AS trigger_message,
      'RECEIVED' AS status,
      from_id AS peer,
      amount, 0 AS fees,
      payment_hash, label, description, preimage, pending_bolt11
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
select * from lightning.transaction where pending_bolt11 IS NOT NULL;
select * from telegram.account inner join lightning.balance on id = account_id order by id;
select count(*) as active_users from telegram.account where chat_id is not null;
select sum(balance)*4000/100000000000 as us_dollars from lightning.balance;
