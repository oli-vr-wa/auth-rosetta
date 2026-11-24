package com.authrosetta.java.security;

import org.junit.jupiter.api.Test;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;

import static org.junit.jupiter.api.Assertions.*;

class PasswordHasherTest {

    @Test
    void defaultCost_isTwelveForBothHashesAndDummy() {
        PasswordHasher hasher = new PasswordHasher();
        assertEquals(12, hasher.cost());
        assertTrue(hasher.dummyHash().startsWith("$2a$12$"));

        String hashed = hasher.hash("correcthorse");
        assertTrue(hashed.startsWith("$2a$12$"));
        assertTrue(hasher.verify("correcthorse", hashed));
        assertFalse(hasher.verify("wrongpassword", hashed));
    }

    @Test
    void fastCost_enforces72ByteCapAndRejectsNulWithoutTruncating() {
        PasswordHasher hasher = new PasswordHasher(4); // min bcrypt cost for fast tests
        String exact72 = "a".repeat(72);
        String over72 = "a".repeat(72) + "b";

        String hash = hasher.hash(exact72);
        assertTrue(hasher.verify(exact72, hash));
        assertFalse(hasher.verify(over72, hash));
        assertThrows(IllegalArgumentException.class, () -> hasher.hash(over72));
        assertThrows(IllegalArgumentException.class, () -> hasher.hash("pass\u0000word"));
        assertFalse(hasher.verify("pass\u0000word", hash));
    }

    @Test
    void rawSpringBcrypt_doesNotTruncateAtNulByte() {
        BCryptPasswordEncoder raw = new BCryptPasswordEncoder(4);
        String hash = raw.encode("abc\u0000def");
        assertTrue(raw.matches("abc\u0000def", hash));
        assertFalse(raw.matches("abc\u0000xyz", hash));
        assertFalse(raw.matches("abc", hash));
    }
}