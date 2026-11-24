package com.authrosetta.java.api;

import com.authrosetta.java.config.AppConfig;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Import;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@WebMvcTest(controllers = {HealthController.class, ApiExceptionHandler.class})
@Import({HttpContractFilter.class, HttpContractTest.TestConfig.class})
class HttpContractTest {

    @Autowired
    private MockMvc mockMvc;

    @TestConfiguration
    static class TestConfig {
        @Bean
        AppConfig appConfig() {
            return new AppConfig(
                "localhost",
                5432,
                "auth_java",
                "user",
                "pass",
                "0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.UTF_8),
                "http://localhost:5173",
                "INFO"
            );
        }

        @Bean
        EchoController echoController() {
            return new EchoController();
        }
    }

    @RestController
    static class EchoController {
        @PostMapping("/test/echo")
        public String echo(@RequestBody String body) {
            return body;
        }
    }

    @Test
    void corsPreflight_returns204WithContractHeaders() throws Exception {
        mockMvc.perform(options("/api/auth/login")
                .header(HttpHeaders.ORIGIN, "http://localhost:5173"))
            .andExpect(status().isNoContent())
            .andExpect(header().string(HttpHeaders.ACCESS_CONTROL_ALLOW_ORIGIN, "http://localhost:5173"))
            .andExpect(header().string(HttpHeaders.ACCESS_CONTROL_ALLOW_CREDENTIALS, "true"))
            .andExpect(header().string(HttpHeaders.ACCESS_CONTROL_ALLOW_METHODS, "GET, POST, OPTIONS"))
            .andExpect(header().string(HttpHeaders.ACCESS_CONTROL_ALLOW_HEADERS, "Content-Type"));
    }

    @Test
    void unknownRouteAndWrongMethod_returnContractJsonAndCorsHeaders() throws Exception {
        mockMvc.perform(get("/api/does-not-exist"))
            .andExpect(status().isNotFound())
            .andExpect(header().string(HttpHeaders.ACCESS_CONTROL_ALLOW_ORIGIN, "http://localhost:5173"))
            .andExpect(jsonPath("$.error").value("Invalid request"));

        mockMvc.perform(delete("/health"))
            .andExpect(status().isMethodNotAllowed())
            .andExpect(header().string(HttpHeaders.ACCESS_CONTROL_ALLOW_ORIGIN, "http://localhost:5173"))
            .andExpect(jsonPath("$.error").value("Invalid request"));
    }

    @Test
    void bodyLimit_accepts8192BytesAndRejects8193Bytes() throws Exception {
        String exact8192 = "x".repeat(8_192);
        mockMvc.perform(post("/test/echo")
                .contentType(MediaType.APPLICATION_JSON)
                .content(exact8192))
            .andExpect(status().isOk());

        String over8193 = "x".repeat(8_193);
        mockMvc.perform(post("/test/echo")
                .contentType(MediaType.APPLICATION_JSON)
                .content(over8193))
            .andExpect(status().isBadRequest())
            .andExpect(header().string(HttpHeaders.ACCESS_CONTROL_ALLOW_ORIGIN, "http://localhost:5173"))
            .andExpect(jsonPath("$.error").value("Invalid request"));
    }

    @Test
    void cookies_setAndClearMatchContractAttributes() {
        MockHttpServletRequest httpReq = new MockHttpServletRequest();
        httpReq.setSecure(false);
        MockHttpServletResponse httpRes = new MockHttpServletResponse();

        AuthCookies.setAccessToken(httpReq, httpRes, "jwt-token-value");
        String setHeader = httpRes.getHeader(HttpHeaders.SET_COOKIE);
        assertNotNull(setHeader);
        assertTrue(setHeader.contains("access_token=jwt-token-value"));
        assertTrue(setHeader.contains("Max-Age=3600"));
        assertFalse(setHeader.contains("Max-Age=3600.0"));
        assertTrue(setHeader.contains("HttpOnly"));
        assertTrue(setHeader.contains("SameSite=Strict"));
        assertTrue(setHeader.contains("Path=/"));
        assertFalse(setHeader.contains("Secure"));

        MockHttpServletRequest httpsReq = new MockHttpServletRequest();
        httpsReq.setSecure(true);
        MockHttpServletResponse clearRes = new MockHttpServletResponse();

        AuthCookies.clearAccessToken(httpsReq, clearRes);
        String clearHeader = clearRes.getHeader(HttpHeaders.SET_COOKIE);
        assertNotNull(clearHeader);
        assertTrue(clearHeader.contains("Max-Age=0"));
        assertTrue(clearHeader.contains("Secure"));
        assertTrue(clearHeader.contains("SameSite=Strict"));
    }
}