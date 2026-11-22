package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	DefaultFrontendOrigin = "http://localhost:5173"
	minJWTKeyLength       = 32
)

type Settings struct {
	DBUser         string
	DBPassword     string
	JWTKey         string
	DBHost         string
	DBPort         int
	DBName         string
	FrontendOrigin string
	LogLevel       slog.Level
}

func Load(dotEnvPath string) (Settings, error) {
	fileVars, err := readDotEnv(dotEnvPath)
	if err != nil {
		return Settings{}, err
	}
	get := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fileVars[key]
	}
	return parse(get)
}

// parse parses the configuration from the given getter function.
// It returns the populated Settings struct or an error if any required settings are missing or invalid.
func parse(get func(string) string) (Settings, error) {
	var problems []error

	s := Settings{
		DBUser:         get("DB_USER"),
		DBPassword:     get("DB_PASSWORD"),
		JWTKey:         get("JWT_KEY"),
		DBHost:         withDefault(get("DB_HOST"), "localhost"),
		DBName:         withDefault(get("DB_NAME"), "auth_go"),
		FrontendOrigin: withDefault(get("FRONTEND_ORIGIN"), DefaultFrontendOrigin),
	}

	if s.DBUser == "" {
		problems = append(problems, errors.New("DB_USER is required"))
	}

	if s.DBPassword == "" {
		problems = append(problems, errors.New("DB_PASSWORD is required"))
	}

	switch {
	case s.JWTKey == "":
		problems = append(problems, errors.New("JWT_KEY is required"))
	case utf8.RuneCountInString(s.JWTKey) < minJWTKeyLength:
		problems = append(problems, fmt.Errorf("JWT_KEY must be at least %d characters", minJWTKeyLength))
	}

	port, err := strconv.Atoi(withDefault(get("DB_PORT"), "5432"))
	if err != nil || port < 1 || port > 65535 {
		problems = append(problems, errors.New("DB_PORT must be a number between 1 and 65535"))
	}
	s.DBPort = port

	if err := validateOrigin(s.FrontendOrigin); err != nil {
		problems = append(problems, err)
	}

	level, err := parseLogLevel(withDefault(get("LOG_LEVEL"), "INFO"))
	if err != nil {
		problems = append(problems, err)
	}
	s.LogLevel = level

	if len(problems) > 0 {
		return Settings{}, errors.Join(problems...)
	}

	return s, nil
}

// DatabaseURL constructs the PostgreSQL database connection URL from the settings.
// It returns the URL as a string.
func (s Settings) DatabaseURL() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(s.DBUser, s.DBPassword),
		Host:   net.JoinHostPort(s.DBHost, strconv.Itoa(s.DBPort)),
		Path:   "/" + s.DBName,
	}
	return u.String()
}

// String returns a string representation of the settings with sensitive information redacted.
func (s Settings) String() string {
	return fmt.Sprintf("Settings{db=%s@%s:%d/%s frontendOrigin=%s logLevel=%s secrets=redacted}",
		s.DBUser, s.DBHost, s.DBPort, s.DBName, s.FrontendOrigin, s.LogLevel)
}

// withDefault returns the value if it is not empty, otherwise it returns the fallback.
func withDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// validateOrigin checks if the given origin is a plain origin without wildcards, paths, or trailing slashes.
// It returns an error if the origin is invalid otherwise nil.
func validateOrigin(origin string) error {
	bad := errors.New("FRONTEND_ORIGIN must be a plain origin like http://localhost:5173 (no wildcard, path or trailing slash)")
	if strings.Contains(origin, "*") {
		return bad
	}
	u, err := url.Parse(origin)
	if err != nil {
		return bad
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" ||
		u.Fragment != "" || u.User != nil || u.Opaque != "" {
		return bad
	}
	return nil
}

// parseLogLevel parses the log level string into a slog.Level.
// It returns an error if the log level is invalid.
func parseLogLevel(s string) (slog.Level, error) {
	switch s {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, errors.New("LOG_LEVEL must be one of DEBUG, INFO, WARNING, ERROR")
	}
}

// readDotEnv reads the content of a .env file and returns a map of key-value pairs.
// If the file does not exist, it returns an empty map without an error.
func readDotEnv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return parseDotEnv(string(data)), nil
}

// parseDotEnv parses the content of a .env file into a map of key-value pairs.
// It handles trimming of whitespace, ignoring comments, and removing surrounding quotes from values.
func parseDotEnv(content string) map[string]string {
	vars := map[string]string{}
	content = strings.TrimPrefix(content, "\ufeff")
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			first, last := value[0], value[len(value)-1]
			if (first == '"' || first == '\'') && first == last {
				value = value[1 : len(value)-1]
			}
		}
		vars[key] = value
	}
	return vars
}
