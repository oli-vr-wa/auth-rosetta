package com.authrosetta.java.store;

import com.authrosetta.java.auth.User;
import com.authrosetta.java.config.AppConfig;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.jdbc.core.simple.JdbcClient;

import javax.sql.DataSource;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.Callable;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

import static org.junit.jupiter.api.Assertions.*;

@Tag("integration")
class PostgresStoreIntegrationTest {

    private static JdbcClient jdbc;
    private static PostgresUserRepository users;
    private static PostgresRevokedTokenRepository revokedTokens;

    @BeforeAll
    static void setUpDatabase() {
        AppConfig cfg = AppConfig.loadDefault();
        DataSource ds = DataSourceBuilder.create()
            .driverClassName("org.postgresql.Driver")
            .url(cfg.jdbcUrl())
            .build();
        jdbc = JdbcClient.create(ds);
        users = new PostgresUserRepository(jdbc);
        revokedTokens = new PostgresRevokedTokenRepository(jdbc);
    }

    @AfterEach
    void cleanUpTestRows() {
        jdbc.sql("DELETE FROM users WHERE email LIKE 'store-test-%@example.com'").update();
        jdbc.sql("DELETE FROM revoked_tokens WHERE jti LIKE 'store-test-%'").update();
    }

    @Test
    void users_crudAndUniqueEmailDetection() {
        String email = "store-test-" + UUID.randomUUID() + "@example.com";
        UUID id = UUID.randomUUID();
        Instant now = Instant.now().truncatedTo(ChronoUnit.MILLIS);

        User user = new User(id, email, "$2a$12$hashedpassword", "JBSWY3DPEHPK3PXP", false, now);
        assertTrue(users.add(user));

        User fetched = users.findByEmail(email).orElseThrow();
        assertEquals(id, fetched.id());
        assertEquals(email, fetched.email());
        assertFalse(fetched.twoFactorEnabled());

        users.enableTwoFactor(id);
        User updated = users.findById(id).orElseThrow();
        assertTrue(updated.twoFactorEnabled());

        // Duplicate email returns false (uq_users_email)
        User duplicateEmail = new User(UUID.randomUUID(), email, "$2a$12$other", "OTHERSECRET", false, now);
        assertFalse(users.add(duplicateEmail));

        // Duplicate primary key (pk_users) is NOT swallowed as duplicate email
        String otherEmail = "store-test-" + UUID.randomUUID() + "@example.com";
        User duplicatePk = new User(id, otherEmail, "$2a$12$secret_value_should_not_leak", "SECRET_TOTP", false, now);
        DataIntegrityViolationException pkEx = assertThrows(
            DataIntegrityViolationException.class,
            () -> users.add(duplicatePk)
        );
        assertFalse(pkEx.getMessage().contains("secret_value_should_not_leak"));
        assertFalse(pkEx.getMessage().contains("SECRET_TOTP"));
    }

    @Test
    void users_eightWayConcurrentRegister_exactlyOneSucceeds() throws Exception {
        String sharedEmail = "store-test-race-" + UUID.randomUUID() + "@example.com";
        int workers = 8;
        CountDownLatch startGate = new CountDownLatch(1);
        List<Callable<Boolean>> tasks = new ArrayList<>();

        for (int i = 0; i < workers; i++) {
            tasks.add(() -> {
                startGate.await();
                User candidate = new User(
                    UUID.randomUUID(),
                    sharedEmail,
                    "$2a$12$hash",
                    "JBSWY3DPEHPK3PXP",
                    false,
                    Instant.now()
                );
                return users.add(candidate);
            });
        }

        try (ExecutorService pool = Executors.newFixedThreadPool(workers)) {
            List<Future<Boolean>> futures = new ArrayList<>();
            for (Callable<Boolean> task : tasks) {
                futures.add(pool.submit(task));
            }
            startGate.countDown();

            long insertedCount = 0;
            for (Future<Boolean> future : futures) {
                if (future.get()) {
                    insertedCount++;
                }
            }
            assertEquals(1, insertedCount);
        }
    }

    @Test
    void revokedTokens_idempotentAndConcurrentSafe() throws Exception {
        String jti = "store-test-" + UUID.randomUUID();
        Instant expiresAt = Instant.now().plus(1, ChronoUnit.HOURS);

        assertFalse(revokedTokens.isRevoked(jti));

        int workers = 8;
        CountDownLatch startGate = new CountDownLatch(1);
        List<Callable<Void>> tasks = new ArrayList<>();
        for (int i = 0; i < workers; i++) {
            tasks.add(() -> {
                startGate.await();
                revokedTokens.revoke(jti, expiresAt);
                return null;
            });
        }

        try (ExecutorService pool = Executors.newFixedThreadPool(workers)) {
            List<Future<Void>> futures = new ArrayList<>();
            for (Callable<Void> task : tasks) {
                futures.add(pool.submit(task));
            }
            startGate.countDown();
            for (Future<Void> future : futures) {
                future.get();
            }
        }

        assertTrue(revokedTokens.isRevoked(jti));
    }
}