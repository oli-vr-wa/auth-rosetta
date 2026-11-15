import { Body, Controller, Get, HttpCode, Post, Req, Res, UseGuards } from '@nestjs/common';
import type { Request, Response } from 'express';
import { clearAuthCookie, setAuthCookie } from './auth-cookie';
import { AuthService } from './auth.service';
import { ConfirmDto, LoginDto, RegisterDto } from './dto/auth.dto';
import { CookieAuthGuard } from './cookie-auth.guard';
import { CurrentUser } from './current-user.decorator';
import { type AccessTokenPayload } from './token.service';

@Controller('api/auth')
export class AuthController {
    constructor(private readonly auth: AuthService) { }

    @Post('register') // NestJS default POST is 201, which the contract expects.
    register(@Body() body: RegisterDto) {
        return this.auth.register(body.email, body.password);
    }

    @Post('2fa/confirm')
    @HttpCode(200)
    async confirm(@Body() body: ConfirmDto) {
        await this.auth.confirmTwoFactor(body.email, body.code);
        return { message: 'Two-factor authentication enabled' };
    }

    @Post('login')
    @HttpCode(200)
    async login(
        @Body() body: LoginDto,
        @Req() req: Request,
        @Res({ passthrough: true }) res: Response,
    ) {
        const token = await this.auth.login(body.email, body.password, body.code);
        setAuthCookie(req, res, token);
        return { message: 'Login successful' };
    }

    @Get('me')
    @UseGuards(CookieAuthGuard)
    me(@CurrentUser() user: AccessTokenPayload) {
        return { email: user.email };
    }

    @Post('logout')
    @HttpCode(204)
    @UseGuards(CookieAuthGuard)
    async logout(
        @CurrentUser() user: AccessTokenPayload,
        @Req() req: Request,
        @Res({ passthrough: true }) res: Response,
    ) {
        await this.auth.logout(user.jti, new Date(user.exp * 1000)); // expiration is in seconds, convert to milliseconds
        clearAuthCookie(req, res);
    }
}