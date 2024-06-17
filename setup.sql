CREATE TABLE users (
       id              INTEGER PRIMARY KEY,
       username        TEXT,
       hashed_password BLOB
);

CREATE TABLE posts (
       id      INTEGER PRIMARY KEY,
       created INTEGER DEFAULT (unixepoch()),
       page    INTEGER,
       content TEXT
);
