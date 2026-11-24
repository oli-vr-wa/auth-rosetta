package com.authrosetta.java.security;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

import java.time.Instant;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;

class TotpServiceTest {

    // RFC 6238 Appendix B SHA-1 test seed ("12345678901234567890" in ASCII -> Base32)
    private static final String RFC_SECRET = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ";

    private final TotpService totp = new TotpService();

    @ParameterizedTest(name = "epoch={0} -> expected={1}")
    @MethodSource("rfc6238Sha1Vectors")
    void verify_matchesRfc6238TestVectors(long epochSeconds, String expectedCode) {
        Instant at = Instant.ofEpochSecond(epochSeconds);
        assertTrue(totp.verify(RFC_SECRET, expectedCode, at));
        assertFalse(totp.verify(RFC_SECRET, "000000", at));
    }

    static Stream<Arguments> rfc6238Sha1Vectors() {
        return Stream.of(
            Arguments.of(59L, "287082"),
            Arguments.of(1111111109L, "081804"),
            Arguments.of(1234567890L, "005924")
        );
    }

    @Test
    void verify_honorsPlusMinusOneStepSkewAndRejectsTwoSteps() {
        // At epoch 59 (step 1), valid code is 287082
        assertTrue(totp.verify(RFC_SECRET, "287082", Instant.ofEpochSecond(30)));  // step 1
        assertTrue(totp.verify(RFC_SECRET, "287082", Instant.ofEpochSecond(60)));  // step 2 (-1 skew -> step 1)
        assertTrue(totp.verify(RFC_SECRET, "287082", Instant.ofEpochSecond(15)));  // step 0 (+1 skew -> step 1)
        assertFalse(totp.verify(RFC_SECRET, "287082", Instant.ofEpochSecond(90))); // step 3 (2 steps away)
    }

    @ParameterizedTest(name = "invalid code: {0}")
    @MethodSource("invalidCodes")
    void verify_rejectsMalformedOrNonAsciiDigits(String badCode) {
        assertFalse(totp.verify(RFC_SECRET, badCode, Instant.ofEpochSecond(59)));
    }

    static Stream<String> invalidCodes() {
        return Stream.of(
            null,
            "",
            "28708",
            "2870820",
            "28708a",
            "٢٨٧٠٨٢" // Arabic-Indic digits (Character.isDigit is true, must be rejected)
        );
    }

    @Test
    void createEnrollment_usesJavaIssuerAnd20ByteBase32Secret() {
        TotpService.Enrollment enrollment = totp.createEnrollment("alice@example.com");
        assertEquals(32, enrollment.secret().length()); // 20 bytes * 8 / 5 = 32 Base32 chars
        assertEquals(20, TotpService.decodeBase32(enrollment.secret()).length);
        assertTrue(enrollment.otpauthUri().startsWith("otpauth://totp/AuthRosetta-Java:alice%40example.com?"));
        assertTrue(enrollment.otpauthUri().contains("secret=" + enrollment.secret()));
        assertTrue(enrollment.otpauthUri().contains("issuer=AuthRosetta-Java"));
    }
}