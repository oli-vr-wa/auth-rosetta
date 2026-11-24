package com.authrosetta.java.auth;

import com.authrosetta.java.security.JwtService;
import com.authrosetta.java.security.PasswordHasher;
import com.authrosetta.java.security.TotpService;
import com.authrosetta.java.validation.ValidationException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.HashMap;
import java.util.Locale;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

import static org.junit.jupiter.api.Assertions.*;

class AuthServiceTest {

    private static final Instant FIXED_NOW = Instant.parse("2026-11-24T03:00:00Z");
    private static final byte[] JWT_KEY = "0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.UTF_8);

    private InMemoryUserRepository userRepo;
    private InMemoryRevokedTokenRepository revokedRepo;
    private PasswordHasher passwords;
    private TotpService totp;
    private JwtService jwt;
    private AuthService service;

    @BeforeEach
    void setUp() {
        Clock clock = Clock.fixed(FIXED_NOW, ZoneOffset.UTC);
        userRepo = new InMemoryUserRepository();
        revokedRepo = new InMemoryRevokedTokenRepository();
        passwords = new PasswordHasher(4); // fast bcrypt cost for unit tests
        totp = new TotpService();
        jwt = new JwtService(JWT_KEY, clock);
        service = new AuthService(userRepo, revokedRepo, passwords, totp, jwt, clock);
    }

    @Test
    void fullHappyPath_registerConfirmLoginAuthenticateLogout() throws Exception {
        AuthService.RegisterResult reg = service.register("  Alice@Example.COM ", "correcthorse");
        assertNotNull(reg.secret());
        assertTrue(reg.otpauthUri().contains("AuthRosetta-Java"));

        // Login before 2FA confirmation fails with generic INVALID_CREDENTIALS
        String code = currentTotpCode(reg.secret(), FIXED_NOW);
        AuthException unconfirmedEx = assertThrows(
            AuthException.class,
            () -> service.login("alice@example.com", "correcthorse", code)
        );
        assertEquals(AuthError.INVALID_CREDENTIALS, unconfirmedEx.error());

        // Confirm 2FA (and confirming a second time is harmless)
        service.confirmTwoFactor("ALICE@example.com", code);
        assertDoesNotThrow(() -> service.confirmTwoFactor("alice@example.com", code));

        // Login succeeds and issues valid JWT
        AuthService.LoginResult login = service.login("alice@example.com", "correcthorse", code);
        assertNotNull(login.token());

        AuthService.AuthenticatedUser me = service.authenticate(login.token());
        assertEquals("alice@example.com", me.email());

        // Logout revokes token; subsequent authenticate and logout fail with UNAUTHORIZED
        service.logout(login.token());
        AuthException afterLogout = assertThrows(AuthException.class, () -> service.authenticate(login.token()));
        assertEquals(AuthError.UNAUTHORIZED, afterLogout.error());
    }

    @Test
    void register_rejectsInvalidInputDuplicateEmailAndConcurrentRace() {
        assertThrows(ValidationException.class, () -> service.register("not-an-email", "short"));

        service.register("alice@example.com", "correcthorse");
        AuthException dup = assertThrows(
            AuthException.class,
            () -> service.register("ALICE@example.com", "otherpassword")
        );
        assertEquals(AuthError.EMAIL_TAKEN, dup.error());

        // Simulate lost concurrent insert race where findByEmail returns empty, then add() returns false
        userRepo.forceAddConflict = true;
        AuthException race = assertThrows(
            AuthException.class,
            () -> service.register("race@example.com", "correcthorse")
        );
        assertEquals(AuthError.EMAIL_TAKEN, race.error());
    }

    @Test
    void confirmTwoFactor_mapsAllFailuresToInvalidCode() {
        AuthService.RegisterResult reg = service.register("alice@example.com", "correcthorse");

        assertEquals(AuthError.INVALID_CODE,
            assertThrows(AuthException.class, () -> service.confirmTwoFactor("unknown@example.com", "123456")).error());
        assertEquals(AuthError.INVALID_CODE,
            assertThrows(AuthException.class, () -> service.confirmTwoFactor("bad\uD800@example.com", "123456")).error());
        assertEquals(AuthError.INVALID_CODE,
            assertThrows(AuthException.class, () -> service.confirmTwoFactor("alice@example.com", "000000")).error());
        assertEquals(AuthError.INVALID_CODE,
            assertThrows(AuthException.class, () -> service.confirmTwoFactor("alice@example.com", "12\u0000456")).error());
        assertNotNull(reg);
    }

