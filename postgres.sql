CREATE TABLE account (
  id int PRIMARY KEY, -- telegram id
  username text NOT NULL, -- telegram username
  chat_id int -- telegram private chat id
);

CREATE INDEX ON account (username);

CREATE TABLE transaction (
  account_id int NOT NULL REFERENCES account (id),
  amount int NOT NULL, -- in msatoshis (positive for receipts, negative for payments)
  fees int, -- in msatoshis (positive for payments, null for receipts)
  description text NOT NULL,
  payment_hash text PRIMARY KEY,
  label text,
  preimage text
);

table account;
table transaction;
SELECT coalesce(sum(amount), 0) - coalesce(sum(fees), 0) from transaction;
delete from transaction;
