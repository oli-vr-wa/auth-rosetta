import { Logger } from '@nestjs/common';
import { json, type ErrorRequestHandler, type RequestHandler } from 'express';

const logger = new Logger('JsonBody');

export const jsonBody: RequestHandler = json();

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

        logger.warn(`${req.method} ${req.originalUrl} -> ${status}: ${reason}`);
        res.status(status).json({ error: 'Invalid request' });
        return;
    }

    next(err);
}