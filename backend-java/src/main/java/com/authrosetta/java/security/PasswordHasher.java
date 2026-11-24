package com.authrosetta.java.security;

import org.springframework.stereotype.Component;

import com.authrosetta.java.validation.ValidationRules;

import java.nio.charset.StandardCharsets;

import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;

@Component 
public class PasswordHasher {
    
    public static final int DEFAULT_COST = 12;

    private final int cost;
    private final BCryptPasswordEncoder encoder;
    private final String dummyHash;

    public PasswordHasher() {
        this(DEFAULT_COST);
    }

    public PasswordHasher(int cost) {
        this.cost = cost;
        this.encoder = new BCryptPasswordEncoder(cost);
        this.dummyHash = this.encoder.encode("dummy-password-for-timing-equalization");
    }

    public int cost() {
        return cost;
    }

    public String dummyHash() {
        return dummyHash;
    }

    /**
     * Hashes the given password using BCrypt with the configured cost.
     * @throws IllegalArgumentException if the password contains invalid characters or exceeds 72 UTF-8 bytes.
     * @param password the password to hash
     * @return the hashed password
     */
    public String hash(String password) {
        if (!ValidationRules.isStorable(password)) {
            throw new IllegalArgumentException("Password contains invalid characters.");
        }
        if (password.getBytes(StandardCharsets.UTF_8).length > ValidationRules.MAX_PASSWORD_BYTES) {
            throw new IllegalArgumentException("Password exceeds 72 UTF-8 bytes.");
        }
        return encoder.encode(password);    
    }

    /**
     * Verifies if a given password matches the stored hash. Returns false if the password is invalid or exceeds 72 UTF-8 bytes.
     * @param password the password to verify
     * @param storedHash the stored hash to compare against
     * @return true if the password matches the stored hash, false otherwise
     */
    public boolean verify(String password, String storedHash) {
        if (!ValidationRules.isStorable(password) || password.getBytes(StandardCharsets.UTF_8).length > ValidationRules.MAX_PASSWORD_BYTES) {
            verifyAgainstDummy();
            return false;
        }
        return encoder.matches(password, storedHash);
    }

    /**
     * Verifies a dummy password against the dummy hash to equalize timing for invalid password attempts.
     */
    public void verifyAgainstDummy() {
        encoder.matches("dummy-candidate-password-for-timing-equalization", dummyHash);
    }
}
