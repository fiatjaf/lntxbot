CREATE TABLE account (
  id int PRIMARY KEY, -- telegram id
  username text NOT NULL, -- telegram username
  chat_id int -- telegram private chat id
);

CREATE INDEX ON account (username);

CREATE TABLE transaction (
  account_id int NOT NULL REFERENCES account (id),
  
);

table account;
