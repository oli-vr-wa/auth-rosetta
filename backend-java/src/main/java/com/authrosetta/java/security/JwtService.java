package com.authrosetta.java.security;

import com.authrosetta.java.config.AppConfig;
import io.jsonwebtoken.Claims;
import io.jsonwebtoken.JwtException;
import io.jsonwebtoken.JwtParser;
import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.Keys;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import javax.crypto.SecretKey;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Date;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;

@Component
public class JwtService {

    public static final String ISSUER = "AuthRosetta";
    public static final String AUDIENCE = "AuthRosetta";
    public static final Duration LIFETIME = Duration.ofMinutes(60);
    public static final long CLOCK_SKEW_SECONDS = 30L;

    private static final Logger log = LoggerFactory.getLogger(JwtService.class);

    private final SecretKey signingKey;
    private final Clock clock;
    private final JwtParser parser;

    public record TokenClaims(
        UUID userId,
        String email,
        String jti,
        Instant expiresAt
    ) {}

    public record IssuedToken(
        String token,
        TokenClaims claims
    ) {}

    @Autowired
    public JwtService(AppConfig config, Clock clock) {
        this(config.jwtKey(), clock);
    }

    public JwtService(byte[] keyBytes, Clock clock) {
        if (keyBytes == null || keyBytes.length < AppConfig.MIN_JWT_KEY_BYTES) {
            throw new IllegalArgumentException("JWT key must be at least 32 bytes.");
        }
        this.signingKey = Keys.hmacShaKeyFor(keyBytes);
        this.clock = clock;
        this.parser = Jwts.parser()
            .verifyWith(this.signingKey)
            .requireIssuer(ISSUER)
            .requireAudience(AUDIENCE)
            .clockSkewSeconds(CLOCK_SKEW_SECONDS)
            .clock(() -> Date.from(this.clock.instant()))
            .build();
    }

    public IssuedToken issue(UUID userId, String email) {
        Instant now = clock.instant();
        Instant expiresAt = now.plus(LIFETIME);
        String jti = UUID.randomUUID().toString();

        String token = Jwts.builder()
            .header().type("JWT").and()
            .issuer(ISSUER)
            .claim("aud", AUDIENCE)
            .subject(userId.toString())
            .claim("email", email)
            .id(jti)
            .expiration(Date.from(expiresAt))
            .signWith(signingKey, Jwts.SIG.HS256)
            .compact();

        return new IssuedToken(token, new TokenClaims(userId, email, jti, expiresAt));
    }

    public Optional<TokenClaims> verify(String token) {
        if (token == null || token.isBlank()) {
            log.debug("JWT rejected: empty token");
            return Optional.empty();
        }
        try {
            Claims claims = parser.parseSignedClaims(token).getPayload();

            String sub = claims.getSubject();
            String email = claims.get("email", String.class);
            String jti = claims.getId();
            Date exp = claims.getExpiration();

            if (sub == null || sub.isBlank()
                || email == null || email.isBlank()
                || jti == null || jti.isBlank()
                || exp == null) {
                log.debug("JWT rejected: missing required claim (hasSub={}, hasEmail={}, hasJti={}, hasExp={})",
                    sub != null && !sub.isBlank(),
                    email != null && !email.isBlank(),
                    jti != null && !jti.isBlank(),
                    exp != null);
                return Optional.empty();
            }

            Set<String> aud = claims.getAudience();
            if (aud == null || aud.size() != 1 || !aud.contains(AUDIENCE)) {
                log.debug("JWT rejected: invalid audience");
                return Optional.empty();
            }

            UUID userId = UUID.fromString(sub);
            return Optional.of(new TokenClaims(userId, email, jti, exp.toInstant()));
        } catch (JwtException | IllegalArgumentException ex) {
            log.debug("JWT rejected: {}", ex.getClass().getSimpleName());
            return Optional.empty();
        }
    }
}