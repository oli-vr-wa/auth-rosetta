import { randomUUID } from 'node:crypto';
import { Injectable } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { JWT_AUDIENCE, JWT_CLOCK_SKEW_SECONDS, JWT_ISSUER } from './auth.constants';

export interface AccessTokenPayload {
    sub: string;
    email: string;
    jti: string;
    iat: number;
    exp: number;
    iss: string;
    aud: string;
}

@Injectable()
export class TokenService {
    constructor(private readonly jwt: JwtService) { }

    createToken(userId: string, email: string): Promise<string> {
        return this.jwt.signAsync({ email }, { subject: userId, jwtid: randomUUID() });
    }

    verifyToken(token: string): Promise<AccessTokenPayload> {
        return this.jwt.verifyAsync<AccessTokenPayload>(token, {
            algorithms: ['HS256'],
            issuer: JWT_ISSUER,
            audience: JWT_AUDIENCE,
            clockTolerance: JWT_CLOCK_SKEW_SECONDS
        });
    }
}