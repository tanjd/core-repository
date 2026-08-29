CREATE TABLE invite_codes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    code       TEXT    NOT NULL,
    inviter_id INTEGER NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_invite_codes_code ON invite_codes(code);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invite_codes_inviter_id ON invite_codes(inviter_id);
