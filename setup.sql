CREATE TABLE posts (
       id      INTEGER PRIMARY KEY,
       created INTEGER DEFAULT (unixepoch()),
       content TEXT
);
