package com.authrosetta.java.validation;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.stream.Stream;
import org.junit.jupiter.params.provider.Arguments;

import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;

class ValidationRulesTest {

    @ParameterizedTest(name = "{0}")
    @MethodSource("storableCases")
    void isStorable_measuresStringsAccurately(String label, String input, boolean expected) {
        assertEquals(expected, ValidationRules.isStorable(input));
    }
    
    static Stream<Arguments> storableCases() {
        return Stream.of(
            Arguments.of("plain ascii", "alice@example.com", true),
            Arguments.of("traditional chinese", "密碼測試1234", true),
            Arguments.of("valid surrogate pair (emoji)", "pass\uD83D\uDD111234", true),
            Arguments.of("null reference", null, false),
            Arguments.of("embedded NUL", "pass\0word", false),
            Arguments.of("lone high surrogate", "pass\uD800word", false),
            Arguments.of("trailing high surrogate", "password\uD800", false),
            Arguments.of("lone low surrogate", "pass\uDC00word", false)
        );
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("registerCases")
    void validateRegister_enforcesContractRules(String label, String email, String password, boolean expectedValid, String expectedSubstring) {
        if (expectedValid) {
            assertDoesNotThrow(() -> ValidationRules.validateRegister(email, password));
        } else {
            ValidationException ex = assertThrows(
                ValidationException.class, 
                () -> ValidationRules.validateRegister(email, password)
            );
            assertTrue(
                ex.getMessage().contains(expectedSubstring),
                "Expected '" + ex.getMessage() + "' to contain '" + expectedSubstring + "'"
            );
        }
    }

    static Stream<Arguments> registerCases() {
        String exact256Email = "a".repeat(244) + "@example.com"; // 244 + 12 = 256
        String tooLong257Email = "a".repeat(245) + "@example.com"; // 257
        String exact72BytesUtf8 = "é".repeat(36); // 36 * 2 bytes = 72 bytes, 36 chars
        String tooLong73BytesUtf8 = "é".repeat(36) + "a"; // 73 bytes

        return Stream.of(
            Arguments.of("valid standard", "Alice@Example.COM ", "correcthorse", true, ""),
            Arguments.of("exact 256 char email", exact256Email, "correcthorse", true, ""),
            Arguments.of("257 char email rejected", tooLong257Email, "correcthorse", false, "256"),
            Arguments.of("blank email rejected", "   ", "correcthorse", false, "required"),
            Arguments.of("missing @ rejected", "alice.example.com", "correcthorse", false, "valid email"),
            Arguments.of("exact 8 codepoints (surrogate pairs)", "\uD83D\uDD11".repeat(8), "a@b.co", false, "valid email"),
            Arguments.of("7 chars rejected", "a@b.co", "1234567", false, "8 characters"),
            Arguments.of("exact 72 UTF-8 bytes accepted", "a@b.co", exact72BytesUtf8, true, ""),
            Arguments.of("73 UTF-8 bytes rejected", "a@b.co", tooLong73BytesUtf8, false, "72 bytes"),
            Arguments.of("NUL in password rejected", "a@b.co", "1234\u00005678", false, "invalid characters"),
            Arguments.of("lone surrogate in email rejected", "a\uD800@b.co", "correcthorse", false, "invalid characters"),
            Arguments.of("multiple errors joined with semicolon", "", "short", false, "; ")
        );
    }
}
