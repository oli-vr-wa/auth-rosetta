package com.authrosetta.java.api;

import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import com.authrosetta.java.auth.AuthError;
import com.authrosetta.java.auth.AuthException;
import com.authrosetta.java.auth.AuthService;

import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;

@RestController 
@RequestMapping(path = "api/auth", produces = MediaType.APPLICATION_JSON_VALUE)
public class AuthController {

    private final AuthService authService;
    private final StrictJsonReader jsonReader;

    public record RegisterResponse(String otpauthUri, String secret) {}
    public record OkResponse(boolean ok) {}
    public record OkResponseMessage(String message) {}
    public record MeResponse(String email) {}

    public AuthController(AuthService authService, StrictJsonReader jsonReader) {
        this.authService = authService;
        this.jsonReader = jsonReader;
    }

    @PostMapping("/register")
    public ResponseEntity<RegisterResponse> register(HttpServletRequest request) {
        StrictJsonReader.CredentialsPayload payload = jsonReader.readCredentials(request);
        AuthService.RegisterResult result = authService.register(payload.email(), payload.password());

        return ResponseEntity.status(HttpStatus.CREATED)
            .body(new RegisterResponse(result.otpauthUri(), result.secret()));
    }

    @PostMapping("/2fa/confirm")
    public ResponseEntity<OkResponse> confirmTwoFactor(HttpServletRequest request) {
        StrictJsonReader.ConfirmTwoFactorPayload payload = jsonReader.readConfirmTwoFactor(request);
        authService.confirmTwoFactor(payload.email(), payload.code());
        return ResponseEntity.ok(new OkResponse(true));
    }

    @PostMapping("/login")
    public ResponseEntity<OkResponseMessage> login(HttpServletRequest request, HttpServletResponse response) {
        StrictJsonReader.LoginPayload payload = jsonReader.readLogin(request);
        AuthService.LoginResult result = authService.login(payload.email(), payload.password(), payload.code());
        AuthCookies.setAccessToken(request, response, result.token());
        return ResponseEntity.ok(new OkResponseMessage("Login successful"));
    }

    @GetMapping("/me")
    public ResponseEntity<MeResponse> me(HttpServletRequest request) {
        String token = AuthCookies.readAccessToken(request)
            .orElseThrow(() -> new AuthException(AuthError.UNAUTHORIZED));
        AuthService.AuthenticatedUser user = authService.authenticate(token);

        return ResponseEntity.ok(new MeResponse(user.email()));
    }

    @PostMapping("/logout")
    public ResponseEntity<Void> logout(HttpServletRequest request, HttpServletResponse response) {
        String token = AuthCookies.readAccessToken(request)
            .orElseThrow(() -> new AuthException(AuthError.UNAUTHORIZED));
        authService.logout(token);
        AuthCookies.clearAccessToken(request, response);
        return ResponseEntity.noContent().build();
    }
}