    @Test
    void login_rejectsWrongPasswordWrongCodeUnknownEmailAndOddText() throws Exception {
        AuthService.RegisterResult reg = service.register("alice@example.com", "correcthorse");
        String validCode = currentTotpCode(reg.secret(), FIXED_NOW);
        service.confirmTwoFactor("alice@example.com", validCode);

        assertEquals(AuthError.INVALID_CREDENTIALS,
            assertThrows(AuthException.class, () -> service.login("unknown@example.com", "correcthorse", validCode)).error());
        assertEquals(AuthError.INVALID_CREDENTIALS,
            assertThrows(AuthException.class, () -> service.login("alice@example.com", "wrongpassword", validCode)).error());
        assertEquals(AuthError.INVALID_CREDENTIALS,
            assertThrows(AuthException.class, () -> service.login("alice@example.com", "correcthorse", "000000")).error());
        assertEquals(AuthError.INVALID_CREDENTIALS,
            assertThrows(AuthException.class, () -> service.login("bad\uD800@example.com", "correcthorse", validCode)).error());
        assertEquals(AuthError.INVALID_CREDENTIALS,
            assertThrows(AuthException.class, () -> service.login("alice@example.com", "bad\u0000pass", validCode)).error());
    }

    @Test
    void authenticate_propagatesDatabaseErrorsInsteadOfMaskingAsUnauthorized() throws Exception {
        AuthService.RegisterResult reg = service.register("alice@example.com", "correcthorse");
        String code = currentTotpCode(reg.secret(), FIXED_NOW);
        service.confirmTwoFactor("alice@example.com", code);
        String token = service.login("alice@example.com", "correcthorse", code).token();

        revokedRepo.failOnQuery = true;
        assertThrows(RuntimeException.class, () -> service.authenticate(token));
    }

    // --- Test helpers & fakes ---

    private static String currentTotpCode(String base32Secret, Instant at) throws Exception {
        String alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
        byte[] key = new byte[(base32Secret.length() * 5) / 8];
        int buffer = 0, bits = 0, count = 0;
        for (char c : base32Secret.toCharArray()) {
            buffer = (buffer << 5) | alphabet.indexOf(c);
            bits += 5;
            if (bits >= 8) {
                key[count++] = (byte) ((buffer >> (bits - 8)) & 0xFF);
                bits -= 8;
            }
        }
        long step = Math.floorDiv(at.getEpochSecond(), 30L);
        Mac mac = Mac.getInstance("HmacSHA1");
        mac.init(new SecretKeySpec(key, "HmacSHA1"));
        byte[] hash = mac.doFinal(ByteBuffer.allocate(8).putLong(step).array());
        int offset = hash[hash.length - 1] & 0x0F;
        int binary = ((hash[offset] & 0x7F) << 24)
            | ((hash[offset + 1] & 0xFF) << 16)
            | ((hash[offset + 2] & 0xFF) << 8)
            | (hash[offset + 3] & 0xFF);
        return String.format(Locale.ROOT, "%06d", binary % 1_000_000);
    }

    static class InMemoryUserRepository implements UserRepository {
        final Map<UUID, User> byId = new HashMap<>();
        boolean forceAddConflict = false;

        @Override
        public Optional<User> findByEmail(String normalizedEmail) {
            return byId.values().stream()
                .filter(u -> u.email().equals(normalizedEmail))
                .findFirst();
        }

        @Override
        public Optional<User> findById(UUID id) {
            return Optional.ofNullable(byId.get(id));
        }

        @Override
        public boolean add(User user) {
            if (forceAddConflict || findByEmail(user.email()).isPresent()) {
                return false;
            }
            byId.put(user.id(), user);
            return true;
        }

        @Override
        public void enableTwoFactor(UUID id) {
            User existing = byId.get(id);
            if (existing != null) {
                byId.put(id, new User(
                    existing.id(),
                    existing.email(),
                    existing.passwordHash(),
                    existing.totpSecret(),
                    true,
                    existing.createdAt()
                ));
            }
        }
    }

    static class InMemoryRevokedTokenRepository implements RevokedTokenRepository {
        final Set<String> revoked = ConcurrentHashMap.newKeySet();
        boolean failOnQuery = false;

        @Override
        public void revoke(String jti, Instant expiresAt) {
            revoked.add(jti);
        }

        @Override
        public boolean isRevoked(String jti) {
            if (failOnQuery) {
                throw new RuntimeException("simulated database connection failure");
            }
            return revoked.contains(jti);
        }
    }
}