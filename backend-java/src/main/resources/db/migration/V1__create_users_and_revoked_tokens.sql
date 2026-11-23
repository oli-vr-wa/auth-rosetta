CREATE TABLE users (
    id UUID NOT NULL,
    email VARCHAR(256) NOT NULL,
    password_hash TEXT NOT NULL,
    totp_secret TEXT NOT NULL,
    two_factor_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE TABLE revoked_tokens (
    jti VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT pk_revoked_tokens PRIMARY KEY (jti)
);