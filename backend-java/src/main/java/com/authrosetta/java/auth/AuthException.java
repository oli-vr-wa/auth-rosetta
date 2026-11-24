package com.authrosetta.java.auth;

public class AuthException extends RuntimeException {
    
    private final AuthError error;

    public AuthException(AuthError error) {
        super(error.name());
        this.error = error;
    }

    public AuthError error() {
        return error;
    }
}
