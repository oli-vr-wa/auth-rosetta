package com.authrosetta.java.security;

import java.net.URLEncoder;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.Arrays;
import java.util.Locale;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

import org.springframework.stereotype.Component;

@Component 
public class TotpService {
   
    public static final String ISSUER = "AuthRosetta-Java";
    public static final int SECRET_BYTES = 20;
    public static final int DIGITS = 6;
    public static final long PERIOD_SECONDS = 30L;
    public static final int SKEW_STEPS = 1;    

    private static final String BASE32_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
    private static final SecureRandom SECURE_RANDOM = new SecureRandom();
    private static final String HMAC_ALGORITHM = "HmacSHA1";

    public record Enrollment(String secret, String otpauthUri) {}

    public Enrollment createEnrollment(String normalizedEmail) {
        byte[] rawSecret = new byte[SECRET_BYTES];
        SECURE_RANDOM.nextBytes(rawSecret);
        String secret = encodeBase32(rawSecret);
        String encodedAccount = urlEncodeString(normalizedEmail);
        String encodedIssuer = urlEncodeString(ISSUER);
        String uri = String.format(
            "otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
            encodedIssuer,
            encodedAccount,
            secret,
            encodedIssuer,
            DIGITS,
            PERIOD_SECONDS
        );
        return new Enrollment(secret, uri);
    }

    /**
     * Verifies a TOTP code against the given base32-encoded secret at the specified time.
     * @param base32Secret the base32-encoded secret key
     * @param code the TOTP code to verify
     * @param at the time at which to verify the code
     * @return true if the code is valid, false otherwise
     */
    public boolean verify(String base32Secret, String code, Instant at) {
        if (base32Secret == null || !isSixAsciiDigits(code)) {
            return false;
        }
        byte[] key;
        try {
            key = decodeBase32(base32Secret);
        } catch (IllegalArgumentException e) {
            return false;
        }
        if (key.length == 0) {
            return false;
        }

        byte[] candidateBytes = code.getBytes(StandardCharsets.US_ASCII);
        long currentStep = Math.floorDiv(at.getEpochSecond(), PERIOD_SECONDS);
        boolean matched = false;
        for (long offset = -SKEW_STEPS; offset <= SKEW_STEPS; offset++) {
            String expected = generateCodeForStep(key, currentStep + offset);
            if (MessageDigest.isEqual(candidateBytes, expected.getBytes(StandardCharsets.US_ASCII))) {
                matched = true;
            }
        }
        return matched;
    }

    /**
     * Generates a TOTP code for the given key and time step.
     * @param key the secret key as a byte array
     * @param step the time step for which to generate the code
     * @return the generated TOTP code as a string
     */
    static String generateCodeForStep(byte[] key, long step) {
        byte[] counter = ByteBuffer.allocate(8).putLong(step).array();
        try {
            Mac mac = Mac.getInstance(HMAC_ALGORITHM);
            mac.init(new SecretKeySpec(key, HMAC_ALGORITHM));
            byte[] hash = mac.doFinal(counter);
            int offset = hash[hash.length - 1] & 0x0F;
            int binary = ((hash[offset] & 0x7F) << 24)
                | ((hash[offset + 1] & 0xFF) << 16)
                | ((hash[offset + 2] & 0xFF) << 8)
                | (hash[offset + 3] & 0xFF);
            int otp = binary % 1_000_000;
            return String.format(Locale.ROOT, "%06d", otp);
        } catch (GeneralSecurityException e) {
            throw new IllegalStateException("HmacSHA1 unavailable", e);
        }
    }

    /**
     * Method to check if the given code consists of exactly six ASCII digits.
     * @param code the code to check
     * @return true if the code consists of exactly six ASCII digits, false otherwise
     */
    static boolean isSixAsciiDigits(String code) {
        if (code == null || code.length() != DIGITS) {
            return false;
        }
        for (int i = 0; i < DIGITS; i++) {
            char c = code.charAt(i);
            if (c < '0' || c > '9') {
                return false;
            }
        }
        return true;
    }

    /**
     * Encodes the given byte array into a Base32 string.
     * @param data the byte array to encode
     * @return the Base32 encoded string
     */
    public String encodeBase32(byte[] data) {
        StringBuilder out = new StringBuilder((data.length * 8 + 4) / 5);
        int buffer = 0;
        int bitsLeft = 0;
        for (byte b : data) {
            buffer = (buffer << 8) | (b & 0xFF);
            bitsLeft += 8;
            while (bitsLeft >= 5) {
                int index = (buffer >> (bitsLeft - 5)) & 0x1F;
                bitsLeft -= 5;
                out.append(BASE32_ALPHABET.charAt(index));                
            }
        }
        if (bitsLeft > 0) {
            int index = (buffer << (5 - bitsLeft)) & 0x1F;
            out.append(BASE32_ALPHABET.charAt(index));
        }
        return out.toString();
    }

    /**
     * Decodes the given Base32 string into a byte array.
     * @param base32 the Base32 encoded string
     * @return the decoded byte array
     */
    static byte[] decodeBase32(String base32) {
        String clean = base32.trim().toUpperCase(Locale.ROOT);
        while (clean.endsWith("=")) {
            clean = clean.substring(0, clean.length() - 1);
        }
        if (clean.isEmpty()) {
            return new byte[0];
        }
        byte[] out = new byte[(clean.length() * 5) / 8];
        int buffer = 0;
        int bitsLeft = 0;
        int count = 0;
        for (int i = 0; i < clean.length(); i++) {
            int val = BASE32_ALPHABET.indexOf(clean.charAt(i));
            if (val < 0) {
                throw new IllegalArgumentException("Invalid Base32 character");
            }
            buffer = (buffer << 5) | val;
            bitsLeft += 5;
            if (bitsLeft >= 8) {
                out[count++] = (byte) ((buffer >> (bitsLeft - 8)) & 0xFF);
                bitsLeft -= 8;
            }
        }
        return Arrays.copyOf(out, count);
    }

    /**
     * Encodes the given string as a URL component using UTF-8 encoding. Spaces are encoded as %20.
     * @param value the string to encode
     * @return the URL-encoded string as a path segment
     */
    private static String urlEncodeString(String value) {
        return URLEncoder.encode(value, StandardCharsets.UTF_8).replace("+", "%20");
    }
}
