package com.authrosetta.java.store;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.stereotype.Repository;
import org.postgresql.util.PSQLException;
import org.postgresql.util.ServerErrorMessage;

import com.authrosetta.java.auth.User;
import com.authrosetta.java.auth.UserRepository;

@Repository 
public class PostgresUserRepository implements UserRepository {

    private static final String UNIQUE_VIOLATION_SQLSTATE = "23505";
    private static final String EMAIL_CONSTRAINT = "uq_users_email";

    private static final RowMapper<User> USER_ROW_MAPPER = (rs, rowNum) -> new User(
        rs.getObject("id", UUID.class),
        rs.getString("email"),
        rs.getString("password_hash"),
        rs.getString("totp_secret"),
        rs.getBoolean("two_factor_enabled"),
        rs.getTimestamp("created_at").toInstant()
    );

    private final JdbcClient jdbc;

    public PostgresUserRepository(JdbcClient jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    public Optional<User> findByEmail(String normalizedEmail) {
        return jdbc.sql("""
            SELECT id, email, password_hash, totp_secret, two_factor_enabled, created_at
                FROM users
                WHERE email = :email
            """)
            .param("email", normalizedEmail)
            .query(USER_ROW_MAPPER)
            .optional();
    }

    @Override
    public Optional<User> findById(UUID id) {
        return jdbc.sql("""
            SELECT id, email, password_hash, totp_secret, two_factor_enabled, created_at
                FROM users
                WHERE id = :id
            """)
            .param("id", id)
            .query(USER_ROW_MAPPER)
            .optional();
    }

    @Override
    public boolean add(User user) {
        try {
            jdbc.sql("""
                INSERT INTO users (id, email, password_hash, totp_secret, two_factor_enabled, created_at)
                VALUES (:id, :email, :password_hash, :totp_secret, :two_factor_enabled, :created_at)
                """)
                .param("id", user.id())
                .param("email", user.email())
                .param("password_hash", user.passwordHash())
                .param("totp_secret", user.totpSecret())
                .param("two_factor_enabled", user.twoFactorEnabled())
                .param("created_at", Timestamp.from(user.createdAt() != null ? user.createdAt() : Instant.now()))
                .update();
            return true;
        } catch (DataIntegrityViolationException ex) {
            if (isEmailUniqueViolation(ex)) {
                return false;
            }
            throw ex;
        }
    }

    @Override
    public void enableTwoFactor(UUID id) {
        jdbc.sql("""
                UPDATE users
                SET two_factor_enabled = TRUE
                WHERE id = :id
                """)
            .param("id", id)
            .update();
    }
 
    static boolean isEmailUniqueViolation(Throwable throwable) {
        Throwable current = throwable;
        while (current != null) {
            if (current instanceof PSQLException psqlEx && UNIQUE_VIOLATION_SQLSTATE.equals(psqlEx.getSQLState())) {
                ServerErrorMessage serverError = psqlEx.getServerErrorMessage();
                if (serverError != null && EMAIL_CONSTRAINT.equals(serverError.getConstraint())) {
                    return true;
                }
            }
            current = current.getCause();
        }
        return false;
    }
}
