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

CREATE TABLE lightning.transaction (
  time timestamp NOT NULL DEFAULT now(),
  from_id int REFERENCES telegram.account (id),
  to_id int REFERENCES telegram.account (id),
  amount int NOT NULL, -- in msatoshis
  fees int NOT NULL DEFAULT 0, -- in msatoshis
  description text, -- null on internal sends/tips
  payment_hash text NOT NULL DEFAULT md5(random()::text) || md5(random()::text), -- null on internal sends/tips
  label text, -- null on internal sends/tips
  preimage text
);

CREATE INDEX ON lightning.transaction (from_id);
CREATE INDEX ON lightning.transaction (to_id);
CREATE INDEX ON lightning.transaction (label);
CREATE INDEX ON lightning.transaction (payment_hash);

CREATE VIEW lightning.account_txn AS
  SELECT
    time, account_id, amount,
    CASE
      WHEN label IS NULL THEN coalesce(t.username, t.telegram_id::text)
      ELSE NULL
    END AS telegram_peer,
    status, fees, payment_hash, label, description, preimage
  FROM (
    SELECT time,
      from_id AS account_id,
      'SENT' AS status,
      to_id AS peer,
      -amount AS amount, fees,
      payment_hash, label, description, preimage
    FROM lightning.transaction
    WHERE from_id IS NOT NULL
  UNION ALL
    SELECT time,
      to_id AS account_id,
      'RECEIVED' AS status,
      from_id AS peer,
      amount, 0 AS fees,
      payment_hash, label, description, preimage
    FROM lightning.transaction
    WHERE to_id IS NOT NULL
  ) AS x
  LEFT OUTER JOIN telegram.account AS t ON x.peer = t.id;

CREATE VIEW lightning.balance AS
    SELECT
      account.id AS account_id,
      coalesce(sum(amount), 0) - coalesce(sum(fees), 0) AS balance
    FROM lightning.account_txn
    RIGHT OUTER JOIN telegram.account AS account ON account_id = account.id
    GROUP BY account.id;

table telegram.account;
table lightning.transaction;
table lightning.account_txn;
table lightning.balance;
