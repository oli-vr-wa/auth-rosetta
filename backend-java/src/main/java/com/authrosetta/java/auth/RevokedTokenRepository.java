package com.authrosetta.java.auth;

import java.time.Instant;

public interface RevokedTokenRepository {
    /**
     * Method to revoke a token by its JTI.
     * @param jti the JTI of the token to revoke
     * @param revokedAt the timestamp when the token was revoked
     */
    void revoke(String jti, Instant revokedAt);
    
    /**
     * Checks if a token with the given JTI is revoked.
     * @param jti the JTI of the token to check
     * @return true if the token is revoked, false otherwise
     */
    boolean isRevoked(String jti);
}
