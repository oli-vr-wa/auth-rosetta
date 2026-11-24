package com.authrosetta.java.security;

import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.Keys;
import org.junit.jupiter.api.Test;
import tools.jackson.core.type.TypeReference;
import tools.jackson.databind.ObjectMapper;

import javax.crypto.SecretKey;
import java.nio.charset.StandardCharsets;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Base64;
import java.util.Date;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;

class JwtServiceTest {

    private static final byte[] KEY = "0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.UTF_8);
    private static final byte[] OTHER_KEY = "fedcba9876543210fedcba9876543210".getBytes(StandardCharsets.UTF_8);
    private static final Instant BASE_TIME = Instant.parse("2026-11-23T12:00:00Z");

    private static Clock fixedClock(Instant at) {
        return Clock.fixed(at, ZoneOffset.UTC);
    }

    @Test
    void issueAndVerify_roundTripsClaimsAndProducesExactPayloadShape() throws Exception {
        JwtService jwt = new JwtService(KEY, fixedClock(BASE_TIME));
        UUID userId = UUID.randomUUID();

        JwtService.IssuedToken issued = jwt.issue(userId, "alice@example.com");
        assertNotNull(issued.token());

        // Inspect raw JSON payload to confirm exact claim names and types
        String[] parts = issued.token().split("\\.");
        assertEquals(3, parts.length);
        byte[] payloadBytes = Base64.getUrlDecoder().decode(parts[1]);
        Map<String, Object> rawPayload = new ObjectMapper().readValue(payloadBytes, new TypeReference<>() {});
        assertEquals(Set.of("iss", "aud", "sub", "email", "jti", "exp"), rawPayload.keySet());
        assertEquals("AuthRosetta", rawPayload.get("iss"));
        assertEquals("AuthRosetta", rawPayload.get("aud"));

        JwtService.TokenClaims verified = jwt.verify(issued.token()).orElseThrow();
        assertEquals(userId, verified.userId());
        assertEquals("alice@example.com", verified.email());
        assertEquals(issued.claims().jti(), verified.jti());
        assertEquals(BASE_TIME.plus(Duration.ofMinutes(60)), verified.expiresAt());
    }

    @Test
    void verify_enforces60MinExpiryAnd30SecondClockSkew() {
        JwtService issuer = new JwtService(KEY, fixedClock(BASE_TIME));
        String token = issuer.issue(UUID.randomUUID(), "alice@example.com").token();

        // At 60m + 29s: within 30s skew -> accepted
        JwtService withinSkew = new JwtService(KEY, fixedClock(BASE_TIME.plus(Duration.ofMinutes(60)).plusSeconds(29)));
        assertTrue(withinSkew.verify(token).isPresent());

        // At 60m + 31s: past 30s skew -> rejected
        JwtService pastSkew = new JwtService(KEY, fixedClock(BASE_TIME.plus(Duration.ofMinutes(60)).plusSeconds(31)));
        assertTrue(pastSkew.verify(token).isEmpty());
    }

    @Test
    void verify_rejectsWrongKeyWrongIssuerWrongAudienceAndUnsigned() {
        SecretKey secretKey = Keys.hmacShaKeyFor(KEY);
        Date exp = Date.from(BASE_TIME.plus(Duration.ofMinutes(30)));
        JwtService verifier = new JwtService(KEY, fixedClock(BASE_TIME));

        // Wrong signing key
        String wrongKeyToken = new JwtService(OTHER_KEY, fixedClock(BASE_TIME))
            .issue(UUID.randomUUID(), "a@b.com").token();
        assertEquals(Optional.empty(), verifier.verify(wrongKeyToken));

        // Wrong issuer (e.g., AuthRosetta-Java instead of AuthRosetta)
        String wrongIss = Jwts.builder()
            .issuer("AuthRosetta-Java")
            .audience().add("AuthRosetta").and()
            .subject(UUID.randomUUID().toString())
            .claim("email", "a@b.com")
            .id(UUID.randomUUID().toString())
            .expiration(exp)
            .signWith(secretKey, Jwts.SIG.HS256)
            .compact();
        assertEquals(Optional.empty(), verifier.verify(wrongIss));

        // Wrong audience
        String wrongAud = Jwts.builder()
            .issuer("AuthRosetta")
            .audience().add("OtherApp").and()
            .subject(UUID.randomUUID().toString())
            .claim("email", "a@b.com")
            .id(UUID.randomUUID().toString())
            .expiration(exp)
            .signWith(secretKey, Jwts.SIG.HS256)
            .compact();
        assertEquals(Optional.empty(), verifier.verify(wrongAud));

        // Unsigned (alg=none)
        String unsigned = Jwts.builder()
            .issuer("AuthRosetta")
            .audience().add("AuthRosetta").and()
            .subject(UUID.randomUUID().toString())
            .claim("email", "a@b.com")
            .id(UUID.randomUUID().toString())
            .expiration(exp)
            .compact();
        assertEquals(Optional.empty(), verifier.verify(unsigned));
    }

    @Test
    void verify_rejectsTokensMissingExpSubEmailOrJti() {
        SecretKey secretKey = Keys.hmacShaKeyFor(KEY);
        Date exp = Date.from(BASE_TIME.plus(Duration.ofMinutes(30)));
        JwtService verifier = new JwtService(KEY, fixedClock(BASE_TIME));

        // Missing exp
        String noExp = Jwts.builder()
            .issuer("AuthRosetta").audience().add("AuthRosetta").and()
            .subject(UUID.randomUUID().toString()).claim("email", "a@b.com").id(UUID.randomUUID().toString())
            .signWith(secretKey, Jwts.SIG.HS256).compact();
        assertEquals(Optional.empty(), verifier.verify(noExp));

        // Missing sub
        String noSub = Jwts.builder()
            .issuer("AuthRosetta").audience().add("AuthRosetta").and()
            .claim("email", "a@b.com").id(UUID.randomUUID().toString()).expiration(exp)
            .signWith(secretKey, Jwts.SIG.HS256).compact();
        assertEquals(Optional.empty(), verifier.verify(noSub));

        // Missing email
        String noEmail = Jwts.builder()
            .issuer("AuthRosetta").audience().add("AuthRosetta").and()
            .subject(UUID.randomUUID().toString()).id(UUID.randomUUID().toString()).expiration(exp)
            .signWith(secretKey, Jwts.SIG.HS256).compact();
        assertEquals(Optional.empty(), verifier.verify(noEmail));

        // Missing jti
        String noJti = Jwts.builder()
            .issuer("AuthRosetta").audience().add("AuthRosetta").and()
            .subject(UUID.randomUUID().toString()).claim("email", "a@b.com").expiration(exp)
            .signWith(secretKey, Jwts.SIG.HS256).compact();
        assertEquals(Optional.empty(), verifier.verify(noJti));
    }
}