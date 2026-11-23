package com.authrosetta.java.validation;

import java.util.Locale;
import java.util.List;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;

public final class ValidationRules {
    
    public static final int MAX_EMAIL_LENGTH = 256;
    public static final int MIN_PASSWORD_LENGTH = 8;
    public static final int MAX_PASSWORD_BYTES = 72;

    private ValidationRules() {}

    /**
     * Normalize an email address by trimming whitespace and converting to lowercase.
     * @param email The email address to normalize.
     * @return The normalized email address.
     */
    public static String normalizeEmail(String email) {
        return email.trim().toLowerCase(Locale.ROOT);
    }

    /**
     * Checks if a string is storable, meaning it does not contain null characters or invalid surrogate pairs.
     * @param value The string to check.
     * @return True if the string is storable, false otherwise.
     */
    public static boolean isStorable(String value) {
        if (value == null) {
            return false;
        }
        int len = value.length();
        for (int i = 0; i < len; i++) {
            char c = value.charAt(i);
            if (c == '\0') {
                return false;
            }
            if (Character.isHighSurrogate(c)) {
                if (i + 1 >= len || !Character.isLowSurrogate(value.charAt(i + 1))) {
                    return false;
                }
                i++; // valid surrogate pair, skip the low surrogate
            } else if (Character.isLowSurrogate(c)) {
                return false;
            }
        }
        return true;
    }

    /**
     * Validates the email and password for registration.
     * Throws a ValidationException if any validation rules are violated.
     * @param email The email address to validate.
     * @param password The password to validate.
     */
    public static void validateRegister(String email, String password) {
        List<String> errors = new ArrayList<>();

        if (!isStorable(email)) {
            errors.add("Email contains invalid characters.");
        } else {
            String normalizedEmail = normalizeEmail(email);
            if (normalizedEmail.isEmpty()) {
                errors.add("Email is required.");
            } else if (normalizedEmail.length() > MAX_EMAIL_LENGTH) {
                errors.add("Email must be at most " + MAX_EMAIL_LENGTH + " characters.");
            } else if (!looksLikeEmail(normalizedEmail)) {
                errors.add("Email must be a valid email address.");
            }
        }

        if (!isStorable(password)) {
            errors.add("Password contains invalid characters.");
        } else {
            int charCount = password.codePointCount(0, password.length());
            int byteCount = password.getBytes(StandardCharsets.UTF_8).length;
            if (charCount < MIN_PASSWORD_LENGTH) {
                errors.add("Password must be at least " + MIN_PASSWORD_LENGTH + " characters.");
            } else if (byteCount > MAX_PASSWORD_BYTES) {
                errors.add("Password must be at most " + MAX_PASSWORD_BYTES + " bytes.");
            }
        }

        if (!errors.isEmpty()) {
            throw new ValidationException(String.join("; ", errors));
        }
    }

    /**
     * Checks if the normalized email looks like a valid email address.
     * @param normalizedEmail The normalized email address to check.
     * @return True if the email looks like a valid email address, false otherwise.
     */
    private static boolean looksLikeEmail(String normalizedEmail) {
        int at = normalizedEmail.indexOf('@');
        if (at <= 0 || at != normalizedEmail.lastIndexOf('@')) {
            return false;
        }
        String domain = normalizedEmail.substring(at + 1);
        int dot = domain.indexOf('.');
        return dot > 0 && dot < domain.length() - 1 && !normalizedEmail.contains(" ");
    }
}
