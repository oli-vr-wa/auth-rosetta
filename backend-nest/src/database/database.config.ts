import { DataSourceOptions } from 'typeorm';
import { RevokedToken } from '../auth/revoked-token.entity';
import { User } from '../users/user.entity';

export function buildDataSourceOptions(
    get: (key: string) => string | undefined,
): DataSourceOptions {
    return {
        type: 'postgres',
        host: get('DB_HOST') ?? 'localhost',
        port: Number(get('DB_PORT') ?? 5432),
        username: get('DB_USER'),
        password: get('DB_PASSWORD'),
        database: 'auth_nest',
        entities: [User, RevokedToken],
        migrations: [`${__dirname}/migrations/*{.ts,.js}`],
        synchronize: false,
    };
}
