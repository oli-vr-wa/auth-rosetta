-- +goose Up
CREATE TABLE users (
    id                 uuid         NOT NULL,
    email              varchar(256) NOT NULL,
    password_hash      text         NOT NULL,
    totp_secret        text         NOT NULL,
    two_factor_enabled boolean      NOT NULL DEFAULT false,
    created_at         timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE TABLE revoked_tokens (
    jti        varchar(64) NOT NULL,
    expires_at timestamptz NOT NULL,
    CONSTRAINT pk_revoked_tokens PRIMARY KEY (jti)
);

-- +goose Down
DROP TABLE revoked_tokens;
DROP TABLE users;
