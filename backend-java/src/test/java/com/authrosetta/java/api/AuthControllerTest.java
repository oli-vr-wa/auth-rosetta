package com.authrosetta.java.api;

import com.authrosetta.java.auth.AuthError;
import com.authrosetta.java.auth.AuthException;
import com.authrosetta.java.auth.AuthService;
import com.authrosetta.java.validation.ValidationException;
import jakarta.servlet.http.Cookie;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.context.annotation.Import;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

import java.time.Instant;
import java.util.UUID;

import static org.hamcrest.Matchers.containsString;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@WebMvcTest(controllers = {AuthController.class, ApiExceptionHandler.class})
@Import({HttpContractFilter.class, StrictJsonReader.class, HttpContractTest.TestConfig.class})
class AuthControllerTest {

    @Autowired
    private MockMvc mockMvc;

    @MockitoBean
    private AuthService authService;

    @Test
    void register_returns201WithOtpauthUriAndSecret_andMapsErrors() throws Exception {
        when(authService.register("alice@example.com", "correcthorse"))
            .thenReturn(new AuthService.RegisterResult("otpauth://totp/AuthRosetta-Java:alice%40example.com?secret=ABC", "ABC"));

        mockMvc.perform(post("/api/auth/register")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"alice@example.com\",\"password\":\"correcthorse\"}"))
            .andExpect(status().isCreated())
            .andExpect(jsonPath("$.otpauthUri").value("otpauth://totp/AuthRosetta-Java:alice%40example.com?secret=ABC"))
            .andExpect(jsonPath("$.secret").value("ABC"));

        when(authService.register("taken@example.com", "correcthorse"))
            .thenThrow(new AuthException(AuthError.EMAIL_TAKEN));

        mockMvc.perform(post("/api/auth/register")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"taken@example.com\",\"password\":\"correcthorse\"}"))
            .andExpect(status().isConflict())
            .andExpect(jsonPath("$.error").value("Email already registered"));

        when(authService.register("bad", "short"))
            .thenThrow(new ValidationException("Invalid email address"));

        mockMvc.perform(post("/api/auth/register")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"bad\",\"password\":\"short\"}"))
            .andExpect(status().isBadRequest())
            .andExpect(jsonPath("$.error").value("Invalid email address"));
    }

    @Test
    void strictJson_rejectsWrongTypesMissingFieldsArraysTrailingTokensAndDuplicates() throws Exception {
        String[] badPayloads = {
            "[]",
            "null",
            "{\"email\":123,\"password\":\"correcthorse\"}",
            "{\"email\":\"a@b.com\",\"password\":null}",
            "{\"email\":\"a@b.com\"}",
            "{\"email\":\"a@b.com\",\"email\":\"b@b.com\",\"password\":\"correcthorse\"}",
            "{\"email\":\"a@b.com\",\"password\":\"correcthorse\"}{\"extra\":1}"
        };

        for (String payload : badPayloads) {
            mockMvc.perform(post("/api/auth/register")
                    .contentType(MediaType.APPLICATION_JSON)
                    .content(payload))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error").value("Invalid request"));
        }

        // Wrong Content-Type rejected
        mockMvc.perform(post("/api/auth/register")
                .contentType(MediaType.TEXT_PLAIN)
                .content("{\"email\":\"a@b.com\",\"password\":\"correcthorse\"}"))
            .andExpect(status().isBadRequest())
            .andExpect(jsonPath("$.error").value("Invalid request"));

        verifyNoInteractions(authService);
    }

    @Test
    void confirmTwoFactor_returnsOkOrInvalidCode() throws Exception {
        mockMvc.perform(post("/api/auth/2fa/confirm")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"alice@example.com\",\"code\":\"123456\"}"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.ok").value(true));

        doThrow(new AuthException(AuthError.INVALID_CODE))
            .when(authService).confirmTwoFactor("alice@example.com", "000000");

        mockMvc.perform(post("/api/auth/2fa/confirm")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"alice@example.com\",\"code\":\"000000\"}"))
            .andExpect(status().isBadRequest())
            .andExpect(jsonPath("$.error").value("Invalid email or code"));
    }

    @Test
    void login_setsHttpOnlyStrictCookieAndReturnsOk() throws Exception {
        when(authService.login("alice@example.com", "correcthorse", "123456"))
            .thenReturn(new AuthService.LoginResult("signed.jwt.token", Instant.parse("2026-11-24T04:00:00Z")));

        mockMvc.perform(post("/api/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"alice@example.com\",\"password\":\"correcthorse\",\"code\":\"123456\"}"))
            .andExpect(status().isOk())
            .andExpect(header().string(HttpHeaders.SET_COOKIE, containsString("access_token=signed.jwt.token")))
            .andExpect(header().string(HttpHeaders.SET_COOKIE, containsString("HttpOnly")))
            .andExpect(header().string(HttpHeaders.SET_COOKIE, containsString("SameSite=Strict")))
            .andExpect(header().string(HttpHeaders.SET_COOKIE, containsString("Max-Age=3600")))
            .andExpect(jsonPath("$.ok").value(true));
    }

    @Test
    void meAndLogout_requireValidCookieAndClearCookieOnLogout() throws Exception {
        // Missing cookie -> 401
        mockMvc.perform(get("/api/auth/me"))
            .andExpect(status().isUnauthorized())
            .andExpect(jsonPath("$.error").value("Unauthorized"));

        mockMvc.perform(post("/api/auth/logout"))
            .andExpect(status().isUnauthorized())
            .andExpect(jsonPath("$.error").value("Unauthorized"));

        UUID userId = UUID.randomUUID();
        when(authService.authenticate("valid.jwt"))
            .thenReturn(new AuthService.AuthenticatedUser(userId, "alice@example.com", "jti-1", Instant.now()));

        mockMvc.perform(get("/api/auth/me")
                .cookie(new Cookie("access_token", "valid.jwt")))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.id").value(userId.toString()))
            .andExpect(jsonPath("$.email").value("alice@example.com"));

        mockMvc.perform(post("/api/auth/logout")
                .cookie(new Cookie("access_token", "valid.jwt")))
            .andExpect(status().isOk())
            .andExpect(header().string(HttpHeaders.SET_COOKIE, containsString("Max-Age=0")))
            .andExpect(jsonPath("$.ok").value(true));

        verify(authService).logout("valid.jwt");
    }
}