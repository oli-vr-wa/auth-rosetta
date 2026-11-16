import { CanActivate, ExecutionContext, Injectable, Logger, UnauthorizedException } from "@nestjs/common";
import { AccessTokenPayload, TokenService } from "./token.service";
import { AuthService } from "./auth.service";
import type { Request } from 'express';
import { AUTH_COOKIE_NAME } from "./auth-cookie";

export interface AuthenticatedRequest extends Request {
    user?: AccessTokenPayload;
}

@Injectable()
export class CookieAuthGuard implements CanActivate {
    private readonly logger = new Logger(CookieAuthGuard.name);

    constructor(
        private readonly tokens: TokenService,
        private readonly auth: AuthService
    ) { }

    /**
     * Determines whether the current request is authenticated based on the presence and validity of the authentication cookie.
     * @param context - The execution context of the request.
     * @returns A promise that resolves to true if the request is authenticated; otherwise, it throws an UnauthorizedException.
     */
    async canActivate(context: ExecutionContext): Promise<boolean> {
        const req = context.switchToHttp().getRequest<AuthenticatedRequest>();

        const cookies = req.cookies as Record<string, unknown> | undefined;
        const token = cookies?.[AUTH_COOKIE_NAME];
        if (typeof token !== 'string' || token === '') {
            this.logger.debug('Request rejected: no access_token cookie');
            throw new UnauthorizedException();
        }

        let payload: AccessTokenPayload;
        try {
            payload = await this.tokens.verifyToken(token);

        } catch (err) {
            this.logger.debug(`Token rejected: ${err instanceof Error ? err.message : 'unknown error'}`);
            throw new UnauthorizedException();
        }

        if (await this.auth.isRevoked(payload.jti)) {
            this.logger.debug('Token rejected: token has been revoked'); 
            throw new UnauthorizedException();
        }

        req.user = payload;
        return true;
    }
}