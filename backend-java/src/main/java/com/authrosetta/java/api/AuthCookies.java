package com.authrosetta.java.api;

import org.springframework.http.ResponseCookie;

import java.util.Optional;

import org.springframework.http.HttpHeaders;

import jakarta.servlet.http.Cookie;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;

public final class AuthCookies {
    
    public static final String COOKIE_NAME = "access_token";
    public static final long MAX_AGE_SECONDS = 3600L;

    private AuthCookies() {}

    /**
     * Sets the access token cookie in the response.
     * @param request the HTTP servlet request
     * @param response the HTTP servlet response
     * @param token the access token to set in the cookie
     */
    public static void setAccessToken(HttpServletRequest request, HttpServletResponse response, String token) {
        ResponseCookie cookie = ResponseCookie.from(COOKIE_NAME, token)
            .httpOnly(true)
            .secure(request.isSecure())
            .sameSite("Strict")
            .path("/")
            .maxAge(MAX_AGE_SECONDS)
            .build();
            
        response.addHeader(HttpHeaders.SET_COOKIE, cookie.toString());
    }

    /**
     * Clears the access token cookie in the response.
     * @param request the HTTP servlet request
     * @param response the HTTP servlet response
     */
    public static void clearAccessToken(HttpServletRequest request, HttpServletResponse response) {
        ResponseCookie cookie = ResponseCookie.from(COOKIE_NAME, "")
            .httpOnly(true)
            .secure(request.isSecure())
            .sameSite("Strict")
            .path("/")
            .maxAge(0L)
            .build();
        response.addHeader(HttpHeaders.SET_COOKIE, cookie.toString());
    }

    /**
     * Reads the access token cookie from the request.
     * @param request the HTTP servlet request
     * @return an Optional containing the access token if present, otherwise an empty Optional
     */
    public static Optional<String> readAccessToken(HttpServletRequest request) {
        Cookie[] cookies = request.getCookies();
        if (cookies == null) {
            return Optional.empty();
        }
        for (Cookie cookie : cookies) {
            if (COOKIE_NAME.equals(cookie.getName())) {
                String value = cookie.getValue();
                if (value != null && !value.isEmpty()) {
                    return Optional.of(value);
                }
            }
        }
        return Optional.empty();
    }
}
