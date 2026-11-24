package com.authrosetta.java.auth;

import com.authrosetta.java.security.JwtService;
import com.authrosetta.java.security.PasswordHasher;
import com.authrosetta.java.security.TotpService;
import com.authrosetta.java.validation.ValidationRules;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;

import java.time.Clock;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

@Service
public class AuthService {

    private static final Logger log = LoggerFactory.getLogger(AuthService.class);

    private final UserRepository users;
    private final RevokedTokenRepository revokedTokens;
    private final PasswordHasher passwords;
    private final TotpService totp;
    private final JwtService jwt;
    private final Clock clock;

    public record RegisterResult(String otpauthUri, String secret) {}
    public record LoginResult(String token, Instant expiresAt) {}
    public record AuthenticatedUser(UUID id, String email, String jti, Instant expiresAt) {}

    public AuthService(
        UserRepository users,
        RevokedTokenRepository revokedTokens,
        PasswordHasher passwords,
        TotpService totp,
        JwtService jwt,
        Clock clock
    ) {
        this.users = users;
        this.revokedTokens = revokedTokens;
        this.passwords = passwords;
        this.totp = totp;
        this.jwt = jwt;
        this.clock = clock;
    }

    public RegisterResult register(String rawEmail, String password) {
        ValidationRules.validateRegister(rawEmail, password);
        String email = ValidationRules.normalizeEmail(rawEmail);

        if (users.findByEmail(email).isPresent()) {
            throw new AuthException(AuthError.EMAIL_TAKEN);
        }

        TotpService.Enrollment enrollment = totp.createEnrollment(email);
        String passwordHash = passwords.hash(password);
        User newUser = new User(
            UUID.randomUUID(),
            email,
            passwordHash,
            enrollment.secret(),
            false,
            clock.instant()
        );

        if (!users.add(newUser)) {
            throw new AuthException(AuthError.EMAIL_TAKEN);
        }

        return new RegisterResult(enrollment.otpauthUri(), enrollment.secret());
    }

    public void confirmTwoFactor(String rawEmail, String code) {
        if (!ValidationRules.isStorable(code)) {
            throw new AuthException(AuthError.INVALID_CODE);
        }
        User user = findStorableUser(rawEmail)
            .orElseThrow(() -> new AuthException(AuthError.INVALID_CODE));

        if (!totp.verify(user.totpSecret(), code, clock.instant())) {
            throw new AuthException(AuthError.INVALID_CODE);
        }

        users.enableTwoFactor(user.id());
    }

    public LoginResult login(String rawEmail, String password, String code) {
        Optional<User> maybeUser = findStorableUser(rawEmail);
        boolean userFound = maybeUser.isPresent();

        boolean passwordValid;
        boolean totpValid;
        boolean twoFactorEnabled;

        if (userFound) {
            User user = maybeUser.get();
            passwordValid = passwords.verify(password, user.passwordHash());
            totpValid = totp.verify(user.totpSecret(), code, clock.instant());
            twoFactorEnabled = user.twoFactorEnabled();
        } else {
            passwords.verifyAgainstDummy();
            passwordValid = false;
            totpValid = false;
            twoFactorEnabled = false;
        }

        if (!userFound || !passwordValid || !totpValid || !twoFactorEnabled) {
            log.debug(
                "Login failed (userFound={}, passwordValid={}, totpValid={}, twoFactorEnabled={})",
                userFound,
                passwordValid,
                totpValid,
                twoFactorEnabled
            );
            throw new AuthException(AuthError.INVALID_CREDENTIALS);
        }

        User user = maybeUser.get();
        JwtService.IssuedToken issued = jwt.issue(user.id(), user.email());
        return new LoginResult(issued.token(), issued.claims().expiresAt());
    }

    public AuthenticatedUser authenticate(String token) {
        JwtService.TokenClaims claims = jwt.verify(token)
            .orElseThrow(() -> new AuthException(AuthError.UNAUTHORIZED));

        if (revokedTokens.isRevoked(claims.jti())) {
            log.debug("Token rejected: jti is revoked");
            throw new AuthException(AuthError.UNAUTHORIZED);
        }

        return new AuthenticatedUser(claims.userId(), claims.email(), claims.jti(), claims.expiresAt());
    }

    public void logout(String token) {
        AuthenticatedUser authenticated = authenticate(token);
        revokedTokens.revoke(authenticated.jti(), authenticated.expiresAt());
    }

    private Optional<User> findStorableUser(String rawEmail) {
        if (!ValidationRules.isStorable(rawEmail)) {
            return Optional.empty();
        }
        String normalized = ValidationRules.normalizeEmail(rawEmail);
        if (normalized.isEmpty()) {
            return Optional.empty();
        }
        return users.findByEmail(normalized);
    }
}