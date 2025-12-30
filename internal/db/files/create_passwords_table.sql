CREATE TABLE IF NOT EXISTS
    passwords
    ( id TEXT PRIMARY KEY
    , password_enc BLOB NOT NULL
    , created_on TEXT DEFAULT CURRENT_TIMESTAMP
    , updated_on TEXT DEFAULT CURRENT_TIMESTAMP
    );
