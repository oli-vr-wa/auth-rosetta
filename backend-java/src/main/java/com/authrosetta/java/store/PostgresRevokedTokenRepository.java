package com.authrosetta.java.store;

import java.sql.Timestamp;
import java.time.Instant;

import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.stereotype.Repository;

import com.authrosetta.java.auth.RevokedTokenRepository;

@Repository 
public class PostgresRevokedTokenRepository implements RevokedTokenRepository {

    private final JdbcClient jdbc;

    public PostgresRevokedTokenRepository(JdbcClient jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    public void revoke(String jti, Instant expiresAt) {
        jdbc.sql("""
                INSERT INTO revoked_tokens (jti, expires_at)
                VALUES (:jti, :expiresAt)
                ON CONFLICT (jti) DO NOTHING
                """)
            .param("jti", jti)
            .param("expiresAt", Timestamp.from(expiresAt))
            .update();
    }

    @Override
    public boolean isRevoked(String jti) {
        Integer found = jdbc.sql("""
                SELECT 1
                FROM revoked_tokens
                WHERE jti = :jti
                """)
            .param("jti", jti)
            .query(Integer.class)
            .optional()
            .orElse(null);
        return found != null;
    }
    
}
