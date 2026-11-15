import { createParamDecorator, ExecutionContext, UnauthorizedException } from '@nestjs/common';
import type { AuthenticatedRequest } from './cookie-auth.guard';
import type { AccessTokenPayload } from './token.service';

/**
 * A custom decorator to extract the current authenticated user from the request context.
 */
export const CurrentUser = createParamDecorator(
    (_data: unknown, ctx: ExecutionContext): AccessTokenPayload => {
        const user = ctx.switchToHttp().getRequest<AuthenticatedRequest>().user;
        if (!user) throw new UnauthorizedException();
        return user;
    }
)