import { Logger } from '@nestjs/common';
import { json, type ErrorRequestHandler, type RequestHandler } from 'express';

const logger = new Logger('JsonBody');

const MAX_BODY_BYTES = 8 * 1024; // 8 KB

export const jsonBody: RequestHandler = json({ limit: MAX_BODY_BYTES });

/**
 * Middleware that ensures the request body is a valid JSON object. 
 * If the body is not a valid JSON object, it responds with a 400 status code and an error message.
 * @param req The incoming HTTP request.
 * @param res The HTTP response object.
 * @param next The next middleware function in the Express request-response cycle.
 * @returns void
 */
export const requireJsonObject: RequestHandler = (req, res, next) => {
    if (
        req.body !== undefined &&
        (typeof req.body !== 'object' || req.body === null || Array.isArray(req.body))
    ) {
        logger.warn(`${req.method} ${req.originalUrl} -> 400: Invalid JSON object`);
        next(res.status(400).json({ error: 'Invalid request' }));
        return
    }
    next();
}

/**
 * Error handler middleware for handling JSON body parsing errors in Express.
 * This middleware checks if the error is a client error (HTTP status code 4xx) and responds with a standardized error message.
 * If the error is not a client error, it passes the error to the next middleware in the chain.
 * @param err - The error object thrown by the JSON body parser.
 * @param req - The incoming HTTP request.
 * @param res - The HTTP response object.
 * @param next - The next middleware function in the Express request-response cycle.
 */
export const jsonBodyErrorHandler: ErrorRequestHandler = (err: unknown, req, res, next) => {
    const status = typeof err === 'object' && err !== null
        ? (err as { status?: unknown }).status
        : undefined;

    if (typeof status === 'number' && status >= 400 && status < 500) {
        const reason = err instanceof Error ? err.message : 'unknown';
        // Normalize 413 Payload Too Large to 400 Bad Request for consistency with the API contract
        const responseStatus = status === 413 ? 400 : status;

        logger.warn(`${req.method} ${req.originalUrl} -> ${responseStatus}: ${reason}`);
        res.status(responseStatus).json({ error: 'Invalid request' });
        return;
    }

    next(err);
}