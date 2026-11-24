package com.authrosetta.java;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.jdbc.core.simple.JdbcClient;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.json.JsonMapper;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.net.CookieManager;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.ByteBuffer;
import java.time.Instant;
import java.util.Locale;

import static org.junit.jupiter.api.Assertions.*;

@Tag("integration")
@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class EndToEndAuthIntegrationTest {

    @LocalServerPort
    private int port;

    @Autowired
    private JdbcClient jdbc;

    private final JsonMapper json = JsonMapper.builder().build();
    private HttpClient client;

    @BeforeEach
    void setUp() {
        jdbc.sql("TRUNCATE TABLE revoked_tokens, users").update();
        client = HttpClient.newBuilder()
            .cookieHandler(new CookieManager())
            .build();
    }

    @Test
    void fullLifecycle_overRealHttpAndPostgres() throws Exception {
        // 1. Health check
        HttpResponse<String> health = get("/health");
        assertEquals(200, health.statusCode());
        assertEquals("ok", json.readTree(health.body()).get("status").asString());

        // 2. Register user -> 201 with otpauthUri and secret
        HttpResponse<String> regRes = postJson(
            "/api/auth/register",
            "{\"email\":\"  Alice@Example.com \",\"password\":\"correcthorse\"}"
        );
        assertEquals(201, regRes.statusCode());
        JsonNode regJson = json.readTree(regRes.body());
        String secret = regJson.get("secret").asString();
        String otpauthUri = regJson.get("otpauthUri").asString();
        assertFalse(secret.isBlank());
        assertTrue(otpauthUri.startsWith("otpauth://totp/AuthRosetta-Java:"));

        // Duplicate registration (case-insensitive) -> 409
        HttpResponse<String> dupRes = postJson(
            "/api/auth/register",
            "{\"email\":\"alice@example.com\",\"password\":\"otherpassword\"}"
        );
        assertEquals(409, dupRes.statusCode());
        assertEquals("Email already registered", json.readTree(dupRes.body()).get("error").asString());

        // 3. Login before 2FA confirmation -> 401
        String code = currentTotpCode(secret, Instant.now());
        HttpResponse<String> unconfirmedLogin = postJson(
            "/api/auth/login",
            "{\"email\":\"alice@example.com\",\"password\":\"correcthorse\",\"code\":\"" + code + "\"}"
        );
        assertEquals(401, unconfirmedLogin.statusCode());
        assertEquals("Invalid credentials", json.readTree(unconfirmedLogin.body()).get("error").asString());

        // 4. Confirm 2FA -> 200
        HttpResponse<String> confirmRes = postJson(
            "/api/auth/2fa/confirm",
            "{\"email\":\"alice@example.com\",\"code\":\"" + code + "\"}"
        );
        assertEquals(200, confirmRes.statusCode());
        assertTrue(json.readTree(confirmRes.body()).get("ok").asBoolean());

        // 5. Login -> 200 + HttpOnly SameSite=Strict access_token cookie
        HttpResponse<String> loginRes = postJson(
            "/api/auth/login",
            "{\"email\":\"alice@example.com\",\"password\":\"correcthorse\",\"code\":\"" + code + "\"}"
        );
        assertEquals(200, loginRes.statusCode());
        assertTrue(json.readTree(loginRes.body()).get("ok").asBoolean());
        String setCookie = loginRes.headers().firstValue("Set-Cookie").orElse("");
        assertTrue(setCookie.contains("access_token="));
        assertTrue(setCookie.contains("HttpOnly"));
        assertTrue(setCookie.contains("SameSite=Strict"));

        // 6. GET /api/auth/me with cookie -> 200
        HttpResponse<String> meRes = get("/api/auth/me");
        assertEquals(200, meRes.statusCode());
        JsonNode meJson = json.readTree(meRes.body());
        assertEquals("alice@example.com", meJson.get("email").asString());
        assertFalse(meJson.get("id").asString().isBlank());

        // 7. POST /api/auth/logout -> 200 + revokes jti in Postgres
        HttpResponse<String> logoutRes = postJson("/api/auth/logout", "{}");
        assertEquals(200, logoutRes.statusCode());
        assertTrue(json.readTree(logoutRes.body()).get("ok").asBoolean());

        Long revokedCount = jdbc.sql("SELECT COUNT(*) FROM revoked_tokens")
            .query(Long.class)
            .single();
        assertEquals(1L, revokedCount);

        // 8. Subsequent /api/auth/me -> 401
        HttpResponse<String> meAfterLogout = get("/api/auth/me");
        assertEquals(401, meAfterLogout.statusCode());
        assertEquals("Unauthorized", json.readTree(meAfterLogout.body()).get("error").asString());
    }

    private HttpResponse<String> get(String path) throws Exception {
        HttpRequest req = HttpRequest.newBuilder()
            .uri(URI.create("http://localhost:" + port + path))
            .header("Origin", "http://localhost:5173")
            .GET()
            .build();
        return client.send(req, HttpResponse.BodyHandlers.ofString());
    }

    private HttpResponse<String> postJson(String path, String body) throws Exception {
        HttpRequest req = HttpRequest.newBuilder()
            .uri(URI.create("http://localhost:" + port + path))
            .header("Origin", "http://localhost:5173")
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(body))
            .build();
        return client.send(req, HttpResponse.BodyHandlers.ofString());
    }

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
}