CREATE TABLE registration_verifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel TEXT NOT NULL,
    identifier TEXT NOT NULL,
    code TEXT NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX idx_registration_verifications_channel_identifier
    ON registration_verifications (channel, identifier);
