package com.authrosetta.java.auth;

import java.time.Instant;
import java.util.UUID;

public record User (
    UUID id,
    String email,
    String passwordHash,
    String totpSecret,
    boolean twoFactorEnabled,
    Instant createdAt
) {}
