package com.authrosetta.java.api;

import com.authrosetta.java.config.AppConfig;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ReadListener;
import jakarta.servlet.ServletException;
import jakarta.servlet.ServletInputStream;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletRequestWrapper;
import jakarta.servlet.http.HttpServletResponse;
import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;

@Component
@Order(Ordered.HIGHEST_PRECEDENCE)
public class HttpContractFilter extends OncePerRequestFilter {

    public static final int MAX_BODY_BYTES = 8_192;

    private final String allowedOrigin;

    public HttpContractFilter(AppConfig config) {
        this.allowedOrigin = config.frontendOrigin();
    }

    @Override
    protected void doFilterInternal(
        HttpServletRequest request,
        HttpServletResponse response,
        FilterChain filterChain
    ) throws ServletException, IOException {
        applyCorsHeaders(response);

        if ("OPTIONS".equalsIgnoreCase(request.getMethod())) {
            response.setStatus(HttpServletResponse.SC_NO_CONTENT);
            return;
        }

        long contentLength = request.getContentLengthLong();
        if (contentLength > MAX_BODY_BYTES) {
            writeErrorJson(response, HttpServletResponse.SC_BAD_REQUEST, "Invalid request");
            return;
        }

        HttpServletRequest boundedRequest = new BoundedBodyRequestWrapper(request, MAX_BODY_BYTES);
        try {
            filterChain.doFilter(boundedRequest, response);
        } catch (BodyTooLargeException ex) {
            if (!response.isCommitted()) {
                writeErrorJson(response, HttpServletResponse.SC_BAD_REQUEST, "Invalid request");
            }
        }
    }

    private void applyCorsHeaders(HttpServletResponse response) {
        response.setHeader(HttpHeaders.ACCESS_CONTROL_ALLOW_ORIGIN, allowedOrigin);
        response.setHeader(HttpHeaders.ACCESS_CONTROL_ALLOW_CREDENTIALS, "true");
        response.setHeader(HttpHeaders.ACCESS_CONTROL_ALLOW_METHODS, "GET, POST, OPTIONS");
        response.setHeader(HttpHeaders.ACCESS_CONTROL_ALLOW_HEADERS, "Content-Type");
        response.setHeader(HttpHeaders.VARY, "Origin");
    }

    static void writeErrorJson(HttpServletResponse response, int status, String message) throws IOException {
        response.resetBuffer();
        response.setStatus(status);
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        response.setCharacterEncoding(StandardCharsets.UTF_8.name());
        String escaped = message.replace("\\", "\\\\").replace("\"", "\\\"");
        response.getWriter().write("{\"error\":\"" + escaped + "\"}");
        response.flushBuffer();
    }

    static class BodyTooLargeException extends RuntimeException {
        BodyTooLargeException() {
            super("Request body exceeds 8192 bytes");
        }
    }

    private static class BoundedBodyRequestWrapper extends HttpServletRequestWrapper {
        private final int maxBytes;
        private ServletInputStream cachedStream;
        private BufferedReader cachedReader;

        BoundedBodyRequestWrapper(HttpServletRequest request, int maxBytes) {
            super(request);
            this.maxBytes = maxBytes;
        }

        @Override
        public ServletInputStream getInputStream() throws IOException {
            if (cachedStream == null) {
                cachedStream = new BoundedServletInputStream(super.getInputStream(), maxBytes);
            }
            return cachedStream;
        }

        @Override
        public BufferedReader getReader() throws IOException {
            if (cachedReader == null) {
                String enc = getCharacterEncoding() != null ? getCharacterEncoding() : StandardCharsets.UTF_8.name();
                cachedReader = new BufferedReader(new InputStreamReader(getInputStream(), enc));
            }
            return cachedReader;
        }
    }

    private static class BoundedServletInputStream extends ServletInputStream {
        private final ServletInputStream delegate;
        private final int maxBytes;
        private long bytesRead = 0;

        BoundedServletInputStream(ServletInputStream delegate, int maxBytes) {
            this.delegate = delegate;
            this.maxBytes = maxBytes;
        }

        @Override
        public int read() throws IOException {
            int b = delegate.read();
            if (b != -1) {
                incrementAndCheck(1);
            }
            return b;
        }

        @Override
        public int read(byte[] b, int off, int len) throws IOException {
            int n = delegate.read(b, off, len);
            if (n > 0) {
                incrementAndCheck(n);
            }
            return n;
        }

        private void incrementAndCheck(int delta) {
            bytesRead += delta;
            if (bytesRead > maxBytes) {
                throw new BodyTooLargeException();
            }
        }

        @Override
        public boolean isFinished() {
            return delegate.isFinished();
        }

        @Override
        public boolean isReady() {
            return delegate.isReady();
        }

        @Override
        public void setReadListener(ReadListener readListener) {
            delegate.setReadListener(readListener);
        }
    }
}