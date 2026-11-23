package com.authrosetta.java.config;

import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.function.UnaryOperator;
import java.util.stream.Collectors;

import javax.sql.DataSource;

import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Application configuration for the Auth Rosetta Java backend.
 * AppConfig
 * @param dbHost the database host
 * @param dbPort the database port
 * @param dbName the database name
 * @param dbUser the database user
 * @param dbPassword the database password
 * @param jwtKey the JWT key used for authentication
 * @param frontendOrigin the frontend origin URL
 * @param logLevel the logging level (DEBUG, INFO, WARNING, ERROR)
 */
public record AppConfig(
    String dbHost,
    int dbPort,
    String dbName,
    String dbUser,
    String dbPassword,
    byte[] jwtKey,
    String frontendOrigin,
    String logLevel
) {
    public static final int MIN_JWT_KEY_BYTES = 32;
    private static final Set<String> VALID_LOG_LEVELS = Set.of("DEBUG", "INFO", "WARNING", "ERROR");

    /**
     * Loads the default application configuration using the dotenv file at "../.env" and the system environment variables.
     * @return the loaded application configuration
     */
    public static AppConfig loadDefault() {
        return load(Path.of("..", ".env"), System::getenv);
        
    }

    /**
     * Loads the application configuration from the specified dotenv file and environment variables.
     * @param dotenvPath the path to the dotenv file
     * @param envLookup a function to retrieve environment variable values by name
     * @return the loaded application configuration
     */
    public static AppConfig load(Path dotenvPath, UnaryOperator<String> envLookup) {
        Map<String, String> fileVars = parseDotenv(dotenvPath);
        return parse(key -> {
            String fromEnv = envLookup.apply(key);
            if (fromEnv != null && !fromEnv.trim().isEmpty()) {
                return fromEnv.trim();
            }
            String fromFile = fileVars.get(key);
            if (fromFile != null && !fromFile.trim().isEmpty()) {
                return fromFile.trim();
            }
            return null;
        });
    }

    /**
     * Parses the application configuration from environment variables.
     * Collects any validation errors and throws an exception if any are found.
     * @param get a function to retrieve environment variable values by name
     * @return the parsed and validated application configuration
     */
    static AppConfig parse(UnaryOperator<String> get) {
        List<String> errors = new ArrayList<>();

        String dbHost = valueOrDefault(get.apply("DB_HOST"), "localhost");
        String dbName = valueOrDefault(get.apply("DB_NAME"), "auth_java");

        int dbPort = 5432;
        String rawPort = get.apply("DB_PORT");
        if (rawPort != null) {
            try {
                dbPort = Integer.parseInt(rawPort);
                if (dbPort < 1 || dbPort > 65535) {
                    errors.add("DB_PORT must be between 1 and 65535");
                }
            } catch (NumberFormatException e) {
                errors.add("DB_PORT must be a valid integer");
            }
        }

        String dbUser = get.apply("DB_USER");
        if (dbUser == null) {
            errors.add("DB_USER is required");
        }

        String dbPassword = get.apply("DB_PASSWORD");
        if (dbPassword == null) {
            errors.add("DB_PASSWORD is required");
        }

        byte[] jwtKey = new byte[0];
        String rawJwtKey = get.apply("JWT_KEY");
        if (rawJwtKey == null) {
            errors.add("JWT_KEY is required");
        } else {
            jwtKey = rawJwtKey.getBytes(StandardCharsets.UTF_8);
            if (jwtKey.length < MIN_JWT_KEY_BYTES) {
                errors.add("JWT_KEY must be at least " + MIN_JWT_KEY_BYTES + " bytes");
            }
        }

        String frontendOrigin = valueOrDefault(get.apply("FRONTEND_ORIGIN"), "http://localhost:5173");
        if (!isValidOrigin(frontendOrigin)) {
            errors.add("FRONTEND_ORIGIN must be a valid http or https origin without a path or trailing slash.");
        }

        String logLevel = valueOrDefault(get.apply("LOG_LEVEL"), "INFO");
        if (!VALID_LOG_LEVELS.contains(logLevel)) {
            errors.add("LOG_LEVEL must be one of " + VALID_LOG_LEVELS.stream().collect(Collectors.joining(", ")));
        }

        if (!errors.isEmpty()) {
            throw new IllegalStateException("Invalid configuration: " + String.join(", ", errors));
        }

        return new AppConfig(dbHost, dbPort, dbName, dbUser, dbPassword, jwtKey, frontendOrigin, logLevel);
    }

    /**
     * Method to construct the JDBC URL for the PostgreSQL database.
     * @return the JDBC URL for the PostgreSQL database
     */
    public String jdbcUrl() {
        return String.format("jdbc:postgresql://%s:%d/%s?user=%s&password=%s",
            dbHost, dbPort, dbName, urlEncode(dbUser), urlEncode(dbPassword));
    }

    /**
     * Method to get the Spring-compatible log level.
     * @return the Spring-compatible log level, converting "WARNING" to "WARN" if necessary
     */
    public String springLogLevel() {
        return "WARNING".equals(logLevel) ? "WARN" : logLevel;
    }

    @Override 
    public String toString() {
        return "AppConfig[dbHost=" + dbHost 
            + ", dbPort=" + dbPort
            + ", dbName=" + dbName
            + ", dbUser=" + dbUser
            + ", dbPassword=[REDACTED]"
            + ", jwtKey=[REDACTED]"
            + ", frontendOrigin=" + frontendOrigin
            + ", logLevel=" + logLevel + "]";
    }

    /**
     * Method to parse a .env file and extract key-value pairs.
     * @param path the path to the .env file
     * @return a map containing the key-value pairs from the .env file
     */
    static Map<String, String> parseDotenv(Path path) {
        Map<String, String> values = new HashMap<>();
        if (path == null || !Files.isRegularFile((path))) {
            return values;
        }

        try {
            String content = Files.readString(path, StandardCharsets.UTF_8);
            if (content.startsWith("\uFEFF")) {
                content = content.substring(1);
            }
            for (String rawLine : content.split("\\R")) {
                String line = rawLine.trim();
                if (line.isEmpty() || line.startsWith("#")) {
                    continue;
                }
                int eq = line.indexOf('=');
                if (eq <= 0) {
                    continue;
                }
                String key = line.substring(0, eq).trim();
                String val = line.substring(eq + 1).trim();
                if (val.length() >= 2 
                    && ((val.startsWith("\"") && val.endsWith("\"")) || (val.startsWith("'") && val.endsWith("'")))) {
                        val = val.substring(1, val.length() - 1);
                }
                values.put(key, val);
            }            
        } catch (IOException e) {
            throw new IllegalStateException("Failed to read .env file at " + path, e);
        }

        return values;
    }

    /**
     * Checks if the given origin is a valid URL without wildcards, trailing slashes, query parameters, or fragments.
     * @param origin the origin URL to validate
     * @return true if the origin is valid, false otherwise
     */
    static boolean isValidOrigin(String origin) {
        if (origin == null || origin.contains("*") || origin.endsWith("/")) {
            return false;
        }

        try {
            URI uri = new URI(origin);
            String scheme = uri.getScheme();
            if (!"http".equals(scheme) && !"https".equals(scheme)) {
                return false;
            }
            if (uri.getHost() == null || uri.getHost().isEmpty()) {
                return false;
            }
            String path = uri.getRawPath();
            return (path == null || path.isEmpty()) 
                && uri.getRawQuery() == null
                && uri.getRawFragment() == null;
        } catch (URISyntaxException e) {
            return false;
        }
    }

    /**
     * Returns the value if it is not null, otherwise returns the fallback.
     * @param value the value to check
     * @param fallback the fallback value to return if the value is null
     * @return the value if it is not null, otherwise the fallback
     */
    private static String valueOrDefault(String value, String fallback) {
        return value != null ? value : fallback;
    }

    /**
     * Encodes the given value for use in a URL, replacing spaces with %20.
     * @param value the value to encode
     * @return the URL-encoded value with spaces replaced by %20
     */
    private static String urlEncode(String value) {
        return URLEncoder.encode(value, StandardCharsets.UTF_8).replace("+", "%20");
    }

    @Configuration 
    public static class StringConfig {
        @Bean 
        public AppConfig appConfig() {
            return AppConfig.loadDefault();
        }

        @Bean 
        public DataSource dataSource(AppConfig config) {
            return DataSourceBuilder.create()
                .driverClassName("org.postgresql.Driver")
                .url(config.jdbcUrl())
                .build();
        }
    }
}
