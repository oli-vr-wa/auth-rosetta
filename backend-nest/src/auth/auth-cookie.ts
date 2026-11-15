import type { CookieOptions, Request, Response } from 'express';
import { JWT_EXPIRY_MINUTES } from './auth.constants';

export const AUTH_COOKIE_NAME = 'access_token';

/**
 * Sets the authentication cookie with the provided token.
 * @param req - The incoming HTTP request.
 * @returns CookieOptions - The options for the authentication cookie.
 */
function baseOptions(req: Request): CookieOptions {
    return {
        httpOnly: true,
        secure: req.secure,
        sameSite: 'strict',
        path: '/',
    };
}

/**
 * Sets the authentication cookie with the provided token.
 * @param req - The incoming HTTP request.
 * @param res - The HTTP response object.
 * @param token - The JWT token to be set in the cookie.
 * @returns void
 */
export function setAuthCookie(req: Request, res: Response, token: string): void {
    res.cookie(AUTH_COOKIE_NAME, token, {
        ...baseOptions(req),
        maxAge: JWT_EXPIRY_MINUTES * 60 * 1000, // Convert minutes to milliseconds
    });
}

/**
 * Clears the authentication cookie from the response.
 * @param req - The incoming HTTP request.
 * @param res - The HTTP response object.
 * @returns void
 */
export function clearAuthCookie(req: Request, res: Response): void {
    res.clearCookie(AUTH_COOKIE_NAME, baseOptions(req));
}