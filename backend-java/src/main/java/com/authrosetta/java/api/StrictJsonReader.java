package com.authrosetta.java.api;

import jakarta.servlet.http.HttpServletRequest;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import tools.jackson.core.JacksonException;
import tools.jackson.core.JsonParser;
import tools.jackson.core.JsonToken;
import tools.jackson.core.StreamReadFeature;
import tools.jackson.core.json.JsonFactory;
import tools.jackson.core.ObjectReadContext;

import java.io.IOException;
import java.io.InputStream;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

@Component
public class StrictJsonReader {

    private final JsonFactory jsonFactory = JsonFactory.builder()
        .enable(StreamReadFeature.STRICT_DUPLICATE_DETECTION)
        .build();

    public record CredentialsPayload(String email, String password) {}
    public record ConfirmTwoFactorPayload(String email, String code) {}
    public record LoginPayload(String email, String password, String code) {}

    public CredentialsPayload readCredentials(HttpServletRequest request) {
        Map<String, String> fields = readStringFields(request, Set.of("email", "password"));
        return new CredentialsPayload(fields.get("email"), fields.get("password"));
    }

    public ConfirmTwoFactorPayload readConfirmTwoFactor(HttpServletRequest request) {
        Map<String, String> fields = readStringFields(request, Set.of("email", "code"));
        return new ConfirmTwoFactorPayload(fields.get("email"), fields.get("code"));
    }

    public LoginPayload readLogin(HttpServletRequest request) {
        Map<String, String> fields = readStringFields(request, Set.of("email", "password", "code"));
        return new LoginPayload(fields.get("email"), fields.get("password"), fields.get("code"));
    }

    private Map<String, String> readStringFields(HttpServletRequest request, Set<String> requiredStringFields) {
        requireJsonContentType(request);

        try (InputStream in = request.getInputStream();
             JsonParser parser = jsonFactory.createParser(ObjectReadContext.empty(), in)) {

            JsonToken first = parser.nextToken();
            if (first != JsonToken.START_OBJECT) {
                throw new InvalidRequestJsonException("Expected top-level JSON object");
            }

            Map<String, String> extracted = new HashMap<>();
            while (parser.nextToken() != JsonToken.END_OBJECT) {
                if (parser.currentToken() != JsonToken.PROPERTY_NAME) {
                    throw new InvalidRequestJsonException("Expected JSON property name");
                }
                String fieldName = parser.currentName();
                JsonToken valueToken = parser.nextToken();

                if (requiredStringFields.contains(fieldName)) {
                    if (valueToken != JsonToken.VALUE_STRING) {
                        throw new InvalidRequestJsonException("Field must be a JSON string: " + fieldName);
                    }
                    extracted.put(fieldName, parser.getString());
                } else if (valueToken != null && valueToken.isStructStart()) {
                    parser.skipChildren();
                }
            }

            if (parser.nextToken() != null) {
                throw new InvalidRequestJsonException("Trailing JSON content is not allowed");
            }

            for (String required : requiredStringFields) {
                if (!extracted.containsKey(required)) {
                    throw new InvalidRequestJsonException("Missing required field: " + required);
                }
            }

            return extracted;
        } catch (HttpContractFilter.BodyTooLargeException ex) {
            throw ex;
        } catch (JacksonException | IOException ex) {
            throw new InvalidRequestJsonException("Malformed JSON payload");
        }
    }

    private static void requireJsonContentType(HttpServletRequest request) {
        String contentType = request.getContentType();
        if (contentType == null || contentType.isBlank()) {
            throw new InvalidRequestJsonException("Missing Content-Type header");
        }
        try {
            MediaType mediaType = MediaType.parseMediaType(contentType);
            if (!MediaType.APPLICATION_JSON.isCompatibleWith(mediaType)) {
                throw new InvalidRequestJsonException("Unsupported Content-Type");
            }
        } catch (IllegalArgumentException ex) {
            throw new InvalidRequestJsonException("Invalid Content-Type header");
        }
    }

    public static class InvalidRequestJsonException extends RuntimeException {
        public InvalidRequestJsonException(String message) {
            super(message);
        }
    }
}