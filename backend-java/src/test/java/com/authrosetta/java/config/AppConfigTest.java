package com.authrosetta.java.config;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.Map;
import java.util.stream.Stream;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

class AppConfigTest {

    private static Map<String, String> validBaseEnv() {
        Map<String, String> env = new HashMap<>();
        env.put("DB_USER", "rosetta");
        env.put("DB_PASSWORD", "s3cr3t!@#");
        env.put("JWT_KEY", "a".repeat(32));
        return env;
    }    

    @Test 
    void parse_validMinimal_appliesDefaultsAndRedactsSecrets() {
        AppConfig cfg = AppConfig.parse(validBaseEnv()::get);

        assertEquals("localhost", cfg.dbHost());
        assertEquals(5432, cfg.dbPort());
        assertEquals("auth_java", cfg.dbName());
        assertEquals("http://localhost:5173", cfg.frontendOrigin());
        assertEquals("INFO", cfg.logLevel());
        assertEquals("jdbc:postgresql://localhost:5432/auth_java?user=rosetta&password=s3cr3t%21%40%23", cfg.jdbcUrl());

        String text = cfg.toString();
        assertFalse(text.contains("s3cr3t"));
        assertFalse(text.contains("aaaa"));
        assertTrue(text.contains("[REDACTED]"));
    }

    @Test
    void parse_reportsMultipleErrorsAtOnceWithoutLeakingSecretValues() {
        Map<String, String> env = Map.of(
            "DB_PORT", "not-a-port",
            "JWT_KEY", "short-secret-value",
            "FRONTEND_ORIGIN", "*",
            "LOG_LEVEL", "info"
        );

        IllegalStateException ex = assertThrows(IllegalStateException.class, () -> AppConfig.parse(env::get));
        String msg = ex.getMessage();

        assertTrue(msg.contains("DB_PORT"));
        assertTrue(msg.contains("DB_USER is required"));
        assertTrue(msg.contains("DB_PASSWORD is required"));
        assertTrue(msg.contains("JWT_KEY must be at least 32 bytes"));
        assertTrue(msg.contains("FRONTEND_ORIGIN"));
        assertTrue(msg.contains("LOG_LEVEL"));
        assertFalse(msg.contains("short-secret-value"));
    }

    @Test
    void load_envOverridesDotenvAndHandlesBomQuotesComments(@TempDir Path tempDir) throws Exception {
        Path dotenv = tempDir.resolve(".env");
        String content = "\uFEFF# Comment line\r\n"
            + "DB_USER=\"file_user\"\r\n"
            + "DB_PASSWORD='file_pass'\r\n"
            + "JWT_KEY=" + "x".repeat(32) + "\r\n"
            + "DB_NAME=from_file_db\r\n";
        Files.writeString(dotenv, content);

        Map<String, String> processEnv = Map.of(
            "DB_USER", "env_user",
            "DB_NAME", "   " // blank counts as unset -> falls back to file
        );

        AppConfig cfg = AppConfig.load(dotenv, processEnv::get);
        assertEquals("env_user", cfg.dbUser());
        assertEquals("file_pass", cfg.dbPassword());
        assertEquals("from_file_db", cfg.dbName());
    }

    @Test
    void load_missingDotenvIsAllowed() {
        AppConfig cfg = AppConfig.load(Path.of("nonexistent", ".env"), validBaseEnv()::get);
        assertEquals("rosetta", cfg.dbUser());
    }

    @ParameterizedTest(name = "{0} -> valid={1}")
    @MethodSource("originCases")
    void isValidOrigin_validatesStrictly(String origin, boolean expected) {
        assertEquals(expected, AppConfig.isValidOrigin(origin));
    }

    static Stream<Arguments> originCases() {
        return Stream.of(
            Arguments.of("http://localhost:5173", true),
            Arguments.of("https://auth.example.com", true),
            Arguments.of("*", false),
            Arguments.of("http://localhost:5173/", false),
            Arguments.of("http://localhost:5173/app", false),
            Arguments.of("ftp://localhost:5173", false),
            Arguments.of("localhost:5173", false),
            Arguments.of("${FRONTEND_ORIGIN:-http://localhost:5173}", false)
        );
    }
}
