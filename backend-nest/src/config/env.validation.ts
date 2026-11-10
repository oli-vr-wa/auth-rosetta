const REQUIRED = ['DB_USER', 'DB_PASSWORD', 'JWT_KEY'] as const;

export function validateEnv(config: Record<string, unknown>) {
    const missing = REQUIRED.filter((key) => !config[key]);

    if (missing.length > 0) {
        throw new Error(`Missing required environment variables: ${missing.join(', ')}`);
    }

    if (String(config.JWT_KEY).length < 32) {
        throw new Error('JWT_KEY must be at least 32 characters long');
    }

    return config;
}